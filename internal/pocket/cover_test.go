package pocket

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"slices"
	"testing"
	"time"
)

func solid(t *testing.T, c color.RGBA, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetRGBA(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func withCovers(t *testing.T) Library {
	lib := sample()
	red := color.RGBA{0xF0, 0x56, 0x4D, 255}
	blue := color.RGBA{0x9D, 0xB4, 0xD1, 255}
	lib.Tracks[0].CoverSHA256 = "red"
	lib.Tracks[1].CoverSHA256 = "red"
	lib.Tracks[2].CoverSHA256 = "blue"
	lib.Covers = map[string][]byte{"red": solid(t, red, 64, 64), "blue": solid(t, blue, 160, 90)}
	return lib
}

func TestCoversAreIndexedOncePerImageWithTheirTint(t *testing.T) {
	files := Files(withCovers(t))
	if got, want := u16s(files["PLAYER/HEADER.BIN"][4:8]), []int{1, 1}; !slices.Equal(got, want) {
		t.Fatalf("version = %v, want %v", got, want)
	}
	if got, want := u16s(files["PLAYER/TRKCOV.BIN"]), []int{0, 0, 1, 0xFFFF, 0xFFFF}; !slices.Equal(got, want) {
		t.Errorf("TRKCOV = %v, want %v", got, want)
	}
	if got, want := u16s(files["PLAYER/ALBCOV.BIN"]), []int{0, 1, 0xFFFF}; !slices.Equal(got, want) {
		t.Errorf("ALBCOV = %v, want %v", got, want)
	}
	if got, want := files["PLAYER/TINTS.BIN"], []byte{0xF0, 0x56, 0x4D, 0, 0x9D, 0xB4, 0xD1, 0}; !bytes.Equal(got, want) {
		t.Errorf("TINTS = % x, want % x", got, want)
	}
}

func TestCoversArePreRenderedAsRGB565AtEverySize(t *testing.T) {
	files := Files(withCovers(t))
	for _, size := range []int{22, 32, 36, 38, 104} {
		data := files[coverFile(size)]
		if len(data) != 2*size*size*2 {
			t.Fatalf("%s holds %d bytes, want two covers", coverFile(size), len(data))
		}
		if got := binary.LittleEndian.Uint16(data); got != 0xF2A9 {
			t.Errorf("%s first pixel = %#x, want 0xF2A9", coverFile(size), got)
		}
	}
}

func TestAPlaylistMosaicTakesFourCoversOfItsSongs(t *testing.T) {
	files := Files(withCovers(t))
	mosaic := files["PLAYER/MOSAIC.BIN"]
	if len(mosaic) != 38*38*2 {
		t.Fatalf("MOSAIC holds %d bytes", len(mosaic))
	}
	pixel := func(x, y int) uint16 { return binary.LittleEndian.Uint16(mosaic[2*(y*38+x):]) }
	blue565 := uint16(0x9D>>3)<<11 | uint16(0xB4>>2)<<5 | uint16(0xD1>>3)
	if pixel(0, 0) != blue565 || pixel(20, 0) != 0xF2A9 || pixel(0, 20) != blue565 || pixel(37, 37) != 0xF2A9 {
		t.Errorf("mosaic corners = %#x %#x %#x %#x", pixel(0, 0), pixel(20, 0), pixel(0, 20), pixel(37, 37))
	}
}

func TestLargeCoversAreProcessedQuickly(t *testing.T) {
	lib := sample()
	lib.Covers = map[string][]byte{}
	for i := range lib.Tracks {
		sha := string(rune('a' + i))
		lib.Tracks[i].CoverSHA256 = sha
		lib.Covers[sha] = solid(t, color.RGBA{uint8(40 * i), 90, 200, 255}, 1500, 1500)
	}
	start := time.Now()
	files := Files(lib)
	if took := time.Since(start); took > time.Duration(len(lib.Tracks))*60*time.Millisecond {
		t.Fatalf("%d covers of 1500 px took %v", len(lib.Tracks), took)
	}
	if got := len(files["PLAYER/TINTS.BIN"]); got != 4*len(lib.Tracks) {
		t.Fatalf("TINTS holds %d bytes", got)
	}
}
