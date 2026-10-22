package tui

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// The expected values below were taken from the output of kitty's own
// "kitten icat --unicode-placeholder" for a 12×6 cell image with the id
// 0x2157F8, not worked out from the code under test.

func TestPlaceholdersNameEveryCellOfTheImage(t *testing.T) {
	lines := placeholderLines(0x2157F8, 12, 6)
	if len(lines) != 6 {
		t.Fatalf("%d lines, want 6", len(lines))
	}
	for i, l := range lines {
		if got := ansi.StringWidth(l); got != 12 {
			t.Errorf("line %d is %d cells wide, want 12", i, got)
		}
		if got := strings.Count(l, "\U0010EEEE"); got != 12 {
			t.Errorf("line %d has %d placeholders, want 12", i, got)
		}
	}

	// Row 0, column 1 and row 5, column 11, as kitty writes them.
	cells := func(line string) []string {
		var out []string
		for _, c := range strings.Split(ansi.Strip(line), "\U0010EEEE")[1:] {
			out = append(out, "\U0010EEEE"+c)
		}
		return out
	}
	if got, want := cells(lines[0])[1], "\U0010EEEE̅̍"; got != want {
		t.Errorf("row 0, column 1 = %q, want %q", got, want)
	}
	if got, want := cells(lines[5])[11], "\U0010EEEE̽͌"; got != want {
		t.Errorf("row 5, column 11 = %q, want %q", got, want)
	}
}

func TestPlaceholdersCarryTheImageIDInTheirColour(t *testing.T) {
	line := placeholderLines(0x2157F8, 3, 1)[0]
	if !strings.HasPrefix(line, "\x1b[38;2;33;87;248m") {
		t.Errorf("the id 0x2157F8 is colour 33,87,248, but the line starts %q", line[:min(len(line), 24)])
	}
	if !strings.HasSuffix(line, "\x1b[39m") {
		t.Errorf("the colour must be switched off again: %q", line)
	}
}

func TestPlaceholdersAreCappedByWhatTheTableCanNumber(t *testing.T) {
	lines := placeholderLines(1, 500, 500)
	if len(lines) > maxPlaceholders || ansi.StringWidth(lines[0]) > maxPlaceholders {
		t.Errorf("%d lines of %d cells: the table only numbers %d", len(lines), ansi.StringWidth(lines[0]), maxPlaceholders)
	}
}

// parseAPC splits a string of graphics commands into their keys and payloads.
func parseAPC(t *testing.T, s string) (cmds []map[string]string, payloads []string) {
	t.Helper()
	re := regexp.MustCompile(`\x1b_G([^;\x1b]*)(?:;([^\x1b]*))?\x1b\\`)
	rest := s
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		keys := map[string]string{}
		for _, kv := range strings.Split(m[1], ",") {
			if k, v, ok := strings.Cut(kv, "="); ok {
				keys[k] = v
			}
		}
		cmds = append(cmds, keys)
		payloads = append(payloads, m[2])
		rest = strings.Replace(rest, m[0], "", 1)
	}
	if rest != "" {
		t.Fatalf("something besides graphics commands: %q", rest)
	}
	return cmds, payloads
}

func TestTransmittingAnImageCreatesAVirtualPlacementOfTheRightSize(t *testing.T) {
	data := encodePNG(t, twoTone(40, 30))
	cmds, payloads := parseAPC(t, transmitSeq(0x2157F8, data, 12, 6))
	if len(cmds) != 1 {
		t.Fatalf("a small image should be one command, got %d", len(cmds))
	}
	want := map[string]string{"a": "T", "f": "100", "U": "1", "q": "2", "C": "1", "i": "2185208", "c": "12", "r": "6"}
	for k, v := range want {
		if cmds[0][k] != v {
			t.Errorf("key %s = %q, want %q (all: %v)", k, cmds[0][k], v, cmds[0])
		}
	}
	got, err := base64.StdEncoding.DecodeString(payloads[0])
	if err != nil || !bytes.Equal(got, data) {
		t.Errorf("the payload is not the image: %v", err)
	}
}

func TestABigImageGoesInChunksTheTerminalCanAssemble(t *testing.T) {
	big := bytes.Repeat([]byte("0123456789abcdef"), 2000) // not a valid PNG, but the chunking does not care
	cmds, payloads := parseAPC(t, transmitSeq(7, big, 10, 5))
	if len(cmds) < 2 {
		t.Fatalf("%d commands for %d bytes", len(cmds), len(big))
	}
	var all strings.Builder
	for i, c := range cmds {
		if len(payloads[i]) > 4096 {
			t.Errorf("chunk %d is %d bytes: at most 4096 are allowed", i, len(payloads[i]))
		}
		last := i == len(cmds)-1
		if (c["m"] == "1") == last {
			t.Errorf("chunk %d of %d has m=%q", i+1, len(cmds), c["m"])
		}
		if i > 0 && len(c) != 1 {
			t.Errorf("after the first, a chunk carries only m: %v", c)
		}
		if len(payloads[i])%4 != 0 && !last {
			t.Errorf("chunk %d does not end on a base64 boundary", i)
		}
		all.WriteString(payloads[i])
	}
	got, err := base64.StdEncoding.DecodeString(all.String())
	if err != nil || !bytes.Equal(got, big) {
		t.Errorf("the chunks do not add up to the image: %v", err)
	}
}

