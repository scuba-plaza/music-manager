package pocket

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"

	"music-manager/internal/tint"
)

var CoverSizes = []int{22, 32, 36, 38, 104}

const mosaicSize = 38

func coverFile(size int) string { return fmt.Sprintf("%s/COV%d.BIN", Folder, size) }

func writeCovers(lib Library, x *index, files map[string][]byte) {
	var images []*image.RGBA
	indexOf := map[string]int{}
	trackCover := make([]int, len(lib.Tracks))
	for i, t := range lib.Tracks {
		trackCover[i] = none
		if t.CoverSHA256 == "" {
			continue
		}
		at, ok := indexOf[t.CoverSHA256]
		if !ok {
			img, _, err := image.Decode(bytes.NewReader(lib.Covers[t.CoverSHA256]))
			if err != nil {
				continue
			}
			at = len(images)
			indexOf[t.CoverSHA256] = at
			images = append(images, prepare(img))
		}
		trackCover[i] = at
	}
	files[Folder+"/TRKCOV.BIN"] = u16(nil, trackCover...)

	var albums []int
	for _, a := range x.albums {
		cover := none
		for _, s := range a.songs {
			if trackCover[s] != none {
				cover = trackCover[s]
				break
			}
		}
		albums = append(albums, cover)
	}
	files[Folder+"/ALBCOV.BIN"] = u16(nil, albums...)

	var tints []byte
	for _, img := range images {
		c := tint.Pick(img)
		tints = append(tints, c.R, c.G, c.B, 0)
	}
	files[Folder+"/TINTS.BIN"] = tints

	for _, size := range CoverSizes {
		var data []byte
		for _, img := range images {
			data = rgb565(data, resize(img, size))
		}
		files[coverFile(size)] = data
	}

	files[Folder+"/MOSAIC.BIN"] = mosaics(x.playlists, images, trackCover)
	files[Folder+"/ARTMOS.BIN"] = mosaics(x.artists, images, trackCover)
	files[Folder+"/GENMOS.BIN"] = mosaics(x.genres, images, trackCover)
}

func mosaics(groups []*group, images []*image.RGBA, trackCover []int) []byte {
	var data []byte
	for _, g := range groups {
		var covers []int
		for _, s := range g.songs {
			if c := trackCover[s]; c != none && !contains(covers, c) && len(covers) < 4 {
				covers = append(covers, c)
			}
		}
		data = rgb565(data, mosaic(covers, images))
	}
	return data
}

func mosaic(covers []int, images []*image.RGBA) *image.RGBA {
	tile := image.NewRGBA(image.Rect(0, 0, mosaicSize, mosaicSize))
	place := func(cover, x0, y0, x1, y1 int) {
		draw.Draw(tile, image.Rect(x0, y0, x1, y1), fit(images[cover], x1-x0, y1-y0), image.Point{}, draw.Src)
	}
	switch n := len(covers); n {
	case 0:
		draw.Draw(tile, tile.Bounds(), image.NewUniform(tint.Mix(tint.Neutral, tint.Ground, 0.20)), image.Point{}, draw.Src)
	case 1:
		place(covers[0], 0, 0, mosaicSize, mosaicSize)
	case 2, 3:
		for i, c := range covers {
			place(c, (i*mosaicSize+n/2)/n, 0, ((i+1)*mosaicSize+n/2)/n, mosaicSize)
		}
	default:
		half := mosaicSize / 2
		for i, c := range covers {
			place(c, (i%2)*half, (i/2)*half, (i%2+1)*half, (i/2+1)*half)
		}
	}
	return tile
}

func contains(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

const workSize = 208

func prepare(img image.Image) *image.RGBA {
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	crop := image.Rect(0, 0, side, side).Add(image.Pt(b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2))
	square := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.Draw(square, square.Bounds(), img, crop.Min, draw.Src)
	if side <= workSize {
		return square
	}
	return resize(square, workSize)
}

func resize(img *image.RGBA, size int) *image.RGBA {
	return fit(img, size, size)
}

func fit(img *image.RGBA, w, h int) *image.RGBA {
	b := img.Bounds()
	cw, ch := b.Dx(), b.Dy()
	if cw*h > ch*w {
		cw = ch * w / h
	} else {
		ch = cw * h / w
	}
	left, top := b.Min.X+(b.Dx()-cw)/2, b.Min.Y+(b.Dy()-ch)/2
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		y0, y1 := top+y*ch/h, top+max((y+1)*ch/h, y*ch/h+1)
		for x := range w {
			x0, x1 := left+x*cw/w, left+max((x+1)*cw/w, x*cw/w+1)
			var r, g, bl, n uint32
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					p := img.Pix[img.PixOffset(sx, sy):]
					r += uint32(p[0])
					g += uint32(p[1])
					bl += uint32(p[2])
					n++
				}
			}
			o := out.PixOffset(x, y)
			out.Pix[o], out.Pix[o+1], out.Pix[o+2], out.Pix[o+3] = uint8(r/n), uint8(g/n), uint8(bl/n), 255
		}
	}
	return out
}
func rgb565(data []byte, img *image.RGBA) []byte {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.RGBAAt(x, y)
			v := uint16(c.R>>3)<<11 | uint16(c.G>>2)<<5 | uint16(c.B>>3)
			data = binary.LittleEndian.AppendUint16(data, v)
		}
	}
	return data
}
