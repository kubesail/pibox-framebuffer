package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	human "github.com/dustin/go-humanize"
	"github.com/fogleman/gg"
	"github.com/gonutz/framebuffer"
	"github.com/rakyll/statik/fs"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"github.com/skip2/go-qrcode"
)

const DefaultScreenSize = 240

type PiboxFrameBuffer struct {
	config *Config

	// enableStats will cycle the statistics screen if set to true
	enableStats bool
}

func (b *PiboxFrameBuffer) openFrameBuffer() *framebuffer.Device {
	fb, err := framebuffer.Open("/dev/" + b.config.fbNum)
	if err != nil {
		panic(err)
	}
	return fb
}

type RGB struct {
	R uint8
	G uint8
	B uint8
}

func (b *PiboxFrameBuffer) RGB(w http.ResponseWriter, req *http.Request) {
	var c RGB
	err := json.NewDecoder(req.Body).Decode(&c)
	if err != nil {
		http.Error(w, "Requires json body with R, G, and B keys! Values must be 0-255\n", http.StatusBadRequest)
		return
	}

	b.DrawSolidColor(c)
	fmt.Fprintf(w, "parsed color: R%v G%v B%v\n", c.R, c.G, c.B)
	fmt.Fprintf(w, "wrote to framebuffer!\n")
}

func (b *PiboxFrameBuffer) DrawSolidColor(c RGB) {
	fb, err := framebuffer.Open("/dev/" + b.config.fbNum)
	if err != nil {
		panic(err)
	}
	defer fb.Close()
	magenta := image.NewUniform(color.RGBA{c.R, c.G, c.B, 255})
	draw.Draw(fb, fb.Bounds(), magenta, image.Point{}, draw.Src)
	b.enableStats = false
}

func (b *PiboxFrameBuffer) QR(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()
	content, present := query["content"]
	if !present {
		http.Error(w, "Pass ?content= to render a QR code\n", http.StatusBadRequest)
		return
	}

	fb, err := framebuffer.Open("/dev/" + b.config.fbNum)
	if err != nil {
		panic(err)
	}
	defer fb.Close()

	// var q qrcode.QRCode
	q, err := qrcode.New(strings.Join(content, ""), qrcode.Low)
	q.DisableBorder = true
	// q.ForegroundColor = color.RGBA{236, 57, 99, 255}
	if err != nil {
		panic(err)
	}
	// var qr image.Image
	img := q.Image(180)

	draw.Draw(fb,
		image.Rectangle{Min: image.Point{X: 30, Y: 47}, Max: image.Point{X: 210, Y: 227}},
		img,
		image.Point{},
		draw.Src)

	fmt.Println("QR Code printed to screen")
	b.enableStats = false
}

type DiskStatsResponse struct {
	BlockDevices    []string
	Partitions      []string
	Models          []string
	RootUsage       []string
	K3sUsage        []string
	K3sStorageUsage []string
	Lvs             string
	Pvs             string
	K3sVersion      string
	K3sMount        string
	MountPoints     string
	Lsblk           string
}

func shell(app string, args []string) string {
	cmd := exec.Command(app, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run \"%v\": %v\n", app, stderr.String())
		return ""
	} else {
		return strings.Replace(strings.Trim(strings.Trim(stdout.String(), "\n"), " "), "\t", " ", -1)
	}
}