func TestDeletingImages(t *testing.T) {
	cmds, _ := parseAPC(t, deleteSeq(0x2157F8))
	if len(cmds) != 1 || cmds[0]["a"] != "d" || cmds[0]["d"] != "I" || cmds[0]["i"] != "2185208" || cmds[0]["q"] != "2" {
		t.Errorf("delete one: %v", cmds)
	}
}

func TestImageIDsAreStableNonZeroAnd24Bits(t *testing.T) {
	seen := map[uint32]string{}
	for _, sha := range []string{"aa", "ab", "ba", "0000", "ffff", strings.Repeat("f", 64)} {
		for _, size := range [][2]int{{10, 5}, {10, 6}, {11, 5}} {
			id := imageID(sha, size[0], size[1])
			if id == 0 || id > 0xFFFFFF {
				t.Errorf("imageID(%q, %v) = %#x", sha, size, id)
			}
			if id != imageID(sha, size[0], size[1]) {
				t.Errorf("imageID is not stable")
			}
			key := sha + "/" + string(rune('0'+size[0]%10)) + string(rune('0'+size[1]%10))
			if other, dup := seen[id]; dup {
				t.Errorf("%s and %s share the id %#x", key, other, id)
			}
			seen[id] = key
		}
	}
}

func TestPlacementKeepsTheImagesProportionsWhateverTheCellsLookLike(t *testing.T) {
	cases := []struct {
		name                                 string
		imgW, imgH, boxC, boxR, cellW, cellH int
		wantC, wantR                         int
	}{
		{"square in a roomy box, cells twice as tall as wide", 600, 600, 40, 10, 10, 20, 20, 10},
		{"square, the width is the limit", 600, 600, 12, 30, 10, 20, 12, 6},
		{"wide image", 800, 400, 40, 20, 10, 20, 40, 10},
		{"tall image", 400, 800, 40, 10, 10, 20, 10, 10},
		{"cells nearly square", 600, 600, 40, 10, 10, 12, 12, 10},
		{"cell size unknown means twice as tall as wide", 600, 600, 40, 10, 0, 0, 20, 10},
		{"never less than one cell", 1000, 10, 4, 4, 10, 20, 4, 1},
	}
	for _, c := range cases {
		cols, rows := placementSize(c.imgW, c.imgH, c.boxC, c.boxR, c.cellW, c.cellH)
		if cols != c.wantC || rows != c.wantR {
			t.Errorf("%s: %d×%d cells, want %d×%d", c.name, cols, rows, c.wantC, c.wantR)
		}
	}
	if c, r := placementSize(600, 600, 0, 10, 10, 20); c != 0 || r != 0 {
		t.Errorf("no room should give nothing, got %d×%d", c, r)
	}
}

func TestEncodeForKittyKeepsSmallImagesAndShrinksBigOnes(t *testing.T) {
	small, w, h, err := encodeForKitty(encodePNG(t, twoTone(200, 100)))
	if err != nil || w != 200 || h != 100 {
		t.Fatalf("small: %dx%d, %v", w, h, err)
	}
	if img, err := png.Decode(bytes.NewReader(small)); err != nil || img.Bounds() != image.Rect(0, 0, 200, 100) {
		t.Errorf("small image changed: %v %v", img.Bounds(), err)
	}
	big, w, h, err := encodeForKitty(encodePNG(t, twoTone(2000, 1000)))
	if err != nil {
		t.Fatal(err)
	}
	if w != kittyMaxPixels || h != kittyMaxPixels/2 {
		t.Errorf("a 2000×1000 image should be shrunk to %d wide, got %dx%d", kittyMaxPixels, w, h)
	}
	if img, err := png.Decode(bytes.NewReader(big)); err != nil || img.Bounds().Dx() != w {
		t.Errorf("the encoded image is not what was reported: %v", err)
	}
	if _, _, _, err := encodeForKitty([]byte("nope")); err == nil {
		t.Error("garbage encoded")
	}
}

func TestGraphicsAreOnlyUsedWhereTheyWork(t *testing.T) {
	env := func(kv ...string) func(string) string {
		m := map[string]string{}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return func(k string) string { return m[k] }
	}
	cases := []struct {
		name string
		env  func(string) string
		want bool
	}{
		{"kitty by TERM", env("TERM", "xterm-kitty"), true},
		{"kitty by its window id", env("TERM", "xterm-256color", "KITTY_WINDOW_ID", "3"), true},
		{"ghostty", env("TERM", "xterm-ghostty"), true},
		{"ghostty by program", env("TERM", "xterm-256color", "TERM_PROGRAM", "ghostty"), true},
		{"a plain terminal", env("TERM", "xterm-256color"), false},
		{"inside tmux the images would not get through", env("TERM", "xterm-kitty", "TMUX", "/tmp/tmux-1/default,1,0"), false},
		{"inside screen", env("TERM", "xterm-kitty", "STY", "123.pts"), false},
		{"forced on", env("TERM", "xterm-256color", "MM_COVERS", "kitty"), true},
		{"forced off", env("TERM", "xterm-kitty", "MM_COVERS", "blocks"), false},
		{"forced on even in tmux", env("TERM", "screen", "TMUX", "x", "MM_COVERS", "kitty"), true},
		{"auto means look", env("TERM", "xterm-kitty", "MM_COVERS", "auto"), true},
	}
	for _, c := range cases {
		if got := wantsKittyGraphics(c.env); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
