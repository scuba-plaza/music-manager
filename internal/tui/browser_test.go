package tui

import (
	"fmt"
	"strings"
	"testing"
)

func browserModel(t *testing.T, w, h int) (Model, Library) {
	t.Helper()
	a := testApp(t)
	seed(t, a,
		seedTrack{"The Wanderer", "Shawn James", "Shadows", 1, 12, 0},
		seedTrack{"Flow", "Shawn James", "Shadows", 7, 12, 0},
		seedTrack{"Along Our Way", "Shawn James", "Shadows", 12, 12, 0},
		seedTrack{"Current", "Phoria", "Caught a Black Rabbit", 4, 10, 0},
		seedTrack{"Wtni", "Fairuz", "", 0, 0, 0},
		seedTrack{"", "", "", 0, 0, 0}, // not identified
	)
	lib := NewLibrary(a)
	return newTestModel(t, lib, w, h), lib
}

func selectedLabel(t *testing.T, m Model) string {
	t.Helper()
	r, ok := m.selectedRow()
	if !ok {
		t.Fatal("nothing is selected")
	}
	return r.Label
}

func TestTheTreeStartsCollapsedWithTheFirstRowSelected(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	contains(t, m, "Needs work", "Fairuz", "Phoria", "Shawn James")
	lacks(t, m, "The Wanderer", "Shadows")
	if got := selectedLabel(t, m); got != "Needs work" {
		t.Errorf("selected %q, want the first row", got)
	}
}

func TestUpAndDownMoveAndStopAtTheEnds(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "up")
	if got := selectedLabel(t, m); got != "Needs work" {
		t.Errorf("up at the top moved to %q", got)
	}
	m = keys(t, m, "down", "down")
	if got := selectedLabel(t, m); got != "Phoria" {
		t.Errorf("after down, down: %q, want Phoria", got)
	}
	m = keys(t, m, "j", "j", "j", "j")
	if got := selectedLabel(t, m); got != "Shawn James" {
		t.Errorf("down at the bottom ended on %q", got)
	}
	m = keys(t, m, "k")
	if got := selectedLabel(t, m); got != "Phoria" {
		t.Errorf("k: %q", got)
	}
}

func TestRightOpensLeftClosesAndThenGoesToTheParent(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "down", "down", "down") // Shawn James
	m = keys(t, m, "right")
	contains(t, m, "Shadows")
	lacks(t, m, "Flow")
	m = keys(t, m, "right") // already open: step into the first child
	if got := selectedLabel(t, m); got != "Shadows" {
		t.Fatalf("selected %q, want the album", got)
	}
	m = keys(t, m, "right")
	contains(t, m, "The Wanderer", "Flow", "Along Our Way")
	m = keys(t, m, "right")
	if got := selectedLabel(t, m); got != "The Wanderer" {
		t.Fatalf("selected %q, want the first track", got)
	}

	m = keys(t, m, "left") // a track has nothing to close: go to its album
	if got := selectedLabel(t, m); got != "Shadows" {
		t.Errorf("left from a track: %q, want its album", got)
	}
	m = keys(t, m, "left") // close the album
	lacks(t, m, "The Wanderer")
	m = keys(t, m, "left") // album closed: go to the artist
	if got := selectedLabel(t, m); got != "Shawn James" {
		t.Errorf("left from a closed album: %q", got)
	}
	m = keys(t, m, "left") // close the artist
	lacks(t, m, "Shadows")
}

func TestEnterOpensAndClosesGroups(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "enter") // Needs work
	if rows := m.rows; len(rows) < 2 || rows[1].Kind != kindTrack {
		t.Fatalf("Needs work did not open: %v", labels(m.rows))
	}
	m = keys(t, m, "enter")
	if len(m.rows) != 4 {
		t.Errorf("Needs work did not close: %v", labels(m.rows))
	}
}

func manyArtists(t *testing.T, n int) (Model, []string) {
	t.Helper()
	a := testApp(t)
	var specs []seedTrack
	var names []string
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("Artist %02d", i)
		names = append(names, name)
		specs = append(specs, seedTrack{fmt.Sprintf("Song %02d", i), name, "", 0, 0, 0})
	}
	seed(t, a, specs...)
	return newTestModel(t, NewLibrary(a), 100, 24), names
}

func TestTheSelectionAlwaysStaysOnScreenAsYouScroll(t *testing.T) {
	m, names := manyArtists(t, 60)
	for i := 0; i < 45; i++ {
		m = keys(t, m, "down")
		if got := selectedLabel(t, m); got != names[i+1-0] && i+1 < len(names) {
			// rows == artists here, so row i+1 is Artist i+1
			t.Fatalf("step %d selected %q", i, got)
		}
		contains(t, m, names[i+1])
	}
	if lacks1 := screen(m); strings.Contains(lacks1, "Artist 00") {
		t.Error("the top of the list should have scrolled away")
	}
	for _, g := range []string{"┃"} {
		contains(t, m, g)
	}
}

