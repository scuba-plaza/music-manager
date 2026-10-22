package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

// ---- the library check

func TestDChecksTheLibraryAndSaysWhenAllIsWell(t *testing.T) {
	a := testApp(t)
	seed(t, a, seedTrack{"Anchor", "Band", "Album", 1, 1, 0})
	m := newTestModel(t, NewLibrary(a), 120, 36)
	m = keys(t, m, "D")
	contains(t, m, "Library check", "No problems found")
	m = keys(t, m, "esc")
	if m.overlay != nil {
		t.Error("esc should close it")
	}
}

func TestTheCheckListsDamageAndLooseEndsWorstFirstAndEnterGoesToTheTrack(t *testing.T) {
	m, lib := browserModel(t, 120, 36) // five identified tracks and one that needs work
	a := appOf(t, lib)
	flow := trackNamed(t, lib, "Flow")
	if err := os.Remove(a.AudioPath(flow.AudioSHA256)); err != nil {
		t.Fatal(err)
	}

	m = keys(t, m, "D")
	contains(t, m, "audio file is missing", "not identified yet", "Flow")
	d, ok := m.overlay.(*doctorDialog)
	if !ok || len(d.problems) < 2 {
		t.Fatalf("problems = %+v", d)
	}
	if d.problems[0].Severity != app.Error {
		t.Errorf("the worst problem should come first: %+v", d.problems[0])
	}

	m = keys(t, m, "enter")
	if m.overlay != nil {
		t.Fatal("enter should close the check")
	}
	if r, _ := m.selectedRow(); r.Kind != kindTrack || r.Track.ID != flow.ID {
		t.Errorf("the track with the problem should be selected, got %+v", r)
	}
}

// ---- the mouse

func click(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
}

func wheel(down bool) tea.MouseMsg {
	b := tea.MouseButtonWheelUp
	if down {
		b = tea.MouseButtonWheelDown
	}
	return tea.MouseMsg{X: 5, Y: 6, Button: b, Action: tea.MouseActionPress}
}

// rowY is the screen row of tree row i when nothing is scrolled: the header,
// the border and the filter line come first.
func rowY(i int) int { return 3 + i }

func TestClickingARowSelectsIt(t *testing.T) {
	m, _ := browserModel(t, 120, 36) // Needs work, Fairuz, Phoria, Shawn James
	m = send(t, m, click(10, rowY(2)))
	if got := selectedLabel(t, m); got != "Phoria" {
		t.Errorf("selected %q", got)
	}
	m = send(t, m, click(10, rowY(40))) // below the last row
	if got := selectedLabel(t, m); got != "Phoria" {
		t.Errorf("a click on empty space must not move the selection, got %q", got)
	}
}

func TestClickingTheSelectedTrackOpensItsFields(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = selectTrack(t, m, "Flow")
	row := m.cursor
	m = send(t, m, click(10, rowY(row-m.offset)))
	if m.focus != paneInspector {
		t.Error("clicking the selected track should go to its fields")
	}
}

func TestDoubleClickOpensAndClosesAnArtist(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	now := time.Now()
	m.now = func() time.Time { return now }
	m = send(t, m, click(10, rowY(3))) // Shawn James
	now = now.Add(200 * time.Millisecond)
	m = send(t, m, click(10, rowY(3)))
	contains(t, m, "Shadows")
	now = now.Add(2 * time.Second) // too slow to be a double click
	m = send(t, m, click(10, rowY(3)))
	contains(t, m, "Shadows")
}

func TestTheWheelScrollsTheTree(t *testing.T) {
	m, names := manyArtists(t, 60)
	m = send(t, m, wheel(true), wheel(true))
	if got := selectedLabel(t, m); got == names[0] {
		t.Error("the wheel did not move the selection down")
	}
	m = send(t, m, wheel(false), wheel(false))
	if got := selectedLabel(t, m); got != names[0] {
		t.Errorf("wheel up should come back, on %q", got)
	}
}

