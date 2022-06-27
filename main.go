package main

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

	"github.com/fogleman/gg"
	"github.com/gonutz/framebuffer"
	_ "github.com/kubesail/pibox-framebuffer/statik"
	"github.com/rakyll/statik/fs"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	qrcode "github.com/skip2/go-qrcode"
	"github.com/stianeikeland/go-rpio/v4"
)

var fbNum string
var statsOff = false

const SCREEN_SIZE = 240

type RGB struct {
	R uint8
	G uint8
	B uint8
}

func rgb(w http.ResponseWriter, req *http.Request) {

	var c RGB
	err := json.NewDecoder(req.Body).Decode(&c)
	if err != nil {
		http.Error(w, "Requires json body with R, G, and B keys! Values must be 0-255\n", http.StatusBadRequest)
		return
	}

	drawSolidColor(c)
	fmt.Fprintf(w, "parsed color: R%v G%v B%v\n", c.R, c.G, c.B)
	fmt.Fprintf(w, "wrote to framebuffer!\n")
}

func drawSolidColor(c RGB) {
	fb, err := framebuffer.Open("/dev/" + fbNum)
	if err != nil {
		panic(err)
	}
	defer fb.Close()
	magenta := image.NewUniform(color.RGBA{c.R, c.G, c.B, 255})
	draw.Draw(fb, fb.Bounds(), magenta, image.Point{}, draw.Src)
	statsOff = true
}

func qr(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()
	content, present := query["content"]
	if !present {
		http.Error(w, "Pass ?content= to render a QR code\n", http.StatusBadRequest)
		return
	}

	fb, err := framebuffer.Open("/dev/" + fbNum)
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
	statsOff = true
}

type DiskStatsResponse struct {
	BlockDevices []string
	Partitions   []string
	Models       []string
	K3sUsage     []string
	Lvs          string
	Pvs          string
}

