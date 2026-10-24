package tui

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestQuestionMarkShowsEveryKeyGroupedByWhatItDoes(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "?")
	contains(t, m, "Keys", "Moving", "Editing the fields", "into the fields", "(mixed)", "Several tracks", "Looking up", "Library",
		"space", "ctrl+a", "J K", "[ ]", "ctrl+r", "tab", "enter", "esc")
	contains(t, m, "esc closes")
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