// fieldRow finds, on the screen as drawn, the row where the inspector shows
// the field with the given label.
func fieldRow(t *testing.T, m Model, label string) int {
	t.Helper()
	l := computeLayout(m.w, m.h)
	for y, line := range strings.Split(ansi.Strip(m.View()), "\n") {
		cells := []rune(line)
		if len(cells) <= l.left+2 {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(string(cells[l.left+1:])), label+" ") ||
			strings.HasPrefix(strings.TrimLeft(string(cells[l.left+1:]), " ▌"), label+" ") {
			return y
		}
	}
	t.Fatalf("no %q field on the screen:\n%s", label, ansi.Strip(m.View()))
	return 0
}

func TestClickingAFieldInTheInspectorChoosesItAndAgainEditsIt(t *testing.T) {
	for _, track := range []string{"Flow", "Wtni"} { // on an album, and a single
		m, _ := browserModel(t, 120, 36)
		m = selectTrack(t, m, track)
		m = keys(t, m, "right") // into the fields
		l := computeLayout(m.w, m.h)

		for i := len(trackFields) - 1; i >= 0; i-- { // backwards, so no click lands on the field already chosen
			f := trackFields[i]
			y := fieldRow(t, m, f.label())
			m = send(t, m, click(l.left+8, y))
			if trackFields[m.field] != f {
				t.Fatalf("%s: a click on the %s row chose %v", track, f.label(), trackFields[m.field])
			}
			if m.editing {
				t.Errorf("%s: the first click only chooses the field", track)
			}
		}
		y := fieldRow(t, m, "Album")
		m = send(t, m, click(l.left+8, y))
		m = send(t, m, click(l.left+8, y))
		if !m.editing {
			t.Errorf("%s: clicking the chosen field should edit it", track)
		}
	}
}

func TestTheMouseDoesNothingWhileADialogIsOpen(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "?")
	before := m.cursor
	m = send(t, m, click(10, rowY(2)), wheel(true))
	if m.cursor != before || m.overlay == nil {
		t.Error("the mouse reached past the dialog")
	}
}

// ---- changes made elsewhere

func TestChangesMadeByAnotherMMAppearOnTheNextLook(t *testing.T) {
	m, lib := browserModel(t, 120, 36)
	m = selectTrack(t, m, "Flow")
	contains(t, m, "Flow")

	// Another process edits the library behind the interface's back.
	other, err := app.Open(appOf(t, lib).Root())
	if err != nil {
		t.Fatal(err)
	}
	flow := trackNamed(t, lib, "Flow")
	renamed := "Flow (remastered)"
	time.Sleep(10 * time.Millisecond) // the file's modification time moves on
	if _, err := other.Edit(flow.ID, app.Edit{Title: &renamed}); err != nil {
		t.Fatal(err)
	}

	m = send(t, m, pollMsg{})
	contains(t, m, "Flow (remastered)")
	if r, _ := m.selectedRow(); r.Track.ID != flow.ID {
		t.Errorf("the selection should stay on the track, got %+v", r)
	}
}

func TestNothingChangedMeansNothingHappens(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = selectTrack(t, m, "Flow")
	before := screen(m)
	m = send(t, m, pollMsg{})
	if after := screen(m); after != before {
		t.Errorf("a look with nothing new changed the screen:\n%s\n-- vs --\n%s", before, after)
	}
}

func TestClickingAFieldOfATrackNotIdentifiedYetFindsTheRightRow(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "right", "down", "right") // open Needs work, its track, its fields
	if r, _ := m.selectedRow(); r.Kind != kindTrack || r.Track.Status == manifest.Identified {
		t.Fatalf("want the unidentified track, got %+v", r)
	}
	l := computeLayout(m.w, m.h)
	m = send(t, m, click(l.left+8, fieldRow(t, m, "Cut end")))
	if trackFields[m.field] != fCutEnd {
		t.Errorf("a click on the Cut end row chose %v", trackFields[m.field])
	}
}
