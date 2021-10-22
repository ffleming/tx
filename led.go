package main

import (
	"image"
	"io/ioutil"
	"strings"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/conn/i2c/i2creg"
	"periph.io/x/periph/devices/ssd1306"
	"periph.io/x/periph/devices/ssd1306/image1bit"
	"periph.io/x/periph/host"

	log "github.com/sirupsen/logrus"
)

type RadioDisplay interface {
	Write(string)
	Close()
}

type OLEDDisplay struct {
	device *ssd1306.Dev
	font   font.Face
	bus    i2c.BusCloser
}

type NullDisplay struct{}

func (nd *NullDisplay) Write(s string) {
	log.Infof("Display write: %q", s)
}

func (nd *NullDisplay) Close() {
	log.Info("Display close")
}

func NewOLEDDisplay() (*OLEDDisplay, error) {
	if _, err := host.Init(); err != nil {
		log.Error(err)
		return nil, err
	}

	bus, err := i2creg.Open("")
	if err != nil {
		log.Error(err)
		return nil, err
	}

	dev, err := ssd1306.NewI2C(bus, &ssd1306.Opts{
		W:          128,
		H:          32,
		Sequential: true,
	})
	if err != nil {
		log.Error(err)
		return nil, err
	}

	ttfData, err := ioutil.ReadFile("/usr/share/fonts/truetype/lato/Lato-Black.ttf")
	if err != nil {
		log.Error(err)
		return nil, err
	}
	ttf, err := truetype.Parse(ttfData)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	ttfFace := truetype.NewFace(ttf, &truetype.Options{
		Size: 14,
		DPI:  72,
	})
	disp := &OLEDDisplay{
		device: dev,
		bus:    bus,
		font:   ttfFace,
	}
	return disp, nil
}

func (rd *OLEDDisplay) Write(s string) {
	log.Infof("Display write: %q)", s)
	arr := strings.SplitN(s, "\n", 2)
	if len(arr) < 2 {
		arr[1] = ""
	}

	// Draw on it.
	img := image1bit.NewVerticalLSB(rd.device.Bounds())
	dot := fixed.P(0, 16)
	drawer := font.Drawer{
		Dst:  img,
		Src:  &image.Uniform{image1bit.On},
		Face: rd.font,
		Dot:  dot,
	}
	drawer.DrawString(arr[0])
	drawer.Dot = fixed.P(0, 32)
	drawer.DrawString(arr[1])
	if err := rd.device.Draw(rd.device.Bounds(), img, image.Point{}); err != nil {
		log.Error(err)
	}
}

func (rd *OLEDDisplay) Close() {
	rd.bus.Close()
}