func TestPageKeysAndHomeAndEnd(t *testing.T) {
	m, names := manyArtists(t, 60)
	m = keys(t, m, "pgdown")
	first := selectedLabel(t, m)
	if first == names[0] || first == names[1] {
		t.Errorf("pgdown stayed near the top: %q", first)
	}
	m = keys(t, m, "G")
	if got := selectedLabel(t, m); got != names[59] {
		t.Errorf("G: %q, want the last", got)
	}
	contains(t, m, names[59])
	m = keys(t, m, "g")
	if got := selectedLabel(t, m); got != names[0] {
		t.Errorf("g: %q, want the first", got)
	}
	contains(t, m, names[0])
	m = keys(t, m, "G", "pgup")
	if got := selectedLabel(t, m); got == names[59] || got == names[58] {
		t.Errorf("pgup stayed at the bottom: %q", got)
	}
}

func TestNoScrollbarWhenEverythingFits(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	lacks(t, m, "┃")
}

func TestSlashFiltersLiveAndEscRestores(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "/")
	contains(t, m, "enter keep", "esc clear")
	m = typed(t, m, "flow")
	contains(t, m, "Shawn James", "Shadows", "Flow")
	lacks(t, m, "Phoria", "Fairuz", "The Wanderer")
	if got := selectedLabel(t, m); got != "Shawn James" {
		t.Errorf("the first match should be selected, got %q", got)
	}

	m = keys(t, m, "esc")
	contains(t, m, "Phoria", "Fairuz")
	lacks(t, m, "Flow")
	if m.filter != "" || m.filtering {
		t.Errorf("filter %q, filtering %v after esc", m.filter, m.filtering)
	}
}

func TestEnterKeepsTheFilterAndLetsYouMoveAround(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "/")
	m = typed(t, m, "shawn")
	m = keys(t, m, "enter")
	if m.filtering || m.filter != "shawn" {
		t.Fatalf("filtering %v, filter %q", m.filtering, m.filter)
	}
	lacks(t, m, "Phoria")
	m = keys(t, m, "down")
	if got := selectedLabel(t, m); got != "Shadows" {
		t.Errorf("down in a kept filter: %q", got)
	}
	m = keys(t, m, "esc") // esc outside the input clears a kept filter
	contains(t, m, "Phoria")
}

func TestFilterWithStatusAndNoMatches(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "/")
	m = typed(t, m, "status:unidentified")
	contains(t, m, "Needs work")
	lacks(t, m, "Shawn")

	m = keys(t, m, "esc", "/")
	m = typed(t, m, "zzzz")
	contains(t, m, "No tracks match")
	if _, ok := m.selectedRow(); ok {
		t.Error("nothing should be selected when nothing matches")
	}
}

func TestTypingInTheFilterIsNotTakenForCommands(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "/")
	m = typed(t, m, "qjk")
	if m.filter != "qjk" {
		t.Errorf("filter = %q: q, j and k must be typed, not obeyed", m.filter)
	}
}

func TestTheSelectionSurvivesAReload(t *testing.T) {
	m, lib := browserModel(t, 120, 36)
	m = keys(t, m, "down", "down", "down", "right", "right", "right", "right") // The Wanderer
	if got := selectedLabel(t, m); got != "The Wanderer" {
		t.Fatalf("setup: %q", got)
	}
	m = send(t, m, m.load())
	if got := selectedLabel(t, m); got != "The Wanderer" {
		t.Errorf("after a reload the selection moved to %q", got)
	}
	_ = lib
}

func TestHeaderCountsTracksAndWhatNeedsWork(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	if first := lines(m)[0]; !strings.Contains(first, "6 tracks") || !strings.Contains(first, "1 needs work") {
		t.Errorf("header = %q", first)
	}
}

func TestTheInspectorShowsTheSelectedTrack(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "down", "down", "down", "right", "right", "right", "right") // The Wanderer
	if got := selectedLabel(t, m); got != "The Wanderer" {
		t.Fatalf("setup: %q", got)
	}
	contains(t, m, "Title", "The Wanderer", "Artist", "Shawn James", "Album", "Shadows", "1 of 12", "Shawn James/Shadows/01 - The Wanderer.mp3")
}

func TestTheInspectorSummarisesArtistsAlbumsAndGroups(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	contains(t, m, "not been identified")  // Needs work is selected
	m = keys(t, m, "down", "down", "down") // Shawn James
	contains(t, m, "Shawn James", "3 tracks", "1 album")
	m = keys(t, m, "right", "right")
	contains(t, m, "Shadows", "3 tracks")
}

