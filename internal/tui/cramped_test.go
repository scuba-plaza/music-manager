package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/app"
)

// The cover is drawn in whatever room the fields leave. A short terminal
// (a large font does that) must still get one, and names in another script
// must not take it away.

func ptr[T any](v T) *T { return &v }

// inArabic gives Flow a title in Arabic.
func inArabic(t *testing.T, hook *editHook) {
	t.Helper()
	tr := trackNamed(t, hook.Library, "Flow")
	_, err := appOf(t, hook.Library).Edit(tr.ID, app.Edit{Title: ptr("فلو")})
	if err != nil {
		t.Fatal(err)
	}
}

func resized(t *testing.T, m Model, w, h int) Model {
	t.Helper()
	return send(t, m, tea.WindowSizeMsg{Width: w, Height: h})
}

func TestTheCoverIsStillShownInAShortTerminal(t *testing.T) {
	for _, size := range [][2]int{{120, 24}, {100, 24}, {80, 24}} {
		m, _ := coverModel(t)
		m = resized(t, m, size[0], size[1])
		if got := strings.Count(screen(m), "▀"); got < 12 {
			t.Errorf("%dx%d: the cover is %d cells\n%s", size[0], size[1], got, screen(m))
		}
	}
}

func TestClickingAFieldFindsItInAShortTerminal(t *testing.T) {
	m, hook := coverModel(t)
	inArabic(t, hook)
	m = send(t, m, m.load())
	m = resized(t, m, 120, 26)
	contains(t, m, "فلو")

	l := computeLayout(m.w, m.h)
	for _, c := range []struct {
		label string
		field fieldKind
	}{{"Artist", fArtist}, {"Album", fAlbum}, {"Released", fReleased}, {"Cover", fCover}} {
		m.focus = paneBrowser
		m = send(t, m, click(l.left+8, fieldRow(t, m, c.label)))
		if m.focus != paneInspector || trackFields[m.field] != c.field {
			t.Errorf("a click on %s landed on field %d (focus %v)", c.label, m.field, m.focus)
		}
	}
}

func TestTypingInATightFitCutsThePictureInsteadOfDroppingIt(t *testing.T) {
	m, log, _ := pictureModel(t, 120, 26)
	before := placeholderCount(m)
	if before == 0 {
		t.Fatalf("no picture to begin with:\n%s", screen(m))
	}
	m = keys(t, m, "right", "enter") // edit the title
	m = typed(t, m, "Ё")
	if got := placeholderCount(m); got == 0 || got > before {
		t.Errorf("%d placeholders while typing, %d before; the picture should lose its bottom rows and stay\n%s", got, before, screen(m))
	}
	if got := len(transmissions(t, log)); got != 1 {
		t.Errorf("typing sent the picture again: %d pictures sent", got)
	}
}
