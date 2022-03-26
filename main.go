package main

import (
	"github.com/gonutz/framebuffer"
	"image"
	"image/color"
	"image/draw"
	"fmt"
    "net/http"
	"encoding/json"
)

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
	fb, err := framebuffer.Open("/dev/fb0")
	if err != nil {
		panic(err)
	}
	defer fb.Close()
	magenta := image.NewUniform(color.RGBA{c.R, c.G, c.B, 255})
	draw.Draw(fb, fb.Bounds(), magenta, image.ZP, draw.Src)
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

func main() {
    http.HandleFunc("/hello", hello)
    http.HandleFunc("/rgb", rgb)
    http.HandleFunc("/headers", headers)
	fmt.Println("Listening on port 8321!")
    http.ListenAndServe(":8321", nil)
}
