package device_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"music-manager/internal/device"
)

func bigPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 1200, 800))
	for y := 0; y < 800; y++ {
		for x := 0; x < 1200; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestAGenericTargetKeepsTheCoverAsItWasGiven(t *testing.T) {
	big := bigPNG(t)
	data, mime, err := device.Generic.Cover(big)
	if err != nil || !bytes.Equal(data, big) || mime != "image/png" {
		t.Errorf("a PNG: %d bytes, %q, %v; want the same %d bytes as image/png", len(data), mime, err, len(big))
	}
	var asJPEG bytes.Buffer
	if err := jpeg.Encode(&asJPEG, image.NewRGBA(image.Rect(0, 0, 900, 900)), nil); err != nil {
		t.Fatal(err)
	}
	data, mime, err = device.Generic.Cover(asJPEG.Bytes())
	if err != nil || !bytes.Equal(data, asJPEG.Bytes()) || mime != "image/jpeg" {
		t.Errorf("a JPEG: %q, %v; want it untouched, as image/jpeg", mime, err)
	}
}

func TestTheWolfgangGetsABaselineJPEGThatFitsItsScreen(t *testing.T) {
	data, mime, err := device.Wolfgang.Cover(bigPNG(t))
	if err != nil || mime != "image/jpeg" {
		t.Fatalf("%q, %v", mime, err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != "jpeg" || cfg.Width != 480 || cfg.Height != 320 {
		t.Errorf("cover is %s %dx%d (%v), want a 480x320 jpeg", format, cfg.Width, cfg.Height, err)
	}
}

func TestTheNameOfTheCoverFileFollowsWhatIsWritten(t *testing.T) {
	for _, c := range []struct {
		dev       device.Target
		stored    string
		want      string
		singleToo bool
	}{
		{device.Generic, "image/jpeg", "cover.jpg", false},
		{device.Generic, "image/png", "cover.png", false},
		{device.Wolfgang, "image/jpeg", "cover.jpg", true},
		{device.Wolfgang, "image/png", "cover.jpg", true}, // it is shrunk to a JPEG
	} {
		if got := c.dev.CoverFile(c.stored); got != c.want {
			t.Errorf("%s.CoverFile(%s) = %q, want %q", c.dev.Name(), c.stored, got, c.want)
		}
		if got := c.dev.SingleCovers(); got != c.singleToo {
			t.Errorf("%s.SingleCovers() = %v, want %v", c.dev.Name(), got, c.singleToo)
		}
	}
}
