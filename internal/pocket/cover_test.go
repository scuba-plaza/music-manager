package pocket

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"music-manager/internal/manifest"
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
	if got, want := u16s(files["PLAYER/HEADER.BIN"][4:8]), []int{1, 2}; !slices.Equal(got, want) {
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

func TestAMosaicShowsEachCoverOnceInColumnsOrAGrid(t *testing.T) {
	yellow, purple := color.RGBA{0xE8, 0xC8, 0x40, 255}, color.RGBA{0x70, 0x40, 0x90, 255}
	red, blue, green := color.RGBA{0xF0, 0x56, 0x4D, 255}, color.RGBA{0x9D, 0xB4, 0xD1, 255}, color.RGBA{0x50, 0xA0, 0x60, 255}
	split := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			split.SetRGBA(x, y, map[bool]color.RGBA{true: yellow, false: purple}[y < 32])
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, split); err != nil {
		t.Fatal(err)
	}
	lib := sample()
	lib.Covers = map[string][]byte{"red": solid(t, red, 64, 64), "blue": solid(t, blue, 64, 64), "green": solid(t, green, 64, 64), "split": b.Bytes()}
	for i, sha := range []string{"red", "blue", "green", "split", "red"} {
		lib.Tracks[i].CoverSHA256 = sha
	}
	lib.Playlists = []manifest.Playlist{
		{Name: "Two", Tracks: []string{"t4", "t1", "t5"}},
		{Name: "Three", Tracks: []string{"t1", "t2", "t5", "t3"}},
		{Name: "Four", Tracks: []string{"t1", "t2", "t3", "t4"}},
	}
	mosaics := Files(lib)["PLAYER/MOSAIC.BIN"]
	if len(mosaics) != 3*38*38*2 {
		t.Fatalf("MOSAIC holds %d bytes", len(mosaics))
	}
	c565 := func(c color.RGBA) uint16 { return uint16(c.R>>3)<<11 | uint16(c.G>>2)<<5 | uint16(c.B>>3) }
	type at struct{ x, y int }
	cases := []map[at]color.RGBA{
		{{0, 0}: yellow, {0, 17}: yellow, {0, 21}: purple, {0, 37}: purple, {18, 37}: purple, {19, 0}: red, {37, 37}: red},
		{{0, 0}: red, {12, 37}: red, {13, 0}: blue, {24, 37}: blue, {25, 0}: green, {37, 37}: green},
		{{0, 0}: red, {37, 0}: blue, {0, 37}: green, {19, 19}: yellow, {37, 37}: purple},
	}
	for i, want := range cases {
		tile := mosaics[i*38*38*2:]
		for p, c := range want {
			if got := binary.LittleEndian.Uint16(tile[2*(p.y*38+p.x):]); got != c565(c) {
				t.Errorf("%s at %v = %#x, want %#x", lib.Playlists[i].Name, p, got, c565(c))
			}
		}
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

func TestArtistsAndGenresGetAMosaicOfTheirSongsCovers(t *testing.T) {
	lib := withCovers(t)
	lib.Tracks[4].CoverSHA256 = "blue"
	files := Files(lib)
	red := uint16(0xF2A9)
	blue := uint16(0x9D>>3)<<11 | uint16(0xB4>>2)<<5 | uint16(0xD1>>3)
	cases := []struct {
		mosaics, records string
		size             int
		corners          map[string][]uint16
	}{
		{"ARTMOS.BIN", "ARTISTS.BIN", 24, map[string][]uint16{
			"Leon Bridges":        {red, blue, red, blue},
			"Gustavo Santaolalla": {blue, blue, blue, blue},
			"عبد الحليم حافظ":     nil,
		}},
		{"GENMOS.BIN", "GENRES.BIN", 20, map[string][]uint16{
			"Soul":       {red, blue, red, blue},
			"Soundtrack": {blue, blue, blue, blue},
			"طرب":        nil,
		}},
	}
	for _, c := range cases {
		count := len(files["PLAYER/"+c.records]) / c.size
		data := files["PLAYER/"+c.mosaics]
		if len(data) != count*38*38*2 {
			t.Fatalf("%s holds %d bytes for %d records", c.mosaics, len(data), count)
		}
		seen := 0
		for i := range count {
			name := record(files, c.records, c.size, i).str()
			want, ok := c.corners[name]
			if !ok {
				continue
			}
			seen++
			tile := data[i*38*38*2 : (i+1)*38*38*2]
			pixel := func(x, y int) uint16 { return binary.LittleEndian.Uint16(tile[2*(y*38+x):]) }
			if want == nil {
				for p := 2; p < len(tile); p += 2 {
					if tile[p] != tile[0] || tile[p+1] != tile[1] {
						t.Fatalf("%s %q has no covers but is not one plain colour", c.mosaics, name)
					}
				}
				continue
			}
			if got := []uint16{pixel(0, 0), pixel(20, 0), pixel(0, 20), pixel(37, 37)}; !slices.Equal(got, want) {
				t.Errorf("%s %q corners = %#x, want %#x", c.mosaics, name, got, want)
			}
		}
		if seen != len(c.corners) {
			t.Errorf("%s: found %d of %d names", c.mosaics, seen, len(c.corners))
		}
	}
}
