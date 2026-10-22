package tui

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

var (
	red  = color.RGBA{255, 0, 0, 255}
	blue = color.RGBA{0, 0, 255, 255}
)

// twoTone is an image whose left half is red and right half blue.
func twoTone(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < w/2 {
				img.Set(x, y, red)
			} else {
				img.Set(x, y, blue)
			}
		}
	}
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestThumbCellsKeepTheImagesProportions(t *testing.T) {
	cases := []struct {
		w, h, cols, rows   int
		wantCols, wantRows int
	}{
		{100, 100, 20, 10, 20, 10}, // square into a box that is exactly square in pixels
		{100, 100, 40, 10, 20, 10}, // too wide a box: height decides
		{100, 100, 10, 40, 10, 5},  // too tall a box: width decides
		{200, 100, 20, 10, 20, 5},  // wide image
		{100, 200, 20, 10, 10, 10}, // tall image
		{1, 1, 8, 4, 8, 4},         // a pixel is blown up to fill the box
	}
	for _, c := range cases {
		cells := thumbCells(twoTone(c.w, c.h), c.cols, c.rows)
		if len(cells) != c.wantRows || (len(cells) > 0 && len(cells[0]) != c.wantCols) {
			t.Errorf("%dx%d into %dx%d cells: got %d rows of %d, want %d of %d", c.w, c.h, c.cols, c.rows, len(cells), len(cells[0]), c.wantRows, c.wantCols)
		}
	}
	if got := thumbCells(twoTone(10, 10), 0, 5); got != nil {
		t.Errorf("no room should give nothing, got %v", got)
	}
}

func TestThumbCellsCarryTheColoursOfTheImage(t *testing.T) {
	cells := thumbCells(twoTone(40, 40), 4, 2)
	if len(cells) != 2 || len(cells[0]) != 4 {
		t.Fatalf("shape %dx%d", len(cells), len(cells[0]))
	}
	for _, row := range cells {
		for x, c := range row {
			want := red
			if x >= 2 {
				want = blue
			}
			if c.top != want || c.bottom != want {
				t.Errorf("cell %d = %v/%v, want %v", x, c.top, c.bottom, want)
			}
		}
	}
}

func TestThumbCellsAverageWhatTheyCover(t *testing.T) {
	// A checkerboard of black and white pixels averages to grey.
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if (x+y)%2 == 0 {
				img.Set(x, y, color.White)
			} else {
				img.Set(x, y, color.Black)
			}
		}
	}
	c := thumbCells(img, 1, 1)[0][0]
	if c.top.R < 100 || c.top.R > 155 || c.bottom.G < 100 || c.bottom.G > 155 {
		t.Errorf("a checkerboard should average to mid grey, got %v / %v", c.top, c.bottom)
	}
}

func TestRenderThumbDrawsOneHalfBlockPerCell(t *testing.T) {
	lines := renderThumb(thumbCells(twoTone(40, 40), 6, 3))
	if len(lines) != 3 {
		t.Fatalf("%d lines", len(lines))
	}
	for _, l := range lines {
		plain := ansi.Strip(l)
		if plain != strings.Repeat("▀", 6) {
			t.Errorf("line = %q", plain)
		}
	}
}

func TestDescribeCoverGivesSizeAndFormat(t *testing.T) {
	var jbuf bytes.Buffer
	if err := jpeg.Encode(&jbuf, twoTone(30, 20), nil); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		data []byte
		want string
	}{
		"jpeg": {jbuf.Bytes(), "30×20 JPEG"},
		"png":  {encodePNG(t, twoTone(500, 500)), "500×500 PNG"},
	} {
		got, err := describeCover(c.data)
		if err != nil || got != c.want {
			t.Errorf("%s: %q, %v; want %q", name, got, err, c.want)
		}
	}
	if _, err := describeCover([]byte("not an image")); err == nil {
		t.Error("garbage described as an image")
	}
}

func TestLoadThumbDecodesAndShrinksAnImage(t *testing.T) {
	th, err := loadThumb(encodePNG(t, twoTone(800, 600)))
	if err != nil {
		t.Fatal(err)
	}
	b := th.img.Bounds()
	if b.Dx() > thumbMaxPixels || b.Dy() > thumbMaxPixels || b.Dx() <= b.Dy() {
		t.Errorf("shrunk to %v: it should fit in %d pixels and stay wide", b, thumbMaxPixels)
	}
	if th.info != "800×600 PNG" {
		t.Errorf("info = %q", th.info)
	}
	if _, err := loadThumb([]byte("nope")); err == nil {
		t.Error("garbage loaded")
	}
}

