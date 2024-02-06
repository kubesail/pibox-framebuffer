package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	// "time"

	pfb "github.com/kubesail/pibox-framebuffer/pkg"
	_ "github.com/kubesail/pibox-framebuffer/statik"

	"github.com/stianeikeland/go-rpio/v4"
)

const DefaultDiskMountPrefix = "/var/lib/rancher"
const DefaultListenHost = "localhost"
const DefaultListenPort = "2019"

func main() {
	listenHost, ok := os.LookupEnv("HOST")
	if !ok {
		listenHost = DefaultListenHost
	}

	listenPort, ok := os.LookupEnv("PORT")
	if !ok {
		listenPort = DefaultListenPort
	}
	
	diskMountPrefix, ok := os.LookupEnv("DISK_MOUNT_PREFIX")
	if !ok {
		diskMountPrefix = DefaultDiskMountPrefix
	}

	buffer := pfb.NewFrameBuffer(pfb.DefaultScreenSize, true, diskMountPrefix)

	err := rpio.Open()
	if err == nil {
		backlight := rpio.Pin(22)
		backlight.Output() // Output mode
		backlight.High()   // Set pin High
		buffer.Splash()
		// time.AfterFunc(6*time.Second, stats)
		// time.AfterFunc(0*time.Second, buffer.Stats)
	} else {
		fmt.Fprintf(os.Stderr, "Could not connect to framebuffer screen: %v\n", err)
	}

	exit := func(http.ResponseWriter, *http.Request) {
		buffer.Exit()
		os.Exit(0)
	}

	// http.HandleFunc("/rgb", buffer.RGB)
	http.HandleFunc("/image", buffer.DrawImage)
	// http.HandleFunc("/gif", buffer.DrawGIF)
	// http.HandleFunc("/text", buffer.TextRequest)
	// http.HandleFunc("/stats/on", buffer.EnableStats)
	// http.HandleFunc("/qr", buffer.QR)
	// http.HandleFunc("/disk-stats", buffer.DiskStats)
	http.HandleFunc("/exit", exit)

	fmt.Printf("Listening on %s:%s\n", listenHost, listenPort)
	// listen on localhost only	
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", listenHost, listenPort))
	if err != nil {
		log.Fatalf("Could not listen on %s:%s, %v", listenPort, err)
		return
	}
	defer listener.Close()

	if err = http.Serve(listener, nil); err != nil {
		log.Fatalf("Could not start HTTP server: %v", err)
	}
}
