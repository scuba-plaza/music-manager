package tui

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"music-manager/internal/manifest"
)

var update = flag.Bool("update", false, "rewrite the golden screens in testdata")

// atMusic is a library that says its folder is somewhere fixed, and that
// every track was added and last edited on fixed days, so the screen reads
// the same on every machine and every day.
type atMusic struct{ Library }

func (atMusic) Root() string { return "/home/me/Music/mp3-player" }

func (a atMusic) Tracks() ([]manifest.Track, error) {
	ts, err := a.Library.Tracks()
	for i := range ts {
		ts[i].AddedAt = time.Date(2024, 3, 5, 12, 0, 0, 0, time.UTC)
		ts[i].UpdatedAt = time.Date(2024, 3, 9, 12, 0, 0, 0, time.UTC)
	}
	return ts, err
}

// goldenScreen is the browser with an album open and a track selected,
// drawn at the given size.
func goldenScreen(t *testing.T, w, h int) string {
	t.Helper()
	t.Setenv("HOME", "/home/me")
	m, lib := browserModel(t, w, h)
	m.lib = atMusic{lib}
	m = send(t, m, m.load())
	m = selectTrack(t, m, "Flow")
	return ansi.Strip(m.View())
}

// TestTheScreenLooksAsDesigned compares what is drawn at five sizes with the
// screens kept in testdata. A difference is either a bug or a design change;
// after checking by eye, run `go test ./internal/tui -update` to accept it.
func TestTheScreenLooksAsDesigned(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {100, 30}, {80, 24}, {60, 20}, {40, 10}} {
		w, h := size[0], size[1]
		t.Run(fmt.Sprintf("%dx%d", w, h), func(t *testing.T) {
			got := goldenScreen(t, w, h)
			file := filepath.Join("testdata", fmt.Sprintf("browser-%dx%d.txt", w, h))
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte(got+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("%v (run with -update to create it)", err)
			}
			if strings.TrimSuffix(string(want), "\n") != got {
				t.Errorf("the screen at %dx%d changed.\n--- want\n%s\n--- got\n%s", w, h, want, got)
			}
		})
	}
}

func TestTheFooterDropsHintsRatherThanCuttingOneInHalf(t *testing.T) {
	for _, w := range []int{120, 80, 62, 60} {
		m, _ := browserModel(t, w, 24)
		footer := lines(m)[23]
		if strings.Contains(footer, "…") {
			t.Errorf("%d wide: the footer was cut: %q", w, footer)
		}
		for _, always := range []string{"? help", "q quit"} {
			if !strings.Contains(footer, always) {
				t.Errorf("%d wide: the footer lost %q: %q", w, always, footer)
			}
		}
	}
}

func TestTheFilterBoxOnlySuggestsAnExampleWhenItFits(t *testing.T) {
	for _, w := range []int{120, 100, 80, 70} {
		m, _ := browserModel(t, w, 24)
		box := lines(m)[2]
		if strings.Contains(box, "(e.g.") && !strings.Contains(box, "status:unidentified)") {
			t.Errorf("%d wide: the example is cut off: %q", w, box)
		}
		if !strings.Contains(box, "filter") {
			t.Errorf("%d wide: the filter box does not say what it is: %q", w, box)
		}
	}
}
