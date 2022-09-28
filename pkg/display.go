package pkg

import (
	"image"
	"image/color"
	"io"
	"log"
	"sync"

	"github.com/rubiojr/go-pirateaudio/st7789"
	"periph.io/x/conn/v3/driver/driverreg"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"
)

type Rotation uint8

const (
	NO_ROTATION  Rotation = 0
	ROTATION_90  Rotation = 1 // 90 degrees clock-wise rotation
	ROTATION_180 Rotation = 2
	ROTATION_270 Rotation = 3
)

var once sync.Once
var display *Display

type Display struct {
	p   spi.PortCloser
	dev *st7789.Device
}

func init() {
	if _, err := host.Init(); err != nil {
		log.Fatal(err)
	}

	if _, err := driverreg.Init(); err != nil {
		log.Fatal(err)
	}

}

func Init() (*Display, error) {
	var err error
	once.Do(func() {
		display = &Display{}
		display.p, err = spireg.Open("SPI0.1")
		if err != nil {
			return
		}
		// USE GPIO9 to send data/commands
		// https://pinout.xyz/pinout/pirate_audio_line_out#
		display.dev, err = st7789.NewSPI(display.p.(spi.Port), gpioreg.ByName("GPIO9"), &st7789.DefaultOpts)
	})

	return display, err
}

func (d *Display) Close() {
	d.p.Close()
}

func (d *Display) DrawImage(reader io.Reader) {
	d.dev.DrawImage(reader)
}

func (d *Display) DrawRAW(img image.Image) {
	d.dev.DrawRAW(img)
}

func (d *Display) Rotate(rotation Rotation) {
	d.dev.SetRotation(st7789.Rotation(rotation))
}

func (d *Display) FillScreen(c color.RGBA) {
	d.dev.FillScreen(c)
}

func (d *Display) SetPixel(x int16, y int16, c color.RGBA) {
	d.dev.SetPixel(x, y, c)
}

// PowerOff the display
func (d *Display) PowerOff() {
	d.dev.PowerOff()
}

// PowerOn the display
func (d *Display) PowerOn() {
	d.dev.PowerOn()
}