func (b *PiboxFrameBuffer) DiskStats(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var responseData DiskStatsResponse

	responseData.RootUsage = strings.Split(shell("df", strings.Split("/", " ")), "\n")
	responseData.K3sUsage = strings.Split(shell("df", strings.Split("/var/lib/rancher/k3s/", " ")), "\n")
	responseData.K3sStorageUsage = strings.Split(shell("du", strings.Split("-b --max-depth=1 /var/lib/rancher/k3s/storage/", " ")), "\n")
	responseData.K3sVersion = shell("k3s", strings.Split("--version", " "))
	responseData.MountPoints = shell("findmnt", strings.Split("-s -J -e", " "))
	responseData.Lvs = shell("lvs", strings.Split("--reportformat json --units=b", " "))
	responseData.Pvs = shell("pvs", strings.Split("--reportformat json --units=b", " "))
	responseData.Lsblk = shell("lsblk", strings.Split("-b -J", " "))

	files, err := ioutil.ReadDir("/sys/block")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stat /sys/block: %v\n", err)
	} else {
		for _, f := range files {
			if strings.HasPrefix(f.Name(), "loop") {
				continue
			}
			if strings.HasPrefix(f.Name(), "ram") {
				continue
			}

			responseData.BlockDevices = append(responseData.BlockDevices, f.Name())

			modelFilePath := fmt.Sprintf("/sys/block/%v/device/model", f.Name())
			modelDataRaw, err := os.ReadFile(modelFilePath)
			var modelData string
			modelData = strings.Trim(strings.Trim(string(modelDataRaw), "\n"), " ")
			if err != nil {
				fmt.Println(f.Name())
				fmt.Fprintf(os.Stderr, "Failed to collect model data: %v\n", err)
			} else {
				responseData.Models = append(responseData.Models, fmt.Sprintf("%v %v", f.Name(), modelData))
			}

			partedOutputRaw := exec.Command("timeout", "2", "parted", fmt.Sprintf("/dev/%v", f.Name()), "print", "-m")
			var partedOutputStdout bytes.Buffer
			partedOutputRaw.Stdout = &partedOutputStdout
			partedErr := partedOutputRaw.Run()
			if partedErr != nil {
				fmt.Println(f.Name())
				fmt.Fprintf(os.Stderr, "Failed to call parted: %v\n", partedErr)
			} else {
				var partedOutput string
				partedOutput = strings.Trim(strings.Trim(partedOutputStdout.String(), "\n"), " ")
				responseData.Partitions = append(responseData.Partitions, fmt.Sprintf("%v %v", f.Name(), partedOutput))
			}
		}
	}
	json.NewEncoder(w).Encode(responseData)
}

func (b *PiboxFrameBuffer) TextRequest(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()
	content := query["content"]
	if len(content) == 0 {
		content = append(content, "no content param")
	}
	background := query["background"]
	dc := gg.NewContext(b.config.screenSize, b.config.screenSize)
	if len(background) == 1 {
		dc.SetHexColor(background[0])
		dc.DrawRectangle(0, 0, 240, 240)
		dc.Fill()
	}
	c := query["color"]
	if len(c) > 0 {
		dc.SetHexColor(c[0])
	} else {
		dc.SetHexColor("cccccc")
	}
	size := query["size"]
	if len(size) == 0 {
		size = append(size, "22")
	}
	sizeInt, _ := strconv.Atoi(size[0])
	x := query["x"]
	xInt := b.config.screenSize / 2
	if len(x) > 0 {
		xInt, _ = strconv.Atoi(x[0])
	}
	y := query["y"]
	yInt := b.config.screenSize / 2
	if len(y) > 0 {
		yInt, _ = strconv.Atoi(y[0])
	}

	b.TextOnContext(dc, float64(xInt), float64(yInt), float64(sizeInt), content[0], true, gg.AlignCenter)
	b.flushTextToScreen(dc)
	b.enableStats = false
}

func (b *PiboxFrameBuffer) TextOnContext(dc *gg.Context, x float64, y float64, size float64, content string, bold bool, align gg.Align) {
	const S = 240
	// dc.SetRGB(float64(c.R), float64(c.G), float64(c.B))
	if bold {
		if err := dc.LoadFontFace("/usr/share/fonts/truetype/piboto/Piboto-Bold.ttf", float64(size)); err != nil {
			panic(err)
		}
	} else {
		if err := dc.LoadFontFace("/usr/share/fonts/truetype/piboto/Piboto-Regular.ttf", float64(size)); err != nil {
			panic(err)
		}
	}
	dc.DrawStringWrapped(content, x, y, 0.5, 0.5, 240, 1.5, align)
	// dc.Clip()
}

func (b *PiboxFrameBuffer) flushTextToScreen(dc *gg.Context) {
	fb := b.openFrameBuffer()
	draw.Draw(fb, fb.Bounds(), dc.Image(), image.Point{}, draw.Src)
	defer fb.Close()
}

func (b *PiboxFrameBuffer) DrawImage(w http.ResponseWriter, req *http.Request) {
	fb := b.openFrameBuffer()
	defer fb.Close()
	img, _, err := image.Decode(req.Body)
	if err != nil {
		panic(err)
	}
	draw.Draw(fb, fb.Bounds(), img, image.Point{}, draw.Src)
	fmt.Fprintf(w, "Image drawn\n")
	b.enableStats = false
}

