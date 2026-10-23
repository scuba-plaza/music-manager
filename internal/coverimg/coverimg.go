// Package coverimg makes cover images into the one kind of file the mp3
// player opens: a baseline JPEG with a JFIF header, 4:2:0, of a modest size.
// The player's "Photos" menu refuses progressive and 4:4:4 files, and does
// not list PNG files at all.
package coverimg

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png" // a cover may be given as a PNG
	"math"
)

// MaxEdge is the most pixels a cover has along its longer side. The player's
// screen is small, and a small file opens faster and fits its memory.
const MaxEdge = 480

// Quality is the JPEG quality (1 to 100) covers are written with.
const Quality = 85

// jfif is the header that follows the start of a JFIF file: version 1.01, no
// units, a 1:1 pixel shape and no thumbnail. Go's encoder writes none, and
// every file the player is known to open has one.
var jfif = []byte{
	0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00,
	0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00,
}

// Normalize turns a JPEG or PNG image into a cover the player opens. A JPEG
// that is one already comes back as it is, so that normalising twice costs no
// quality.
func Normalize(data []byte) ([]byte, error) {
	if suits(data) {
		return data, nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("cover must be a JPEG or PNG image: %w", err)
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, fitted(onWhite(img)), &jpeg.Options{Quality: Quality}); err != nil {
		return nil, err
	}
	out := b.Bytes()
	return append(append(out[:2:2], jfif...), out[2:]...), nil
}

// onWhite lays img on a white background, so that see-through areas do not
// turn black in a format without transparency.
func onWhite(img image.Image) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Over)
	return out
}

// fitted shrinks img to fit in MaxEdge pixels along its longer side, keeping
// its shape. A picture that is small enough is not changed, or enlarged.
func fitted(img *image.RGBA) image.Image {
	b := img.Bounds()
	long := max(b.Dx(), b.Dy())
	if long <= MaxEdge {
		return img
	}
	scale := float64(MaxEdge) / float64(long)
	w := max(int(math.Round(float64(b.Dx())*scale)), 1)
	h := max(int(math.Round(float64(b.Dy())*scale)), 1)
	return Resize(img, w, h)
}

// Resize scales img to w×h by averaging the pixels each new pixel covers
// (or repeating pixels when it grows).
func Resize(img image.Image, w, h int) *image.RGBA {
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		y0 := y * sh / h
		y1 := max(y0+1, (y+1)*sh/h)
		for x := 0; x < w; x++ {
			x0 := x * sw / w
			x1 := max(x0+1, (x+1)*sw/w)
			var r, g, bl, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					pr, pg, pb, _ := img.At(b.Min.X+sx, b.Min.Y+sy).RGBA() // premultiplied: transparent reads as black
					r, g, bl, n = r+uint64(pr>>8), g+uint64(pg>>8), bl+uint64(pb>>8), n+1
				}
			}
			out.SetRGBA(x, y, color.RGBA{uint8(r / n), uint8(g / n), uint8(bl / n), 255})
		}
	}
	return out
}

// suits reports whether data is a JPEG the player opens, and needs no change:
// a JFIF file and nothing else in the way of metadata, a baseline frame of
// grey or 4:2:0 colour, within MaxEdge.
func suits(data []byte) bool {
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 || !bytes.HasPrefix(data[2:], jfif) {
		return false
	}
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xff {
			return false
		}
		marker := data[i+1]
		if marker == 0xff {
			i++
			continue
		}
		size := int(binary.BigEndian.Uint16(data[i+2:]))
		if i+2+size > len(data) || size < 2 {
			return false
		}
		body := data[i+4 : i+2+size]
		switch {
		case marker >= 0xe1 && marker <= 0xef, marker == 0xfe:
			return false // Exif, comments and the like
		case marker == 0xc0:
			return baselineAndSmall(body)
		case marker >= 0xc1 && marker <= 0xcf && marker != 0xc4 && marker != 0xc8 && marker != 0xcc:
			return false // progressive, lossless, arithmetic coding
		}
		i += 2 + size
	}
	return false
}

// baselineAndSmall judges the body of a frame header: height, width and the
// sampling of each component.
func baselineAndSmall(body []byte) bool {
	if len(body) < 6 {
		return false
	}
	h, w := int(binary.BigEndian.Uint16(body[1:])), int(binary.BigEndian.Uint16(body[3:]))
	if max(w, h) > MaxEdge {
		return false
	}
	n := int(body[5])
	if len(body) < 6+3*n {
		return false
	}
	switch n {
	case 1:
		return true
	case 3:
		return body[7] == 0x22 && body[10] == 0x11 && body[13] == 0x11
	}
	return false
}