// treeLine is the line of the tree pane, counted from its first row, on
// which the label appears, or -1.
func treeLine(m Model, label string) int {
	left := computeLayout(m.w, m.h).left
	for i, l := range lines(m)[3:] {
		if cells := []rune(l); len(cells) > left && strings.Contains(string(cells[:left]), label) {
			return i
		}
	}
	return -1
}

func TestZCentresTheCursorLikeZZInVim(t *testing.T) {
	m, names := manyArtists(t, 60)
	for i := 0; i < 30; i++ {
		m = keys(t, m, "down")
	}
	if got := treeLine(m, names[30]); got != m.viewRows()-1 {
		t.Fatalf("before z the cursor should be at the bottom edge, it is on line %d of %d", got, m.viewRows())
	}
	m = keys(t, m, "z")
	if got, want := treeLine(m, names[30]), m.viewRows()/2; got != want {
		t.Errorf("after z the cursor is on line %d, want the middle, %d", got, want)
	}
	if got := selectedLabel(t, m); got != names[30] {
		t.Errorf("z moved the selection to %q", got)
	}
	m = keys(t, m, "down")
	if got := treeLine(m, names[31]); got != m.viewRows()/2+1 {
		t.Errorf("the view should stay where z put it while the cursor moves inside it, line %d", got)
	}
}

func TestZNearTheEdgesCentresAsFarAsTheListAllows(t *testing.T) {
	m, names := manyArtists(t, 60)
	m = keys(t, m, "down", "down", "z")
	if got := treeLine(m, names[0]); got != 0 {
		t.Errorf("near the top the list stays at the top, first artist on line %d", got)
	}
	m = keys(t, m, "G", "z")
	if got := treeLine(m, names[59]); got != m.viewRows()-1 {
		t.Errorf("at the end the list is not scrolled past its last row, last artist on line %d", got)
	}
}

// shown lists the labels of the rows of the tree, indented by depth.
func shown(m Model) []string {
	var out []string
	for _, r := range m.rows {
		out = append(out, strings.Repeat(" ", r.Depth)+r.Label)
	}
	return out
}

func TestPlusOpensTheTreeOneLevelAtATime(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	if got := len(m.rows); got != 4 {
		t.Fatalf("starts with %d rows: %v", got, shown(m))
	}

	m = keys(t, m, "+") // the artists, and the group of tracks to identify
	if got := shown(m); len(got) != 8 {
		t.Fatalf("after + : %v", got)
	}
	for _, r := range m.rows {
		if r.Kind == kindTrack && r.Depth == 2 {
			t.Errorf("one + must not reach the tracks of an album: %v", shown(m))
		}
	}
	contains(t, m, "Shadows", "Singles", "Caught a Black Rabbit")

	m = keys(t, m, "+") // the albums
	contains(t, m, "The Wanderer", "Flow", "Along Our Way", "Wtni", "Current")

	before := shown(m)
	m = keys(t, m, "+") // nothing left to open
	if got := shown(m); strings.Join(got, "|") != strings.Join(before, "|") {
		t.Errorf("a third + changed the tree:\n%v\n%v", before, got)
	}
}

func TestEqualsDoesTheSameAsPlus(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "=")
	if len(m.rows) != 8 {
		t.Errorf("= should open a level, rows: %v", shown(m))
	}
}

func TestPlusOpensTheShallowestClosedLevelEvenWhenSomethingIsOpenAlready(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = selectTrack(t, m, "Flow") // Shawn James and Shadows are open
	m = keys(t, m, "+")
	contains(t, m, "Caught a Black Rabbit", "Singles") // the other artists opened
	lacks(t, m, "Current")                             // but not their albums
}

func TestMinusClosesEverythingAtOnceAndKeepsTheSelectionOnItsArtist(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "+", "+")
	m = selectTrack(t, m, "Flow")
	m = keys(t, m, "-")
	if got := shown(m); len(got) != 4 {
		t.Fatalf("after - : %v", got)
	}
	if got := selectedLabel(t, m); got != "Shawn James" {
		t.Errorf("the selection should stay with the artist of the track, got %q", got)
	}
	m = keys(t, m, "+")
	for _, r := range m.rows {
		if r.Kind == kindAlbum && r.Expanded {
			t.Errorf("closing must forget what was open: %v", shown(m))
		}
	}
}

func TestOpeningAndClosingAllDoNothingWhileFiltering(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "/")
	m = typed(t, m, "shawn")
	m = keys(t, m, "enter")
	before := shown(m)
	m = keys(t, m, "+")
	m = keys(t, m, "-")
	if got := shown(m); strings.Join(got, "|") != strings.Join(before, "|") {
		t.Errorf("the filtered tree changed:\n%v\n%v", before, got)
	}
}
