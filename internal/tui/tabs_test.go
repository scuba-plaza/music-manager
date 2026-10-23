package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// The inspector shows a track's fields in tabs, so that each has room.

func TestTheInspectorOpensOnTheDetailsAndOffersTheAudioTab(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "right")
	contains(t, m, "Details", "Audio", "Title", "Artist", "Album", "Track", "Released", "Cover")
	lacks(t, m, "Cut start", "Cut end", "Kept")
}

func TestRightAndLeftMoveBetweenTheTabsAndLeftOnTheFirstLeaves(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "right", "right") // into the fields, then to the next tab
	contains(t, m, "Cut start", "Cut end", "Kept")
	lacks(t, m, "Released")
	if m.focus != paneInspector {
		t.Fatal("moving to another tab left the inspector")
	}

	m = keys(t, m, "right") // there is no tab after the last
	contains(t, m, "Cut start")

	m = keys(t, m, "left")
	contains(t, m, "Released")
	lacks(t, m, "Cut start")
	if m.focus != paneInspector {
		t.Fatal("left from the second tab should stay in the inspector")
	}
	m = keys(t, m, "left")
	if m.focus != paneBrowser {
		t.Error("left from the first tab should go back to the tree")
	}
}

func TestNumbersJumpToATab(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "right", "2")
	contains(t, m, "Cut start")
	lacks(t, m, "Released")
	m = keys(t, m, "1")
	contains(t, m, "Released")
	lacks(t, m, "Cut start")
	m = keys(t, m, "9") // no such tab
	contains(t, m, "Released")
}

func TestMovingDownPastTheLastFieldOfATabContinuesOnTheNext(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = focusField(t, m, fCover)
	lacks(t, m, "Cut start")
	m = keys(t, m, "down")
	if trackFields[m.field] != fCutStart {
		t.Fatalf("on %v, want Cut start", trackFields[m.field])
	}
	contains(t, m, "Cut start", "Kept")
	m = keys(t, m, "up")
	if trackFields[m.field] != fCover {
		t.Errorf("up came to %v, want Cover", trackFields[m.field])
	}
	contains(t, m, "Released")
}

func TestTheTabStaysWhenAnotherTrackIsSelected(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "right", "2", "esc")
	contains(t, m, "Cut start")
	m = selectTrack(t, m, "Along Our Way")
	contains(t, m, "Cut start", "Kept")
	lacks(t, m, "Released")
}

func TestEveryTabKeepsThePathAndTheDatesInView(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "right")
	contains(t, m, "07 - Flow.mp3", "plays")
	m = keys(t, m, "2")
	contains(t, m, "07 - Flow.mp3", "plays")
}

func TestOnlyTheDetailsTabDrawsTheCover(t *testing.T) {
	m, _ := coverModel(t)
	contains(t, m, "▀")
	m = keys(t, m, "right", "2")
	lacks(t, m, "▀")
	m = keys(t, m, "1")
	contains(t, m, "▀")
}

func TestClickingATabLabelChoosesIt(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	l := computeLayout(m.w, m.h)
	strip := strings.Split(ansi.Strip(m.View()), "\n")[firstContentRow]
	x := strings.Index(string([]rune(strip)[l.left:]), "Audio")
	if x < 0 {
		t.Fatalf("no Audio label on the strip %q", strip)
	}
	m = send(t, m, click(l.left+x+1, firstContentRow))
	contains(t, m, "Cut start", "Kept")
	if m.focus != paneInspector {
		t.Error("clicking a tab should go to the fields")
	}
	strip = strings.Split(ansi.Strip(m.View()), "\n")[firstContentRow]
	x = strings.Index(string([]rune(strip)[l.left:]), "Details")
	m = send(t, m, click(l.left+x+1, firstContentRow))
	contains(t, m, "Released")
	lacks(t, m, "Cut start")
}

func TestTheFooterOffersTheTabsInTheInspector(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	lacks(t, m, "1-2 tabs")
	m = keys(t, m, "right")
	contains(t, m, "1-2 tabs")
}
