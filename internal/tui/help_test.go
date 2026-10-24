package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// The help is one page for each kind of thing the keys do, with the tabs of
// the pages along the top, so that no page is crowded.

func TestQuestionMarkOpensTheHelpOnItsFirstPageWithTheTabsOfTheOthers(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "?")
	contains(t, m, "Keys", "1 Moving", "2 Editing", "3 Marking", "4 Library", "esc closes", "← → pages")
	contains(t, m, "↑↓", "PgUp PgDn", "enter", "open / close")
	lacks(t, m, "ctrl+a", "nudge", "undo")
}

func TestTheArrowsAndTheNumbersTurnTheHelpPages(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "?", "right")
	contains(t, m, "choose a field", "nudge 0.1 s", "(mixed)")
	lacks(t, m, "PgUp PgDn", "ctrl+a")

	m = keys(t, m, "l")
	contains(t, m, "ctrl+a", "number an album")
	lacks(t, m, "nudge", "undo")

	m = keys(t, m, "right")
	contains(t, m, "undo / redo", "MusicBrainz")
	m = keys(t, m, "right") // there is no page after the last
	contains(t, m, "undo / redo")

	m = keys(t, m, "left", "h")
	contains(t, m, "nudge 0.1 s")
	m = keys(t, m, "1")
	contains(t, m, "PgUp PgDn")
	m = keys(t, m, "left") // nor before the first
	contains(t, m, "PgUp PgDn")
	m = keys(t, m, "4")
	contains(t, m, "undo / redo")
}

func TestTabTurnsTheHelpPagesRoundAndBack(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "?", "tab", "tab", "tab")
	contains(t, m, "undo / redo")
	lacks(t, m, "PgUp PgDn")
	m = keys(t, m, "tab")
	contains(t, m, "PgUp PgDn")
	lacks(t, m, "undo / redo")
	m = keys(t, m, "shift+tab")
	contains(t, m, "undo / redo")
	lacks(t, m, "PgUp PgDn")
}

func TestEveryDescriptionOfEveryPageIsShownInFullAtTheSmallestUsualSize(t *testing.T) {
	m, _ := browserModel(t, 80, 24)
	m = keys(t, m, "?")
	for page := range helpPages {
		m = keys(t, m, string(rune('1'+page)))
		for _, s := range helpPages[page].sections {
			contains(t, m, s.name)
			for _, e := range s.entries {
				contains(t, m, e.what)
			}
		}
	}
}

func TestOpeningTheHelpAgainStartsOnTheFirstPage(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "?", "3", "esc", "?")
	contains(t, m, "PgUp PgDn")
	lacks(t, m, "ctrl+a")
}

func TestHelpClosesWithAnyOfTheUsualKeysAndNeverQuits(t *testing.T) {
	for _, k := range []string{"esc", "?", "q", "enter"} {
		m, _ := editingModel(t, "Flow")
		m = keys(t, m, "?", k)
		if m.overlay != nil {
			t.Errorf("%s did not close the help", k)
		}
	}
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "?")
	if _, cmd := m.Update(press("q")); quits(cmd) {
		t.Error("q in the help quit the program")
	}
}

func TestHelpFitsEveryTerminalSizeThatIsBigEnoughToUse(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {100, 30}, {80, 24}, {70, 20}, {60, 16}} {
		m := newTestModel(t, NewLibrary(testApp(t)), size[0], size[1])
		m = keys(t, m, "?")
		ls := lines(m)
		if len(ls) != size[1] {
			t.Errorf("%dx%d: %d lines", size[0], size[1], len(ls))
		}
		for i, l := range ls {
			if ansi.StringWidth(l) > size[0] {
				t.Errorf("%dx%d: line %d is %d wide", size[0], size[1], i, ansi.StringWidth(l))
			}
		}
	}
}

func TestInTheNarrowLayoutAFieldPaneReplacesTheTree(t *testing.T) {
	m, _ := browserModel(t, 65, 26)
	m = selectTrack(t, m, "Flow")
	contains(t, m, "Flow", "Shadows")
	lacks(t, m, "Title")

	m = keys(t, m, "right")
	contains(t, m, "Title", "Artist")
	lacks(t, m, "Library", "Along Our Way", "Cut start")
	m = keys(t, m, "2")
	contains(t, m, "Cut start")
	m = keys(t, m, "1")

	m = keys(t, m, "esc")
	contains(t, m, "Library", "Along Our Way")
	lacks(t, m, "Cut start")
}

func TestAllTheDialogsWorkInTheNarrowLayoutToo(t *testing.T) {
	m, _ := browserModel(t, 62, 18)
	m = selectTrack(t, m, "Flow")
	for _, k := range []string{"d", "e", "?", "s"} {
		m = keys(t, m, k)
		ls := lines(m)
		if len(ls) != 18 {
			t.Errorf("%s: %d lines", k, len(ls))
		}
		for i, l := range ls {
			if ansi.StringWidth(l) > 62 {
				t.Errorf("%s: line %d is %d wide", k, i, ansi.StringWidth(l))
			}
		}
		m = keys(t, m, "esc")
		if m.overlay != nil {
			m.overlay = nil
		}
	}
}

func TestTheHelpDialogStaysWhereItIsWhenThePagesTurn(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "?")
	row := func() int {
		for i, l := range lines(m) {
			if strings.Contains(l, "4 Library") {
				return i
			}
		}
		return -1
	}
	want := row()
	for _, page := range []string{"2", "3", "4", "1"} {
		if m = keys(t, m, page); row() != want {
			t.Errorf("the tabs moved from row %d to %d on page %s", want, row(), page)
		}
	}
}