// coverModel is the browser library with a cover on "Flow" and the track selected.
func coverModel(t *testing.T) (Model, *editHook) {
	t.Helper()
	m, hook := editingModel(t, "Flow")
	tr := trackNamed(t, hook.Library, "Flow")
	app := appOf(t, hook.Library)
	if err := app.SetCover(tr.ID, encodePNG(t, twoTone(60, 60))); err != nil {
		t.Fatal(err)
	}
	m = send(t, m, m.load())
	return selectTrack(t, m, "Flow"), hook
}

func TestTheInspectorShowsThePictureAndItsSize(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	contains(t, m, "Cover")
	lacks(t, m, "▀")
	if !strings.Contains(screen(m), "none") {
		t.Error("a track without a cover should say none")
	}

	m, _ = coverModel(t)
	contains(t, m, "60×60 PNG", "▀")
}

func writeImage(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestChoosingACoverFromAFile(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	path := writeImage(t, "front.png", encodePNG(t, twoTone(32, 32)))
	m = keys(t, m, "right")
	for i := 0; i < len(trackFields)-1; i++ {
		m = keys(t, m, "down")
	}
	if trackFields[m.field] != fCover {
		t.Fatalf("on %v", trackFields[m.field])
	}
	m = keys(t, m, "enter")
	contains(t, m, "JPEG or PNG")
	m = typed(t, m, path)
	m = keys(t, m, "enter")

	if got := trackNamed(t, hook.Library, "Flow"); got.CoverSHA256 == "" {
		t.Fatal("the cover was not saved")
	}
	contains(t, m, "32×32 PNG", "▀", "Saved")

	m = keys(t, m, "u")
	if got := trackNamed(t, hook.Library, "Flow"); got.CoverSHA256 != "" {
		t.Error("undo did not remove the cover again")
	}
	lacks(t, m, "▀")
}

func TestAPastedPathMayBeQuotedEscapedOrUnderHome(t *testing.T) {
	path := writeImage(t, "my front.png", encodePNG(t, twoTone(16, 16)))
	t.Setenv("HOME", filepath.Dir(path))
	for name, typedPath := range map[string]string{
		"quoted":  "'" + path + "'",
		"dquoted": `"` + path + `"`,
		"escaped": strings.ReplaceAll(path, " ", `\ `),
		"home":    "~/my front.png",
		"spaces":  "  " + path + "  ",
	} {
		m, hook := editingModel(t, "Flow")
		m = keys(t, m, "right", "down", "down", "down", "down", "down", "down", "enter")
		m = typed(t, m, typedPath)
		m = keys(t, m, "enter")
		if got := trackNamed(t, hook.Library, "Flow"); got.CoverSHA256 == "" {
			t.Errorf("%s: not saved; screen:\n%s", name, screen(m))
		}
	}
}

func TestACoverThatCannotBeUsedIsExplained(t *testing.T) {
	text := writeImage(t, "notes.txt", []byte("not a picture"))
	for name, c := range map[string]struct{ path, want string }{
		"missing":  {"/no/such/file.png", "no such file"},
		"not-img":  {text, "JPEG or PNG"},
		"a folder": {filepath.Dir(text), "is a directory"},
	} {
		m, hook := editingModel(t, "Flow")
		m = keys(t, m, "right", "down", "down", "down", "down", "down", "down", "enter")
		m = typed(t, m, c.path)
		m = keys(t, m, "enter")
		contains(t, m, c.want)
		if !m.editing {
			t.Errorf("%s: a bad path must leave the prompt open", name)
		}
		if trackNamed(t, hook.Library, "Flow").CoverSHA256 != "" {
			t.Errorf("%s: a cover was saved", name)
		}
	}
}

func TestXRemovesTheCoverAndUndoBringsItBack(t *testing.T) {
	m, hook := coverModel(t)
	before := trackNamed(t, hook.Library, "Flow").CoverSHA256
	m = keys(t, m, "right", "down", "down", "down", "down", "down", "down", "x")
	if got := trackNamed(t, hook.Library, "Flow"); got.CoverSHA256 != "" {
		t.Fatalf("still has a cover: %s", got.CoverSHA256)
	}
	lacks(t, m, "▀")
	m = keys(t, m, "u")
	if got := trackNamed(t, hook.Library, "Flow"); got.CoverSHA256 != before {
		t.Error("undo did not restore the cover")
	}
	contains(t, m, "▀")
}

func TestEmptyPathRemovesTheCover(t *testing.T) {
	m, hook := coverModel(t)
	m = keys(t, m, "right", "down", "down", "down", "down", "down", "down", "enter", "enter")
	if trackNamed(t, hook.Library, "Flow").CoverSHA256 != "" {
		t.Error("an empty path should remove the cover")
	}
}
