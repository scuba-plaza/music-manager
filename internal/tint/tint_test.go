package tint

import (
	"image"
	"image/color"
	"math/rand/v2"
	"testing"
)

func hex(v uint32) color.RGBA {
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
}

var board = []color.RGBA{hex(0xF0564D), hex(0xE8705F), hex(0x9DB4D1), hex(0xE1BE63), hex(0xE8A0A8), hex(0xEBCB92), hex(0xF0EFE8)}

func TestTheTintsOnTheFoundationsBoardAreKeptAsTheyAre(t *testing.T) {
	for _, c := range board {
		if got := Fit(c); got != c {
			t.Errorf("Fit(%v) = %v, want it unchanged", c, got)
		}
	}
}

func TestEveryFittedTintPassesBothContrastRules(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for range 2000 {
		c := color.RGBA{uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)), 255}
		got := Fit(c)
		if !Passes(got) {
			t.Fatalf("Fit(%v) = %v does not pass", c, got)
		}
	}
}

func TestPickFindsTheMostVividColourOfACover(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := range 40 {
		for x := range 40 {
			c := color.RGBA{90, 90, 95, 255}
			if x < 12 && y < 12 {
				c = color.RGBA{40, 90, 220, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	got := Pick(img)
	if !(got.B > got.R && got.B > got.G) {
		t.Fatalf("Pick = %v, want a blue", got)
	}
	if !Passes(got) {
		t.Fatalf("Pick = %v does not pass", got)
	}
}

func TestAGreyCoverGetsTheNeutralTint(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 10, 10))
	if got := Pick(img); got != Neutral {
		t.Fatalf("Pick = %v, want neutral", got)
	}
}