func (b *PiboxFrameBuffer) DrawGIF(w http.ResponseWriter, req *http.Request) {
	fb := b.openFrameBuffer()
	defer fb.Close()
	imgGif, err := gif.DecodeAll(req.Body)
	if err != nil {
		panic(err)
	}
	for i, frame := range imgGif.Image {
		draw.Draw(fb, fb.Bounds(), frame, image.Point{}, draw.Src)
		time.Sleep(time.Millisecond * 3 * time.Duration(imgGif.Delay[i]))
	}
	fmt.Fprintf(w, "GIF drawn\n")
	b.enableStats = false
}

func (b *PiboxFrameBuffer) Exit() {
	fmt.Println("Received exit request, shutting down...")
	c := RGB{R: 0, G: 0, B: 255}
	b.DrawSolidColor(c)
}

func (b *PiboxFrameBuffer) setFramebuffer() {
	items, _ := ioutil.ReadDir("/sys/class/graphics")
	for _, item := range items {
		data, err := ioutil.ReadFile("/sys/class/graphics/" + item.Name() + "/name")
		if item.Name() == "fbcon" {
			continue
		}
		if err != nil {
			log.Fatalf("Could not enumerate framebuffers %v", err)
			return
		}
		if string(data) == "fb_st7789v\n" {
			// Update the config accordingly
			b.config.fbNum = item.Name()
			fmt.Println("Displaying on " + b.config.fbNum)
		}
	}
}

func (b *PiboxFrameBuffer) Splash() {
	fb := b.openFrameBuffer()
	defer fb.Close()

	statikFS, err := fs.New()
	if err != nil {
		panic(err)
	}
	r, err := statikFS.Open("/pibox-splash.png")
	if err != nil {
		panic(err)
	}
	img, _, err := image.Decode(r)
	if err != nil {
		panic(err)
	}
	draw.Draw(fb, fb.Bounds(), img, image.ZP, draw.Src)
	dc := gg.NewContext(b.config.screenSize, b.config.screenSize)
	dc.SetColor(color.RGBA{100, 100, 100, 255})
	b.TextOnContext(dc, 120, 210, 20, "starting services", true, gg.AlignCenter)
	b.flushTextToScreen(dc)
}

func (b *PiboxFrameBuffer) EnableStats(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "Stats on\n")
	b.enableStats = true
}

