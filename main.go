package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gonutz/framebuffer"
	qrcode "github.com/skip2/go-qrcode"
	"github.com/stianeikeland/go-rpio/v4"
)

var fbNum string

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
}

func hello(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "hello\n")
}

func headers(w http.ResponseWriter, req *http.Request) {
	for name, headers := range req.Header {
		for _, h := range headers {
			fmt.Fprintf(w, "%v: %v\n", name, h)
		}
	}
}

func drawImage(w http.ResponseWriter, req *http.Request) {
	fb, err := framebuffer.Open("/dev/" + fbNum)
	if err != nil {
		panic(err)
	}
	img, _, err := image.Decode(req.Body)
	if err != nil {
		panic(err)
	}
	draw.Draw(fb, fb.Bounds(), img, image.ZP, draw.Src)
}

func exit(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Received exit request, shutting down...")
	os.Exit(0)
}

func main() {
	err := rpio.Open()
	if err != nil {
		panic(err)
	}
	backlight := rpio.Pin(22)
	backlight.Output() // Output mode
	backlight.High()   // Set pin High

	http.HandleFunc("/hello", hello)
	http.HandleFunc("/rgb", rgb)
	http.HandleFunc("/image", drawImage)
	http.HandleFunc("/headers", headers)
	http.HandleFunc("/qr", qr)
	http.HandleFunc("/exit", exit)

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

	file := "/var/run/pibox/framebuffer.sock"
	os.Remove(file)
	fmt.Printf("Listening on socket: %s\n", file)
	listener, err := net.Listen("unix", file)
	os.Chmod(file, 0777)
	if err != nil {
		log.Fatalf("Could not listen on %s: %v", file, err)
		return
	}
	defer listener.Close()
	if err = http.Serve(listener, nil); err != nil {
		log.Fatalf("Could not start HTTP server: %v", err)
	}
}
