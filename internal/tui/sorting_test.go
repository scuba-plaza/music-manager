package tui

import (
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

// listed lists the titles of the identified tracks in the order the list
// shows them. The one track nobody identified is left out: where it goes
// depends on a file name.
func listed(m Model) []string {
	var out []string
	for _, r := range m.rows {
		if r.Kind == kindTrack && r.Track.Status == manifest.Identified {
			out = append(out, r.Label)
		}
	}
	return out
}

func same(got []string, want ...string) bool {
	return strings.Join(got, "|") == strings.Join(want, "|")
}

// sorted opens the sort menu and picks entry n (1 is the tree, 2 the first
// key), in the direction asked for.
func sorted(t *testing.T, m Model, n int, reversed bool) Model {
	t.Helper()
	m = keys(t, m, "S")
	if reversed != m.sortReverse {
		m = keys(t, m, "r")
	}
	return keys(t, m, string(rune('0'+n)))
}

const (
	byTree = iota + 1
	byArtist
	byAlbum
	byTitle
	byTrack
	byLength
	byReleased
	byAdded
	byEdited
)

func TestSOpensTheSortMenuAndEscLeavesTheListAlone(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	before := screen(m)
	m = keys(t, m, "S")
	contains(t, m, "Sort", "Tree", "Artist", "Album", "Title", "Track number", "Length", "Release date", "Date added", "Date edited", "enter", "esc")
	m = keys(t, m, "esc")
	if m.overlay != nil || screen(m) != before {
		t.Errorf("esc should close the menu and change nothing:\n%s", screen(m))
	}
	m = keys(t, m, "S", "down", "down", "esc")
	if m.sortKey != "" {
		t.Errorf("moving about the menu changed the sort to %q", m.sortKey)
	}
}

func TestEachSortKeyPutsAllTracksInOneFlatListInThatOrder(t *testing.T) {
	for _, c := range []struct {
		name     string
		n        int
		reversed bool
		want     []string
	}{
		{"title", byTitle, false, []string{"Along Our Way", "Current", "Flow", "The Wanderer", "Wtni"}},
		{"title reversed", byTitle, true, []string{"Wtni", "The Wanderer", "Flow", "Current", "Along Our Way"}},
		{"artist", byArtist, false, []string{"Wtni", "Current", "The Wanderer", "Flow", "Along Our Way"}},
		{"album, singles last", byAlbum, false, []string{"Current", "The Wanderer", "Flow", "Along Our Way", "Wtni"}},
		{"track number, unnumbered last", byTrack, false, []string{"The Wanderer", "Current", "Flow", "Along Our Way", "Wtni"}},
		{"track number reversed keeps unnumbered last", byTrack, true, []string{"Along Our Way", "Flow", "Current", "The Wanderer", "Wtni"}},
		{"length", byLength, false, []string{"The Wanderer", "Flow", "Along Our Way", "Current", "Wtni"}},
		{"date added, newest first", byAdded, false, []string{"Wtni", "Current", "Along Our Way", "Flow", "The Wanderer"}},
		{"date added, oldest first", byAdded, true, []string{"The Wanderer", "Flow", "Along Our Way", "Current", "Wtni"}},
	} {
		m, _ := browserModel(t, 120, 36)
		m = sorted(t, m, c.n, c.reversed)
		if got := listed(m); !same(got, c.want...) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
		for _, r := range m.rows {
			if r.Kind != kindTrack || r.Depth != 0 {
				t.Errorf("%s: a sorted list is flat, found %+v", c.name, r)
			}
		}
	}
}

func TestTheFlatListShowsEveryTrackWithoutOpeningAnything(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	lacks(t, m, "The Wanderer")
	m = sorted(t, m, byTitle, false)
	contains(t, m, "Along Our Way", "Current", "Flow", "The Wanderer", "Wtni")
	contains(t, m, "Shawn James", "Phoria", "Fairuz") // the artist stays beside each title
	lacks(t, m, "▸", "▾")
	contains(t, m, "Library", "Title")
}

func TestTheListSaysWhatItIsSortedBy(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	for _, c := range []struct {
		n        int
		reversed bool
		want     string
	}{
		{byTitle, false, "Title ▲"},
		{byTitle, true, "Title ▼"},
		{byAdded, false, "Date added ▼"}, // dates start with the newest
		{byAdded, true, "Date added ▲"},
	} {
		got := lines(sorted(t, m, c.n, c.reversed))[1]
		if !strings.Contains(got, c.want) {
			t.Errorf("pane title %q lacks %q", got, c.want)
		}
	}
	if got := lines(sorted(t, sorted(t, m, byTitle, false), byTree, false))[1]; strings.Contains(got, "▲") || strings.Contains(got, "▼") {
		t.Errorf("the tree is not sorted: %q", got)
	}
}

func TestDatesAreShownBesideTheTracksTheyBelongTo(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	a := appOf(t, hook.Library)
	for _, c := range [][2]string{{"Flow", "2001"}, {"Current", "1999-05"}} { // Current is edited last
		if _, err := a.Edit(trackNamed(t, hook.Library, c[0]).ID, app.Edit{Released: &c[1]}); err != nil {
			t.Fatal(err)
		}
	}
	m = send(t, m, m.load())

	m = sorted(t, m, byReleased, false)
	if got := listed(m); !same(got, "Flow", "Current", "Wtni", "The Wanderer", "Along Our Way") {
		t.Errorf("release dates, newest first, undated last: %v", got)
	}
	contains(t, m, "2001", "1999-05", "—")
	m = sorted(t, m, byReleased, true)
	if got := listed(m); !same(got, "Current", "Flow", "Wtni", "The Wanderer", "Along Our Way") {
		t.Errorf("oldest first, undated still last: %v", got)
	}

	m = sorted(t, m, byAdded, false)
	flow := trackNamed(t, hook.Library, "Flow")
	contains(t, m, app.SortValue(flow, app.SortAdded))

	// Editing a track makes it the last one edited.
	m = sorted(t, m, byEdited, false)
	if got := listed(m); !same(got, "Current", "Flow", "Wtni", "Along Our Way", "The Wanderer") {
		t.Errorf("the track edited last comes first: %v", got)
	}
}

func TestSwitchingKeepsTheSelectedTrackSelected(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = selectTrack(t, m, "Flow")
	m = sorted(t, m, byTitle, false)
	if got := selectedLabel(t, m); got != "Flow" {
		t.Errorf("selected %q after sorting", got)
	}
	contains(t, m, "Details", "Shawn James") // the inspector follows
	for selectedLabel(t, m) != "Current" {
		m = keys(t, m, "up")
	}
	m = sorted(t, m, byTree, false)
	if got := selectedLabel(t, m); got != "Current" {
		t.Errorf("selected %q after going back to the tree", got)
	}
	// The album of Current was closed in the tree; it opened up to show the track.
	if treeLine(m, "Caught a Black Rabbit") < 0 || treeLine(m, "Current") < 0 {
		t.Errorf("the tree does not show the selected track:\n%s", screen(m))
	}
}

func TestSortingFromAnArtistRowLandsOnItsFirstTrack(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	for selectedLabel(t, m) != "Shawn James" {
		m = keys(t, m, "down")
	}
	m = sorted(t, m, byTitle, false)
	if got := selectedLabel(t, m); got != "The Wanderer" {
		t.Errorf("selected %q, want the artist's first track", got)
	}
}

func TestTheFilterStillAppliesToASortedList(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = sorted(t, m, byTitle, false)
	m = keys(t, m, "/")
	m = typed(t, m, "shawn")
	m = keys(t, m, "enter")
	if got := listed(m); !same(got, "Along Our Way", "Flow", "The Wanderer") {
		t.Errorf("filtered: %v", got)
	}
	m = keys(t, m, "esc")
	if got := listed(m); len(got) != 5 {
		t.Errorf("unfiltered: %v", got)
	}
}

func TestEditingATrackInASortedListMovesItAndKeepsItSelected(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = sorted(t, m, byTitle, false)
	m = keys(t, m, "right", "enter")
	m = clearField(t, m)
	m = typed(t, m, "Zzz")
	m = keys(t, m, "enter")
	if got := listed(m); !same(got, "Along Our Way", "Current", "The Wanderer", "Wtni", "Zzz") {
		t.Errorf("after renaming: %v", got)
	}
	if got := selectedLabel(t, m); got != "Zzz" {
		t.Errorf("selected %q", got)
	}
	if trackNamed(t, hook.Library, "Zzz").OriginalArtist != "Shawn James" {
		t.Error("not saved")
	}
}

func TestTheTreeKeysExplainThemselvesInASortedList(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = sorted(t, m, byTitle, false)
	for _, k := range []string{"+", "-", "left"} {
		m = keys(t, m, k)
		if k != "left" {
			contains(t, m, "sorted", "S")
		}
		if len(m.rows) < 6 {
			t.Fatalf("%s changed the list: %v", k, shown(m))
		}
	}
	m = keys(t, m, "J")
	contains(t, m, "tree") // moving a track only makes sense in its album
}

func TestSortedListsFitEverySize(t *testing.T) {
	for _, size := range [][2]int{{160, 50}, {120, 36}, {100, 30}, {80, 24}, {65, 20}, {60, 16}} {
		for _, n := range []int{byTitle, byTrack, byReleased, byAdded} {
			m, _ := browserModel(t, size[0], size[1])
			m = sorted(t, m, n, false)
			ls := lines(m)
			if len(ls) != size[1] {
				t.Errorf("%dx%d sort %d: %d lines", size[0], size[1], n, len(ls))
			}
			for i, l := range ls {
				if w := len([]rune(l)); w > size[0] {
					t.Errorf("%dx%d sort %d: line %d is %d wide: %q", size[0], size[1], n, i, w, l)
				}
			}
		}
	}
}

func TestTheMenuSwitchesDirectionWithRAndRemembersTheChoice(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "S")
	lacks(t, m, "reverse: on")
	m = keys(t, m, "r")
	contains(t, m, "reverse: on")
	m = keys(t, m, "down", "down", "down", "enter") // Tree, Artist, Album, Title
	if m.sortKey != app.SortTitle || !m.sortReverse {
		t.Errorf("sort %q, reversed %v", m.sortKey, m.sortReverse)
	}
	m = keys(t, m, "S")
	contains(t, m, "reverse: on") // the menu opens as the list is now
	m = keys(t, m, "esc")
}

func TestEnterOnTheTreeGoesBack(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = sorted(t, m, byTitle, false)
	m = keys(t, m, "S", "up", "up", "up", "enter") // Title, Album, Artist, Tree
	if m.sortKey != "" || m.sortReverse {
		t.Errorf("sort %q, reversed %v", m.sortKey, m.sortReverse)
	}
}

func TestTheHelpScreenListsTheSortKey(t *testing.T) {
	m, _ := browserModel(t, 120, 40)
	m = keys(t, m, "?")
	contains(t, m, "sort")
}