func (b *PiboxFrameBuffer) Stats() {
	defer time.AfterFunc(3*time.Second, b.Stats)
	if !b.enableStats {
		return
	}

	// create new context and clear screen
	dc := gg.NewContext(b.config.screenSize, b.config.screenSize)
	dc.DrawRectangle(0, 0, 240, 240)
	dc.SetColor(color.RGBA{51, 51, 51, 255})
	dc.Fill()

	var DISK_BAR_WIDTH float64 = 200
	var DISK_BAR_HEIGHT float64 = 105
	// outline
	dc.DrawRoundedRectangle(20, DISK_BAR_HEIGHT, DISK_BAR_WIDTH, 40, 5)
	dc.SetColor(color.RGBA{160, 160, 160, 255})
	dc.Fill()
	// inside
	dc.DrawRoundedRectangle(21, DISK_BAR_HEIGHT+1, DISK_BAR_WIDTH-2, 38, 4)
	dc.SetColor(color.RGBA{51, 51, 51, 255})
	dc.Fill()

	parts, _ := disk.Partitions(false)
	var found = false
	for _, p := range parts {
		device := p.Mountpoint
		if !strings.HasPrefix(device, b.config.diskMountPrefix) {
			continue
		}
		s, _ := disk.Usage(device)

		if s.Total == 0 {
			continue
		}

		percent := fmt.Sprintf("%s / %s",
			human.Bytes(s.Used),
			human.Bytes(s.Total),
		)

		// usage
		dc.DrawRoundedRectangle(21, DISK_BAR_HEIGHT+1, (s.UsedPercent/100.0)*(DISK_BAR_WIDTH-2), 38, 4)
		dc.SetColor(color.RGBA{70, 70, 70, 255})
		dc.Fill()

		dc.SetColor(color.RGBA{160, 160, 160, 255})
		b.TextOnContext(dc, 120, 125, 22, percent, false, gg.AlignCenter)
		found = true
	}
	if !found {
		dc.SetColor(color.RGBA{160, 160, 160, 255})
		b.TextOnContext(dc, 120, 125, 22, "No SSD configured", false, gg.AlignCenter)
	}

	var cpuUsage, _ = cpu.Percent(0, false)
	v, _ := mem.VirtualMemory()

	dc.SetColor(color.RGBA{160, 160, 160, 255})
	b.TextOnContext(dc, 70, 28, 22, "CPU", false, gg.AlignCenter)
	cpuPercent := cpuUsage[0]
	colorCpu := color.RGBA{183, 225, 205, 255}
	if cpuPercent > 40 {
		colorCpu = color.RGBA{252, 232, 178, 255}
	}
	if cpuPercent > 70 {
		colorCpu = color.RGBA{244, 199, 195, 255}
	}
	dc.SetColor(colorCpu)
	b.TextOnContext(dc, 70, 66, 30, fmt.Sprintf("%v%%", math.Round(cpuPercent)), true, gg.AlignCenter)
	dc.SetColor(color.RGBA{160, 160, 160, 255})
	b.TextOnContext(dc, 170, 28, 22, "MEM", false, gg.AlignCenter)
	colorMem := color.RGBA{183, 225, 205, 255}
	if cpuPercent > 40 {
		colorMem = color.RGBA{252, 232, 178, 255}
	}
	if cpuPercent > 70 {
		colorMem = color.RGBA{244, 199, 195, 255}
	}
	dc.SetColor(colorMem)
	b.TextOnContext(dc, 170, 66, 30, fmt.Sprintf("%v%%", math.Round(v.UsedPercent)), true, gg.AlignCenter)

	interfaces, _ := net.Interfaces()
	for _, inter := range interfaces {
		if inter.Name == "eth0" {
			dc.SetColor(color.RGBA{160, 160, 160, 255})
			b.TextOnContext(dc, 130, 180, 22, "eth", false, gg.AlignLeft)
			addrs, _ := inter.Addrs()
			var ipv4 = ""
			for _, addr := range addrs {
				ip := addr.String()
				if strings.Contains(ip, ".") {
					ipv4 = ip[:len(ip)-3] // assign and remove subnet mask
				}
			}
			if ipv4 == "" {
				dc.SetColor(color.RGBA{100, 100, 100, 255})
				b.TextOnContext(dc, 110, 180, 22, "Disconnected", true, gg.AlignRight)
			} else {
				w, _ := dc.MeasureString(ipv4)
				var fontSize float64 = 26
				if w > 150 {
					fontSize = 22
				}
				dc.SetColor(color.RGBA{180, 180, 180, 255})
				b.TextOnContext(dc, 110, 180, fontSize, ipv4, true, gg.AlignRight)
			}
		} else if inter.Name == "wlan0" {
			dc.SetColor(color.RGBA{180, 180, 180, 255})
			b.TextOnContext(dc, 130, 210, 22, "wifi", false, gg.AlignLeft)
			addrs, _ := inter.Addrs()
			var ipv4 = ""
			for _, addr := range addrs {
				ip := addr.String()
				if strings.Contains(ip, ".") {
					ipv4 = ip[:len(ip)-3] // assign and remove subnet mask
				}
			}
			if ipv4 == "" {
				dc.SetColor(color.RGBA{100, 100, 100, 255})
				b.TextOnContext(dc, 110, 210, 22, "Disconnected", true, gg.AlignRight)
			} else {
				w, _ := dc.MeasureString(ipv4)
				var fontSize float64 = 26
				if w > 150 {
					fontSize = 22
				}
				dc.SetColor(color.RGBA{180, 180, 180, 255})
				b.TextOnContext(dc, 110, 210, fontSize, ipv4, true, gg.AlignRight)
			}

		}
	}
	b.flushTextToScreen(dc)
}

func NewFrameBuffer(screenSize int, enableStats bool, diskMountPrefix string) *PiboxFrameBuffer {
	buf := &PiboxFrameBuffer{
		config: &Config{
			screenSize:      screenSize,
			diskMountPrefix: diskMountPrefix,
		},
		enableStats: enableStats,
	}
	buf.setFramebuffer()
	return buf
}