func diskStats(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var responseData DiskStatsResponse
	k3sStorageProbe := exec.Command("du", "-b", "--max-depth=1", "/var/lib/rancher/k3s/storage/")
	var k3sStorageProbeStdout bytes.Buffer
	var k3sStorageProbeStderr bytes.Buffer
	k3sStorageProbe.Stdout = &k3sStorageProbeStdout
	k3sStorageProbe.Stderr = &k3sStorageProbeStderr
	err := k3sStorageProbe.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run du command: %v\n", k3sStorageProbeStderr.String())
	} else {
		responseData.K3sUsage = strings.Split(strings.Replace(strings.Trim(strings.Trim(k3sStorageProbeStdout.String(), "\n"), " "), "\t", " ", -1), "\n")
	}

	lvs := exec.Command("lvs", "--reportformat", "json", "--units=G")
	var lvsStdout bytes.Buffer
	var lvsStderr bytes.Buffer
	lvs.Stdout = &lvsStdout
	lvs.Stderr = &lvsStderr
	lvsErr := lvs.Run()
	if lvsErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to run lvs command: %v\n", lvsStderr.String())
	} else {
		responseData.Lvs = strings.Replace(strings.Trim(strings.Trim(lvsStdout.String(), "\n"), " "), "\t", " ", -1)
	}

	pvs := exec.Command("pvs", "--reportformat", "json", "--units=G")
	var pvsStdout bytes.Buffer
	var pvsStderr bytes.Buffer
	pvs.Stdout = &pvsStdout
	pvs.Stderr = &pvsStderr
	pvsErr := pvs.Run()
	if pvsErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to run pvs command: %v\n", pvsStderr.String())
	} else {
		responseData.Pvs = strings.Replace(strings.Trim(strings.Trim(pvsStdout.String(), "\n"), " "), "\t", " ", -1)
	}

	files, err := ioutil.ReadDir("/sys/block")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stat /sys/block: %v\n", err)
	} else {
		for _, f := range files {
			if strings.HasPrefix(f.Name(), "loop") {
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

			fmt.Println("calling parted " + fmt.Sprintf("/dev/%v", f.Name()))
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

func textRequest(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()
	content := query["content"]
	if len(content) == 0 {
		content = append(content, "no content param")
	}
	background := query["background"]
	dc := gg.NewContext(SCREEN_SIZE, SCREEN_SIZE)
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
	xInt := SCREEN_SIZE / 2
	if len(x) > 0 {
		xInt, _ = strconv.Atoi(x[0])
	}
	y := query["y"]
	yInt := SCREEN_SIZE / 2
	if len(y) > 0 {
		yInt, _ = strconv.Atoi(y[0])
	}

	textOnContext(dc, float64(xInt), float64(yInt), float64(sizeInt), content[0], true, gg.AlignCenter)
	flushTextToScreen(dc)
	statsOff = true
}

func textOnContext(dc *gg.Context, x float64, y float64, size float64, content string, bold bool, align gg.Align) {
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

func flushTextToScreen(dc *gg.Context) {
	fb, err := framebuffer.Open("/dev/" + fbNum)
	if err != nil {
		panic(err)
	}
	draw.Draw(fb, fb.Bounds(), dc.Image(), image.Point{}, draw.Src)
}

func drawImage(w http.ResponseWriter, req *http.Request) {
	fb, err := framebuffer.Open("/dev/" + fbNum)
	if err != nil {
		panic(err)
	}
	defer fb.Close()
	img, _, err := image.Decode(req.Body)
	if err != nil {
		panic(err)
	}
	draw.Draw(fb, fb.Bounds(), img, image.Point{}, draw.Src)
	fmt.Fprintf(w, "Image drawn\n")
	statsOff = true
}

func drawGIF(w http.ResponseWriter, req *http.Request) {
	fb, err := framebuffer.Open("/dev/" + fbNum)
	if err != nil {
		panic(err)
	}
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
	statsOff = true
}

func exit(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Received exit request, shutting down...")
	c := RGB{R: 0, G: 0, B: 255}
	drawSolidColor(c)
	os.Exit(0)
}

func setFramebuffer() {
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
			fbNum = item.Name()
			fmt.Println("Displaying on " + fbNum)
		}
	}
}

func splash() {
	fb, err := framebuffer.Open("/dev/" + fbNum)
	if err != nil {
		panic(err)
	}
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
	dc := gg.NewContext(SCREEN_SIZE, SCREEN_SIZE)
	dc.SetColor(color.RGBA{100, 100, 100, 255})
	textOnContext(dc, 120, 210, 20, "starting services", true, gg.AlignCenter)
	flushTextToScreen(dc)
}

func statsOn(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "Stats on\n")
	statsOff = false
}

func stats() {
	defer time.AfterFunc(3*time.Second, stats)
	if statsOff {
		return
	}

	var cpuUsage, _ = cpu.Percent(0, false)
	v, _ := mem.VirtualMemory()

	dc := gg.NewContext(SCREEN_SIZE, SCREEN_SIZE)
	dc.DrawRectangle(0, 0, 240, 240)
	dc.SetColor(color.RGBA{51, 51, 51, 255})
	dc.Fill()
	dc.SetColor(color.RGBA{160, 160, 160, 255})
	textOnContext(dc, 70, 28, 22, "CPU", false, gg.AlignCenter)
	cpuPercent := cpuUsage[0]
	colorCpu := color.RGBA{183, 225, 205, 255}
	if cpuPercent > 40 {
		colorCpu = color.RGBA{252, 232, 178, 255}
	}
	if cpuPercent > 70 {
		colorCpu = color.RGBA{244, 199, 195, 255}
	}
	dc.SetColor(colorCpu)
	textOnContext(dc, 70, 66, 30, fmt.Sprintf("%v%%", math.Round(cpuPercent)), true, gg.AlignCenter)
	dc.SetColor(color.RGBA{160, 160, 160, 255})
	textOnContext(dc, 170, 28, 22, "MEM", false, gg.AlignCenter)
	colorMem := color.RGBA{183, 225, 205, 255}
	if cpuPercent > 40 {
		colorMem = color.RGBA{252, 232, 178, 255}
	}
	if cpuPercent > 70 {
		colorMem = color.RGBA{244, 199, 195, 255}
	}
	dc.SetColor(colorMem)
	textOnContext(dc, 170, 66, 30, fmt.Sprintf("%v%%", math.Round(v.UsedPercent)), true, gg.AlignCenter)

	interfaces, _ := net.Interfaces()
	for _, inter := range interfaces {
		if inter.Name == "eth0" {
			dc.SetColor(color.RGBA{160, 160, 160, 255})
			textOnContext(dc, 130, 180, 22, "eth", false, gg.AlignLeft)
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
				textOnContext(dc, 110, 180, 22, "Disconnected", true, gg.AlignRight)
			} else {
				w, _ := dc.MeasureString(ipv4)
				var fontSize float64 = 26
				if w > 150 {
					fontSize = 22
				}
				dc.SetColor(color.RGBA{180, 180, 180, 255})
				textOnContext(dc, 110, 180, fontSize, ipv4, true, gg.AlignRight)
			}
		} else if inter.Name == "wlan0" {
			dc.SetColor(color.RGBA{180, 180, 180, 255})
			textOnContext(dc, 130, 210, 22, "wifi", false, gg.AlignLeft)
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
				textOnContext(dc, 110, 210, 22, "Disconnected", true, gg.AlignRight)
			} else {
				w, _ := dc.MeasureString(ipv4)
				var fontSize float64 = 26
				if w > 150 {
					fontSize = 22
				}
				dc.SetColor(color.RGBA{180, 180, 180, 255})
				textOnContext(dc, 110, 210, fontSize, ipv4, true, gg.AlignRight)
			}

		}
	}
	flushTextToScreen(dc)
}

func main() {
	err := rpio.Open()
	if err == nil {
		backlight := rpio.Pin(22)
		backlight.Output() // Output mode
		backlight.High()   // Set pin High
		setFramebuffer()
		splash()
		time.AfterFunc(6*time.Second, stats)
	} else {
		fmt.Fprintf(os.Stderr, "Could not connect to framebuffer screen: %v\n", err)
	}

	http.HandleFunc("/rgb", rgb)
	http.HandleFunc("/image", drawImage)
	http.HandleFunc("/gif", drawGIF)
	http.HandleFunc("/text", textRequest)
	http.HandleFunc("/stats/on", statsOn)
	http.HandleFunc("/qr", qr)
	http.HandleFunc("/disk-stats", diskStats)
	http.HandleFunc("/exit", exit)

	os.MkdirAll("/var/run/pibox/", 0755)
	listenSocket := os.Getenv("LISTEN_SOCKET")
	if listenSocket == "" {
		listenSocket = "/var/run/pibox/framebuffer.sock"
	}
	os.Remove(listenSocket)
	fmt.Printf("Listening on socket: %s\n", listenSocket)
	listener, err := net.Listen("unix", listenSocket)
	os.Chmod(listenSocket, 0777)
	if err != nil {
		log.Fatalf("Could not listen on %s: %v", listenSocket, err)
		return
	}
	defer listener.Close()
	if err = http.Serve(listener, nil); err != nil {
		log.Fatalf("Could not start HTTP server: %v", err)
	}
}
