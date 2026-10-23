package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/app"
)

// The cover is drawn in whatever room the fields leave. A short terminal
// (a large font does that) must still get one, and names with an ASCII form
// of their own, which add a line each, must not be what takes it away.

func ptr[T any](v T) *T { return &v }

// inArabic gives Flow original names that differ from their ASCII forms.
func inArabic(t *testing.T, hook *editHook) {
	t.Helper()
	tr := trackNamed(t, hook.Library, "Flow")
	_, err := appOf(t, hook.Library).Edit(tr.ID, app.Edit{
		Title: ptr("فلو"), TitleASCII: ptr("Flow"),
		Artist: ptr("شون جيمس"), ArtistASCII: ptr("Shawn James"),
		Album: ptr("ظلال"), AlbumASCII: ptr("Shadows"),
	})
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

func TestTheAsciiFormsGiveWayToTheCoverWhenThereIsNoRoomForBoth(t *testing.T) {
	m, hook := coverModel(t)
	inArabic(t, hook)
	m = send(t, m, m.load())

	roomy := resized(t, m, 120, 40)
	contains(t, roomy, "ascii  Flow", "ascii  Shawn James", "ascii  Shadows", "▀")

	cramped := resized(t, m, 120, 26)
	if got := strings.Count(screen(cramped), "▀"); got < 12 {
		t.Errorf("the cover is only %d cells in 26 lines:\n%s", got, screen(cramped))
	}
	lacks(t, cramped, "ascii  Flow")
	contains(t, cramped, "فلو") // the names themselves stay
}

func TestAFullResolutionCoverAlsoSurvivesTheAsciiForms(t *testing.T) {
	m, _, hook := pictureModel(t, 120, 26)
	inArabic(t, hook)
	m = send(t, m, m.load())
	if placeholderCount(m) == 0 {
		t.Errorf("no picture in 26 lines:\n%s", screen(m))
	}
}

func TestClickingAFieldStillFindsItWhenTheAsciiFormsAreHidden(t *testing.T) {
	m, hook := coverModel(t)
	inArabic(t, hook)
	m = send(t, m, m.load())
	m = resized(t, m, 120, 26)
	lacks(t, m, "ascii  Flow")

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
	m = keys(t, m, "right", "enter") // edit the title: a line for its ASCII form opens under it
	m = typed(t, m, "Ё")
	if got := placeholderCount(m); got == 0 || got > before {
		t.Errorf("%d placeholders while typing, %d before; the picture should lose its bottom rows and stay\n%s", got, before, screen(m))
	}
	if got := len(transmissions(t, log)); got != 1 {
		t.Errorf("typing sent the picture again: %d pictures sent", got)
	}
}

// coverRowsShown counts the lines of the screen that have a piece of the
// cover (drawn in blocks) in them.
func coverRowsShown(m Model) int {
	n := 0
	for _, l := range lines(m) {
		if strings.Contains(l, "▀") {
			n++
		}
	}
	return n
}

func TestTheAsciiFormsAreOnlyShownWhileTheCoverKeepsAGoodSize(t *testing.T) {
	for h := 24; h <= 44; h++ {
		m, hook := coverModel(t)
		inArabic(t, hook)
		m = resized(t, send(t, m, m.load()), 120, h)
		rows, ascii := coverRowsShown(m), strings.Contains(screen(m), "ascii  Flow")
		if rows < minCoverRows {
			t.Errorf("%d lines: no cover", h)
		}
		if ascii && rows < comfortCoverRows {
			t.Errorf("%d lines: the ASCII forms are shown and the cover is down to %d lines; it should win", h, rows)
		}
		if !ascii && h >= 40 {
			t.Errorf("%d lines: there is room for the ASCII forms, but they are gone", h)
		}
	}
}
