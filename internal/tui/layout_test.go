package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestComputeLayout(t *testing.T) {
	cases := []struct {
		w, h int
		mode layoutMode
	}{
		{160, 50, modeTwoPane},
		{120, 36, modeTwoPane},
		{100, 30, modeTwoPane},
		{99, 30, modeTwoPane},
		{70, 24, modeTwoPane},
		{69, 24, modeOnePane},
		{60, 16, modeOnePane},
		{59, 30, modeTooSmall},
		{120, 15, modeTooSmall},
		{0, 0, modeTooSmall},
	}
	for _, c := range cases {
		l := computeLayout(c.w, c.h)
		if l.mode != c.mode {
			t.Errorf("%dx%d: mode %v, want %v", c.w, c.h, l.mode, c.mode)
			continue
		}
		switch l.mode {
		case modeTwoPane:
			if l.left+l.right != c.w || l.left < 28 || l.right < 36 {
				t.Errorf("%dx%d: panes %d + %d", c.w, c.h, l.left, l.right)
			}
		case modeOnePane:
			if l.left != c.w || l.right != c.w {
				t.Errorf("%dx%d: pane widths %d, %d, want the full width", c.w, c.h, l.left, l.right)
			}
		}
		if l.mode != modeTooSmall && l.body != c.h-2 {
			t.Errorf("%dx%d: body %d, want %d (header and footer take two lines)", c.w, c.h, l.body, c.h-2)
		}
	}
}

func TestBoxIsExactlyTheRequestedSize(t *testing.T) {
	content := []string{"one", "a line that is much too long to fit in the box that holds it", "", "wide: 日本語日本語日本語日本語日本語日本語"}
	for _, size := range [][2]int{{30, 8}, {12, 4}, {80, 3}, {20, 12}} {
		w, h := size[0], size[1]
		lines := strings.Split(box("Title", content, w, h, true), "\n")
		if len(lines) != h {
			t.Errorf("%dx%d: %d lines", w, h, len(lines))
		}
		for i, l := range lines {
			if got := ansi.StringWidth(l); got != w {
				t.Errorf("%dx%d: line %d is %d wide: %q", w, h, i, got, l)
			}
		}
		if !strings.Contains(ansi.Strip(lines[0]), "Title") && w >= 12 {
			t.Errorf("%dx%d: title missing from %q", w, h, lines[0])
		}
	}
}

func TestBoxDrawsRoundedBorders(t *testing.T) {
	lines := strings.Split(ansi.Strip(box("T", []string{"x"}, 10, 4, false)), "\n")
	if !strings.HasPrefix(lines[0], "╭") || !strings.HasSuffix(lines[0], "╮") || !strings.HasPrefix(lines[3], "╰") || !strings.HasSuffix(lines[3], "╯") {
		t.Errorf("border = %q", lines)
	}
	if !strings.HasPrefix(lines[1], "│") || !strings.HasSuffix(lines[1], "│") {
		t.Errorf("sides = %q", lines[1])
	}
}

func TestFit(t *testing.T) {
	cases := []struct {
		in   string
		w    int
		want string
	}{
		{"short", 8, "short   "},
		{"exactly8", 8, "exactly8"},
		{"much too long", 8, "much to…"},
		{"日本語です", 7, "日本語…"},
		{"anything", 0, ""},
		{"x", 1, "x"},
		{"xy", 1, "…"},
	}
	for _, c := range cases {
		if got := fit(c.in, c.w); got != c.want {
			t.Errorf("fit(%q, %d) = %q, want %q", c.in, c.w, got, c.want)
		}
	}
}

func TestSpread(t *testing.T) {
	if got := spread("left", "right", 20); got != "left           right" {
		t.Errorf("= %q", got)
	}
	if got := spread("a long left side", "right side", 12); ansi.StringWidth(got) != 12 || !strings.HasSuffix(got, "right side") {
		t.Errorf("= %q: the right side must survive, the left shrinks", got)
	}
}

func TestTilde(t *testing.T) {
	t.Setenv("HOME", "/home/tau")
	for in, want := range map[string]string{
		"/home/tau/Music/mp3-player": "~/Music/mp3-player",
		"/home/tau":                  "~",
		"/home/taurus/x":             "/home/taurus/x",
		"/srv/music":                 "/srv/music",
	} {
		if got := tilde(in); got != want {
			t.Errorf("tilde(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestScrollbar(t *testing.T) {
	if got := scrollbar(10, 10, 0, 5); got != nil {
		t.Errorf("a list that fits needs no scrollbar, got %v", got)
	}
	thumbs := func(bar []string) (first, count int) {
		first = -1
		for i, g := range bar {
			if g == "┃" {
				if first < 0 {
					first = i
				}
				count++
			}
		}
		return
	}
	top := scrollbar(100, 10, 0, 10)
	if len(top) != 10 {
		t.Fatalf("%d cells", len(top))
	}
	if first, n := thumbs(top); first != 0 || n != 1 {
		t.Errorf("at the top the thumb is at %d, %d long", first, n)
	}
	bottom := scrollbar(100, 10, 90, 10)
	if first, n := thumbs(bottom); first != 9 || n != 1 {
		t.Errorf("at the bottom the thumb is at %d, %d long", first, n)
	}
	middle := scrollbar(100, 10, 45, 10)
	if first, _ := thumbs(middle); first < 3 || first > 6 {
		t.Errorf("in the middle the thumb is at %d", first)
	}
	if _, n := thumbs(scrollbar(30, 20, 5, 20)); n < 10 {
		t.Errorf("a thumb for two thirds of the list should be long, got %d of 20", n)
	}
}

func TestBoxWithBarReplacesTheRightBorderBesideTheThumb(t *testing.T) {
	lines := strings.Split(ansi.Strip(boxWithBar("T", []string{"a", "b", "c"}, []string{"", "┃", ""}, 10, 5, true)), "\n")
	if !strings.HasSuffix(lines[2], "┃") {
		t.Errorf("thumb row = %q", lines[2])
	}
	if !strings.HasSuffix(lines[1], "│") || !strings.HasSuffix(lines[3], "│") {
		t.Errorf("other rows should keep the border: %q / %q", lines[1], lines[3])
	}
	for i, l := range lines {
		if ansi.StringWidth(l) != 10 {
			t.Errorf("line %d is %d wide", i, ansi.StringWidth(l))
		}
	}
}
