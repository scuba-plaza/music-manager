package tint

import (
	"image"
	"image/color"
	"math"
)

var (
	Ground  = color.RGBA{0x12, 0x14, 0x13, 255}
	Ink     = color.RGBA{0xF0, 0xEF, 0xE8, 255}
	OnTint  = color.RGBA{0x10, 0x12, 0x11, 255}
	Neutral = color.RGBA{0xF0, 0xEF, 0xE8, 255}
)

const (
	minContrast = 4.5
	minChroma   = 0.12
	hueBins     = 24
)

func Mix(over, under color.RGBA, amount float64) color.RGBA {
	ch := func(a, b uint8) uint8 { return uint8(math.Round(float64(a)*amount + float64(b)*(1-amount))) }
	return color.RGBA{ch(over.R, under.R), ch(over.G, under.G), ch(over.B, under.B), 255}
}

func luminance(c color.RGBA) float64 {
	lin := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

func Contrast(a, b color.RGBA) float64 {
	la, lb := luminance(a), luminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

func Passes(c color.RGBA) bool {
	if Contrast(OnTint, c) < minContrast {
		return false
	}
	for _, amount := range []float64{0.20, 0.14} {
		surface := Mix(c, Ground, amount)
		if Contrast(Mix(Ink, surface, 0.68), surface) < minContrast {
			return false
		}
	}
	return true
}

func Fit(c color.RGBA) color.RGBA {
	if Passes(c) {
		return c
	}
	h, s, l := toHSL(c)
	for step := 1; step <= 100; step++ {
		for _, sign := range []float64{1, -1} {
			candidate := fromHSL(h, s, l+sign*float64(step)/100)
			if Passes(candidate) {
				return candidate
			}
		}
	}
	return Neutral
}

func Pick(img image.Image) color.RGBA {
	b := img.Bounds()
	stride := max(1, max(b.Dx(), b.Dy())/64)
	var weight [hueBins]float64
	var sum [hueBins][3]float64
	for y := b.Min.Y; y < b.Max.Y; y += stride {
		for x := b.Min.X; x < b.Max.X; x += stride {
			c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
			h, s, l := toHSL(c)
			chroma := s * (1 - math.Abs(2*l-1))
			if chroma < minChroma {
				continue
			}
			bin := int(h*hueBins) % hueBins
			weight[bin] += chroma
			sum[bin][0] += chroma * float64(c.R)
			sum[bin][1] += chroma * float64(c.G)
			sum[bin][2] += chroma * float64(c.B)
		}
	}
	best := -1
	for i := range weight {
		if weight[i] > 0 && (best < 0 || weight[i] > weight[best]) {
			best = i
		}
	}
	if best < 0 {
		return Neutral
	}
	w := weight[best]
	avg := color.RGBA{uint8(math.Round(sum[best][0] / w)), uint8(math.Round(sum[best][1] / w)), uint8(math.Round(sum[best][2] / w)), 255}
	return Fit(avg)
}

func toHSL(c color.RGBA) (h, s, l float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	hi, lo := max(r, g, b), min(r, g, b)
	l = (hi + lo) / 2
	if hi == lo {
		return 0, 0, l
	}
	d := hi - lo
	s = d / (1 - math.Abs(2*l-1))
	switch hi {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h /= 6
	if h < 0 {
		h++
	}
	return h, s, l
}

func fromHSL(h, s, l float64) color.RGBA {
	l = min(1, max(0, l))
	c := (1 - math.Abs(2*l-1)) * s
	hp := h * 6
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
	to := func(v float64) uint8 { return uint8(math.Round(min(1, max(0, v+m)) * 255)) }
	return color.RGBA{to(r), to(g), to(b), 255}
}
