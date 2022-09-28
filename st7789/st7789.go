package st7789

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"time"

	"periph.io/x/conn/v3"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
)

// DefaultOpts is the recommended default options.
var DefaultOpts = Opts{
	W: 240,
	H: 240,
}

// Opts defines the options for the device.
type Opts struct {
	W int16
	H int16
}

func NewSPI(p spi.Port, dc gpio.PinOut, opts *Opts) (*Device, error) {
	if dc == gpio.INVALID {
		return nil, errors.New("ssd1306: use nil for dc to use 3-wire mode, do not use gpio.INVALID")
	}
	bits := 8
	if err := dc.Out(gpio.Low); err != nil {
		return nil, err
	}
	c, err := p.Connect(80*physic.MegaHertz, spi.Mode0, bits)
	if err != nil {
		return nil, err
	}

	pin := gpioreg.ByName("GPIO22")
	if err = pin.Out(gpio.Low); err != nil {
		panic(err)
	}
	time.Sleep(100 * time.Millisecond)
	if err = pin.Out(gpio.High); err != nil {
		panic(err)
	}

	return newDev(c, opts, dc)
}

// Device is an open handle to the display controller.
type Device struct {
	// Communication
	c    conn.Conn
	dc   gpio.PinOut
	rect image.Rectangle

	rotation                      Rotation
	width                         int16
	height                        int16
	rowOffsetCfg, rowOffset       int16
	columnOffset, columnOffsetCfg int16
	isBGR                         bool
	batchLength                   int32
	backlight                     gpio.PinIO
}

func (d *Device) String() string {
	return fmt.Sprintf("st7789.Device{%s, %s, %s}", d.c, d.dc, d.rect.Max)
}

// Bounds implements display.Drawer. Min is guaranteed to be {0, 0}.
func (d *Device) Bounds() image.Rectangle {
	return d.rect
}

// PowerOff the display
func (d *Device) PowerOff() error {
	return d.backlight.Out(gpio.Low)
}

// PowerOn the display
func (d *Device) PowerOn() error {
	return d.backlight.Out(gpio.High)
}

// Invert the display (black on white vs white on black).
func (d *Device) Invert(blackOnWhite bool) {
	b := byte(0xA6)
	if blackOnWhite {
		b = 0xA7
	}
	d.Command(b)
}

func newDev(c conn.Conn, opts *Opts, dc gpio.PinOut) (*Device, error) {
	d := &Device{
		c:           c,
		dc:          dc,
		rect:        image.Rect(0, 0, int(opts.W), int(opts.H)),
		rotation:    NO_ROTATION,
		width:       opts.W,
		height:      opts.H,
		batchLength: int32(opts.W),
		backlight:   gpioreg.ByName("GPIO22"),
	}
	d.batchLength = d.batchLength & 1

	d.Command(SWRESET)
	time.Sleep(150 * time.Millisecond)

	d.Command(MADCTL)
	d.Data(0x70)

	d.Command(FRMCTR2)
	d.SendData([]byte{0x0C, 0x0C, 0x00, 0x33, 0x33})

	d.Command(COLMOD)
	d.Data(0x05)

	d.Command(GCTRL)
	d.Data(0x14)

	d.Command(VCOMS)
	d.Data(0x37)

	d.Command(LCMCTRL)
	d.Data(0x2C)

	d.Command(VDVVRHEN)
	d.Data(0x01)

	d.Command(VRHS)
	d.Data(0x12)

	d.Command(VDVS)
	d.Data(0x20)

	d.Command(0xD0)
	d.SendData([]byte{0xA4, 0xA1})

	d.Command(FRCTRL2)
	d.Data(0x0F)

	d.Command(GMCTRP1)
	d.SendData([]byte{0xD0, 0x04, 0x0D, 0x11, 0x13, 0x2B, 0x3F, 0x54, 0x4C, 0x18, 0x0D, 0x0B, 0x1F, 0x23})

	d.Command(GMCTRN1)
	d.SendData([]byte{0xD0, 0x04, 0x0C, 0x11, 0x13, 0x2C, 0x3F, 0x44, 0x51, 0x2F, 0x1F, 0x1F, 0x20, 0x23})

	d.Command(INVON)

	d.Command(SLPOUT)

	d.Command(DISPON)

	return d, nil
}

func (d *Device) SetWindow() {
	x1 := d.width - 1
	y1 := d.height - 1
	y0 := 0
	x0 := 0

	d.Command(CASET)
	d.Data(byte(x0 >> 8))
	d.Data(byte(x0 & 0xFF))
	d.Data(byte(x1 >> 8))
	d.Data(byte(x1 & 0xFF))

	d.Command(RASET)
	d.Data(byte(y0 >> 8))
	d.Data(byte(y0 & 0xFF))
	d.Data(byte(y1 >> 8))
	d.Data(byte(y1 & 0xFF))

	d.Command(RAMWR)
	d.Data(0x89)
}

func (d *Device) SendData(c []byte) error {
	if err := d.dc.Out(gpio.High); err != nil {
		return err
	}
	return d.c.Tx(c, nil)
}

func (d *Device) SendCommand(c []byte) error {
	if err := d.dc.Out(gpio.Low); err != nil {
		return err
	}
	return d.c.Tx(c, nil)
}

