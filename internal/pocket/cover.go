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

const (
	mosaicSize = 38
	mosaicPart = 19
)

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

	var mosaics []byte
	for _, p := range x.playlists {
		var covers []int
		for _, s := range p.songs {
			if c := trackCover[s]; c != none && !contains(covers, c) && len(covers) < 4 {
				covers = append(covers, c)
			}
		}
		tile := image.NewRGBA(image.Rect(0, 0, mosaicSize, mosaicSize))
		for y := range mosaicSize {
			for x := range mosaicSize {
				tile.SetRGBA(x, y, tint.Mix(tint.Neutral, tint.Ground, 0.20))
			}
		}
		for part := range 4 {
			if len(covers) == 0 {
				break
			}
			small := resize(images[covers[part%len(covers)]], mosaicPart)
			ox, oy := (part%2)*mosaicPart, (part/2)*mosaicPart
			for y := range mosaicPart {
				for x := range mosaicPart {
					tile.Set(ox+x, oy+y, small.At(x, y))
				}
			}
		}
		mosaics = rgb565(mosaics, tile)
	}
	files[Folder+"/MOSAIC.BIN"] = mosaics
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
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		y0, y1 := y*b.Dy()/size, max((y+1)*b.Dy()/size, y*b.Dy()/size+1)
		for x := range size {
			x0, x1 := x*b.Dx()/size, max((x+1)*b.Dx()/size, x*b.Dx()/size+1)
			var r, g, bl, n uint32
			for sy := y0; sy < y1; sy++ {
				row := img.Pix[img.PixOffset(b.Min.X, b.Min.Y+sy):]
				for sx := x0; sx < x1; sx++ {
					p := row[4*sx : 4*sx+3]
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
