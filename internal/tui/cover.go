package tui

import (
	"bytes"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"image"
	"image/color"
	_ "image/jpeg" // the cover formats the library accepts
	_ "image/png"
	"math"
	"net/url"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"music-manager/internal/coverimg"
)

// The cover is drawn with half-block characters: every character cell shows
// two pixels, one above the other (the upper half block "▀" in the colour of
// the top pixel, on a background of the bottom pixel's colour). That makes
// terminal "pixels" nearly square and needs nothing but colour support.

// thumbMaxPixels is the longest side of the shrunken copy that is kept in
// memory. It is plenty for the few dozen cells the inspector shows.
const thumbMaxPixels = 128

// cell is one character cell of a thumbnail: its upper and lower pixel.
type cell struct{ top, bottom color.RGBA }

// thumb is a decoded cover, shrunk, with a description of the original.
type thumb struct {
	img  *image.RGBA
	info string // e.g. "500×500 JPEG"

	// For terminals with the graphics protocol: the cover as a PNG of at most
	// kittyMaxPixels, and its size in pixels.
	kitty  []byte
	kw, kh int
}

// describeCover says how big a cover image is and what format.
func describeCover(data []byte) (string, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d×%d %s", cfg.Width, cfg.Height, strings.ToUpper(format)), nil
}

// loadThumb decodes a cover and shrinks it.
func loadThumb(data []byte) (thumb, error) { return loadThumbFor(data, false) }

// loadThumbFor is loadThumb that also prepares the full-resolution picture
// for the graphics protocol if asked to.
func loadThumbFor(data []byte, graphics bool) (thumb, error) {
	info, err := describeCover(data)
	if err != nil {
		return thumb{}, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return thumb{}, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if longest := max(w, h); longest > thumbMaxPixels {
		w, h = max(w*thumbMaxPixels/longest, 1), max(h*thumbMaxPixels/longest, 1)
	}
	th := thumb{img: coverimg.Resize(img, w, h), info: info}
	if graphics {
		// If this fails the blocks are still there.
		if th.kitty, th.kw, th.kh, err = encodeForKitty(data); err != nil {
			th.kitty = nil
		}
	}
	return th, nil
}

// thumbCells fits img into a box of cols×rows character cells, keeping its
// proportions, and returns the cells.
func thumbCells(img image.Image, cols, rows int) [][]cell {
	b := img.Bounds()
	if cols < 1 || rows < 1 || b.Dx() < 1 || b.Dy() < 1 {
		return nil
	}
	boxW, boxH := cols, 2*rows
	scale := math.Min(float64(boxW)/float64(b.Dx()), float64(boxH)/float64(b.Dy()))
	w := min(max(int(math.Round(float64(b.Dx())*scale)), 1), boxW)
	h := max(int(math.Round(float64(b.Dy())*scale)), 2)
	h = min(h+h%2, boxH) // whole cells: an even number of pixel rows

	px := coverimg.Resize(img, w, h)
	cells := make([][]cell, h/2)
	for y := range cells {
		cells[y] = make([]cell, w)
		for x := 0; x < w; x++ {
			cells[y][x] = cell{px.RGBAAt(x, 2*y), px.RGBAAt(x, 2*y+1)}
		}
	}
	return cells
}

func hex(c color.RGBA) lipgloss.Color {
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B))
}

// renderThumb draws cells, one line per row of cells.
func renderThumb(cells [][]cell) []string {
	lines := make([]string, len(cells))
	for y, row := range cells {
		var b strings.Builder
		for _, c := range row {
			b.WriteString(lipgloss.NewStyle().Foreground(hex(c.top)).Background(hex(c.bottom)).Render("▀"))
		}
		lines[y] = b.String()
	}
	return lines
}

// thumbMsg delivers a cover that has been loaded in the background.
type thumbMsg struct {
	sha string
	thumb
}

// wantThumb starts loading the cover of the selected track if it is not
// loaded or loading already.
func (m Model) wantThumb() tea.Cmd {
	r, ok := m.selectedRow()
	if !ok || r.Kind != kindTrack {
		return nil
	}
	sha := r.Track.CoverSHA256
	if _, have := m.thumbs[sha]; sha == "" || have || m.thumbPending[sha] {
		return nil
	}
	m.thumbPending[sha] = true
	lib, graphics := m.lib, m.gfx != nil
	return func() tea.Msg {
		data, err := lib.CoverImage(sha)
		if err == nil {
			var th thumb
			if th, err = loadThumbFor(data, graphics); err == nil {
				return thumbMsg{sha, th}
			}
		}
		return thumbMsg{sha, thumb{info: "unreadable image"}}
	}
}

// cleanPath turns what was typed or pasted as a file name into a path: it
// drops quotes and shell escapes and a file:// prefix, and expands ~.
func cleanPath(s string) string {
	s = strings.TrimSpace(s)
	if n := len(s); n >= 2 && (s[0] == '\'' && s[n-1] == '\'' || s[0] == '"' && s[n-1] == '"') {
		s = s[1 : n-1]
	}
	if rest, ok := strings.CutPrefix(s, "file://"); ok {
		if p, err := url.PathUnescape(rest); err == nil {
			s = p
		}
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	s = b.String()
	if s == "~" || strings.HasPrefix(s, "~/") {
		s = os.Getenv("HOME") + s[1:]
	}
	return s
}