// FillRectangle fills a rectangle at a given coordinates with a color
func (d *Device) FillRectangle(x, y, width, height int16, c color.RGBA) error {
	k, i := d.Size()
	if x < 0 || y < 0 || width <= 0 || height <= 0 ||
		x >= k || (x+width) > k || y >= i || (y+height) > i {
		return errors.New("rectangle coordinates outside display area")
	}
	d.SetWindow()
	c565 := RGBATo565(c)
	c1 := uint8(c565)
	c2 := uint8(c565 >> 8)

	data := make([]uint8, 240*2)
	for i := int32(0); i < 240; i++ {
		data[i*2] = c1
		data[i*2+1] = c2
	}
	j := int32(width) * int32(height)
	for j > 0 {
		if j >= 240 {
			d.SendData(data)
		} else {
			d.SendData(data[:j*2])
		}
		j -= 240
	}
	return nil
}

// Size returns the current size of the display.
func (d *Device) Size() (w, h int16) {
	if d.rotation == NO_ROTATION || d.rotation == ROTATION_180 {
		return d.width, d.height
	}
	return 240, 240
}

// RGBATo565 converts a color.RGBA to uint16 used in the display
func RGBATo565(c color.RGBA) uint16 {
	r, g, b, _ := c.RGBA()
	return uint16((r & 0xF800) +
		((g & 0xFC00) >> 5) +
		((b & 0xF800) >> 11))
}

// SetPixel sets a pixel in the screen
func (d *Device) SetPixel(x int16, y int16, c color.RGBA) {
	if x < 0 || y < 0 ||
		(((d.rotation == NO_ROTATION || d.rotation == ROTATION_180) && (x >= d.width || y >= d.height)) ||
			((d.rotation == ROTATION_90 || d.rotation == ROTATION_270) && (x >= d.height || y >= d.width))) {
		return
	}
	d.FillRectangle(x, y, 1, 1, c)
}

// FillScreen fills the screen with a given color
func (d *Device) FillScreen(c color.RGBA) {
	if d.rotation == NO_ROTATION || d.rotation == ROTATION_180 {
		d.FillRectangle(0, 0, d.width, d.height, c)
	} else {
		d.FillRectangle(0, 0, d.height, d.width, c)
	}
}

// SetRotation changes the rotation of the device (clock-wise)
func (d *Device) SetRotation(rotation Rotation) {
	madctl := uint8(0)
	switch rotation % 4 {
	case 0:
		madctl = MADCTL_MX | MADCTL_MY
		d.rowOffset = d.rowOffsetCfg
		d.columnOffset = d.columnOffsetCfg
		break
	case 1:
		madctl = MADCTL_MY | MADCTL_MV
		d.rowOffset = d.columnOffsetCfg
		d.columnOffset = d.rowOffsetCfg
		break
	case 2:
		d.rowOffset = 0
		d.columnOffset = 0
		break
	case 3:
		madctl = MADCTL_MX | MADCTL_MV
		d.rowOffset = 0
		d.columnOffset = 0
		break
	}
	if d.isBGR {
		madctl |= MADCTL_BGR
	}
	d.Command(MADCTL)
	d.Data(madctl)
}

// IsBGR changes the color mode (RGB/BGR)
func (d *Device) IsBGR(bgr bool) {
	d.isBGR = bgr
}

// InverColors inverts the colors of the screen
func (d *Device) InvertColors(invert bool) {
	if invert {
		d.Command(INVON)
	} else {
		d.Command(INVOFF)
	}
}

// Command sends a command to the device
func (d *Device) Command(cmd uint8) {
	d.SendCommand([]byte{cmd})
}

// Data sends data to the device
func (d *Device) Data(data uint8) {
	d.SendData([]byte{data})
}

// DrawFastVLine draws a vertical line faster than using SetPixel
func (d *Device) DrawFastVLine(x, y0, y1 int16, c color.RGBA) {
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	d.FillRectangle(x, y0, 1, y1-y0+1, c)
}

// DrawFastHLine draws a horizontal line faster than using SetPixel
func (d *Device) DrawFastHLine(x0, x1, y int16, c color.RGBA) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	d.FillRectangle(x0, y, x1-x0+1, 1, c)
}

func (d *Device) DrawImage(reader io.Reader) {
	d.SetWindow()
	img, _, err := image.Decode(reader)
	if err != nil {
		log.Fatal(err)
	}

	d.DrawRAW(img)
}

func (d *Device) DrawRAW(img image.Image) {
	d.SetWindow()
	rect := img.Bounds()
	rgbaimg := image.NewRGBA(rect)
	draw.Draw(rgbaimg, rect, img, rect.Min, draw.Src)

	np := []uint8{}
	for i := 0; i < 240; i++ {
		for j := 0; j < 240; j++ {
			rgba := rgbaimg.At(int(d.width)-i, j).(color.RGBA)
			c565 := RGBATo565(rgba)
			c1 := uint8(c565)
			c2 := uint8(c565 >> 8)
			np = append(np, c1, c2)
		}
	}

	for i := 0; i < len(np); i += 4096 {
		d.SendData(np[i : i+4096])
	}
}
