package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

func standIn(hue float64) ([]byte, error) {
	const size = 160
	ground := hsl(hue, 0.45, 0.22)
	light := hsl(hue, 0.70, 0.62)
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			dx, dy := float64(x-size/2), float64(y-size*2/5)
			c := ground
			if math.Hypot(dx, dy) < size/4 {
				c = light
			}
			img.SetRGBA(x, y, c)
		}
	}
	var b bytes.Buffer
	err := png.Encode(&b, img)
	return b.Bytes(), err
}

func hsl(h, s, l float64) color.RGBA {
	c := (1 - math.Abs(2*l-1)) * s
	hp := math.Mod(h, 360) / 60
	x := c * (1 - math.Abs(math.Mod(hp, 2)-1))
	var r, g, b float64
	switch {
	case hp < 1:
		r, g = c, x
	case hp < 2:
		r, g = x, c
	case hp < 3:
		g, b = c, x
	case hp < 4:
		g, b = x, c
	case hp < 5:
		r, b = x, c
	default:
		r, b = c, x
	}
	m := l - c/2
	to := func(v float64) uint8 { return uint8(math.Round((v + m) * 255)) }
	return color.RGBA{to(r), to(g), to(b), 255}
}
