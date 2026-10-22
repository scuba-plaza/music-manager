package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

func smallLibrary(t *testing.T) (Library, *app.App) {
	a := testApp(t)
	seed(t, a,
		seedTrack{"The Wanderer", "Shawn James", "Shadows", 1, 12, 0},
		seedTrack{"Along Our Way", "Shawn James", "Shadows", 12, 12, 0},
		seedTrack{"Current", "Phoria", "", 0, 0, 0},
	)
	return NewLibrary(a), a
}

func TestTheScreenIsFilledExactly(t *testing.T) {
	lib, _ := smallLibrary(t)
	for _, size := range [][2]int{{160, 50}, {120, 36}, {100, 30}, {80, 24}, {70, 20}, {65, 20}, {60, 16}, {50, 10}, {20, 5}} {
		w, h := size[0], size[1]
		m := newTestModel(t, lib, w, h)
		ls := lines(m)
		if len(ls) != h {
			t.Errorf("%dx%d: %d lines, want %d", w, h, len(ls), h)
		}
		for i, l := range ls {
			if got := ansi.StringWidth(l); got > w {
				t.Errorf("%dx%d: line %d is %d wide: %q", w, h, i, got, l)
			}
		}
	}
}

func TestNothingIsDrawnBeforeTheSizeIsKnown(t *testing.T) {
	lib, _ := smallLibrary(t)
	if v := New(lib).View(); v != "" {
		t.Errorf("View() = %q before any size", v)
	}
}

func TestHeaderNamesTheLibraryAndCountsTracks(t *testing.T) {
	t.Setenv("HOME", "/home/me")
	lib, _ := smallLibrary(t)
	m := newTestModel(t, atMusic{lib}, 120, 36) // a folder with a short name: a long one is cut off
	first := lines(m)[0]
	for _, want := range []string{"mm", "~/Music/mp3-player", "3 tracks"} {
		if !strings.Contains(first, want) {
			t.Errorf("header %q lacks %q", first, want)
		}
	}
}

func TestTwoPanesWithTitlesAndAFooter(t *testing.T) {
	lib, _ := smallLibrary(t)
	m := newTestModel(t, lib, 120, 36)
	contains(t, m, "Library", "q quit", "╭", "╰")
	ls := lines(m)
	if last := ls[len(ls)-1]; !strings.Contains(last, "q quit") {
		t.Errorf("the footer should be the last line, got %q", last)
	}
}

func TestOnePaneWhenNarrow(t *testing.T) {
	lib, _ := smallLibrary(t)
	m := newTestModel(t, lib, 65, 24)
	if n := countPanes(lines(m)[1]); n != 1 {
		t.Errorf("%d panes at 65 columns, want 1:\n%s", n, screen(m))
	}
	m = newTestModel(t, lib, 120, 36)
	if n := countPanes(lines(m)[1]); n != 2 {
		t.Errorf("%d panes at 120 columns, want 2", n)
	}
}

// countPanes counts the top-left corners on a pane's first line.
func countPanes(line string) int {
	n := 0
	for _, r := range line {
		if r == '╭' {
			n++
		}
	}
	return n
}

func TestATerminalThatIsTooSmallSaysSo(t *testing.T) {
	lib, _ := smallLibrary(t)
	m := newTestModel(t, lib, 50, 10)
	contains(t, m, "too small", "60", "16")
	lacks(t, m, "╭")
	if cmd := func() tea.Cmd { _, c := m.Update(press("q")); return c }(); !quits(cmd) {
		t.Error("q must still quit a tiny terminal")
	}
}

func TestEmptyLibraryExplainsWhatToDo(t *testing.T) {
	m := newTestModel(t, NewLibrary(testApp(t)), 120, 36)
	contains(t, m, "No tracks yet", "0 tracks")
}

func TestQuitKeys(t *testing.T) {
	lib, _ := smallLibrary(t)
	m := newTestModel(t, lib, 120, 36)
	for _, k := range []string{"q", "ctrl+c"} {
		if _, cmd := m.Update(press(k)); !quits(cmd) {
			t.Errorf("%s did not quit", k)
		}
	}
	if _, cmd := m.Update(press("x")); quits(cmd) {
		t.Error("x quit")
	}
}

type brokenLibrary struct{ Library }

func (brokenLibrary) Tracks() ([]manifest.Track, error) { return nil, errors.New("disk on fire") }

func TestALibraryThatCannotBeReadShowsTheError(t *testing.T) {
	lib, _ := smallLibrary(t)
	m := newTestModel(t, brokenLibrary{lib}, 120, 36)
	contains(t, m, "disk on fire")
}
