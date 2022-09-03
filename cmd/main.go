package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	pfb "github.com/kubesail/pibox-framebuffer/pkg"
	_ "github.com/kubesail/pibox-framebuffer/statik"

	"github.com/stianeikeland/go-rpio/v4"
)

func main() {
	buffer := pfb.NewFrameBuffer(pfb.DefaultScreenSize, "/var/lib/rancher")

	err := rpio.Open()
	if err == nil {
		backlight := rpio.Pin(22)
		backlight.Output() // Output mode
		backlight.High()   // Set pin High
		buffer.Splash()
		// time.AfterFunc(6*time.Second, stats)
		time.AfterFunc(0*time.Second, buffer.Stats)
	} else {
		fmt.Fprintf(os.Stderr, "Could not connect to framebuffer screen: %v\n", err)
	}

	exit := func(http.ResponseWriter, *http.Request) {
		buffer.Exit()
		os.Exit(0)
	}

	http.HandleFunc("/rgb", buffer.RGB)
	http.HandleFunc("/image", buffer.DrawImage)
	http.HandleFunc("/gif", buffer.DrawGIF)
	http.HandleFunc("/text", buffer.TextRequest)
	http.HandleFunc("/stats/on", buffer.EnableStats)
	http.HandleFunc("/qr", buffer.QR)
	http.HandleFunc("/disk-stats", buffer.DiskStats)
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
