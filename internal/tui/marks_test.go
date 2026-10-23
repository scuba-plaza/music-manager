package tui

import (
	"testing"
)

// marking helpers: select a track by title and press space on it
func mark(t *testing.T, m Model, title string) Model {
	t.Helper()
	m = selectTrack(t, m, title)
	return keys(t, m, "space")
}

func TestSpaceMarksATrackAndMovesDown(t *testing.T) {
	m, _ := editingModel(t, "The Wanderer")
	m = keys(t, m, "space")
	if got := selectedLabel(t, m); got != "Flow" {
		t.Errorf("space should move on to the next track, now on %q", got)
	}
	contains(t, m, "●", "1 marked")
	if len(m.marks) != 1 {
		t.Errorf("marks = %v", m.marks)
	}
	m = selectTrack(t, m, "The Wanderer")
	m = keys(t, m, "space") // marking again unmarks
	lacks(t, m, "marked")
}

func TestMarkingAnAlbumMarksAllItsTracks(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "left") // the album row
	if r, _ := m.selectedRow(); r.Kind != kindAlbum {
		t.Fatalf("on %v", r.Kind)
	}
	m = keys(t, m, "space")
	contains(t, m, "3 marked")
	m = keys(t, m, "k", "space") // back on the album row: all marked, so this unmarks
	lacks(t, m, "marked")
}

func TestAPartlyMarkedAlbumShowsItAndMarksTheRestFirst(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "space") // Flow only
	m = selectTrack(t, m, "Flow")
	m = keys(t, m, "left") // album
	contains(t, m, "◐")
	m = keys(t, m, "space")
	contains(t, m, "3 marked")
}

func TestCtrlAMarksEverythingShownAndAgainClears(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "/")
	m = typed(t, m, "shawn")
	m = keys(t, m, "enter", "ctrl+a")
	contains(t, m, "3 marked")
	m = keys(t, m, "ctrl+a")
	lacks(t, m, "marked")
}

func TestEscClearsTheFilterFirstAndThenTheMarks(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "space", "/")
	m = typed(t, m, "shawn")
	m = keys(t, m, "enter", "esc")
	contains(t, m, "1 marked")
	if m.filter != "" {
		t.Error("the first esc should clear the filter")
	}
	m = keys(t, m, "esc")
	lacks(t, m, "marked")
}

func TestBulkEditOfMarkedTracks(t *testing.T) {
	m, hook := editingModel(t, "The Wanderer")
	m = keys(t, m, "space", "space") // The Wanderer, Flow
	m = keys(t, m, "e")
	contains(t, m, "Edit 2 marked tracks", "Artist", "Album", "Tracks on album", "Cut start", "Cut end", "(keep)")
	m = typed(t, m, "Shawn J.")
	m = keys(t, m, "tab", "tab") // past the Latin form of the artist, to the album
	m = typed(t, m, "-")
	contains(t, m, "Artist → Shawn J.", "Album → none")
	m = keys(t, m, "enter")

	if m.overlay != nil {
		t.Fatalf("the form is still open:\n%s", screen(m))
	}
	for _, title := range []string{"The Wanderer", "Flow"} {
		tr := trackNamed(t, hook.Library, title)
		if tr.Artist != "Shawn J." || tr.Album != "" {
			t.Errorf("%s = %+v", title, tr)
		}
	}
	if got := trackNamed(t, hook.Library, "Along Our Way"); got.Artist != "Shawn James" {
		t.Error("an unmarked track was changed")
	}
	contains(t, m, "Changed 2 tracks")

	m = keys(t, m, "u") // one step undoes both
	for _, title := range []string{"The Wanderer", "Flow"} {
		if tr := trackNamed(t, hook.Library, title); tr.Artist != "Shawn James" || tr.Album != "Shadows" {
			t.Errorf("after undo %s = %+v", title, tr)
		}
	}
}

func TestTheBulkFormExplainsMistakesAndSavesNothing(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "space", "e")
	m = keys(t, m, "tab", "tab", "tab", "tab") // tracks on album
	m = typed(t, m, "many")
	m = keys(t, m, "enter")
	contains(t, m, "whole number")
	if m.overlay == nil || hook.manyCalls != 0 {
		t.Errorf("overlay %v, calls %d", m.overlay, hook.manyCalls)
	}

	m = keys(t, m, "ctrl+u")
	m = keys(t, m, "enter")
	contains(t, m, "Nothing to change")
}

func TestEscClosesTheBulkFormWithoutChangingAnything(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "space", "e")
	m = typed(t, m, "Nobody")
	m = keys(t, m, "esc")
	if m.overlay != nil || hook.manyCalls != 0 {
		t.Errorf("overlay %v, calls %d", m.overlay, hook.manyCalls)
	}
	lacks(t, m, "Edit 1 marked")
}

func TestRenameAnAlbumFromItsRow(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "left", "e")
	contains(t, m, "Rename the album", "Shadows")
	m = keys(t, m, "tab") // the form starts on the album; go to Artist and back to be sure of the order
	m = keys(t, m, "shift+tab")
	m = keys(t, m, "ctrl+u")
	m = typed(t, m, "Shadows (Deluxe)")
	m = keys(t, m, "enter")
	for _, title := range []string{"The Wanderer", "Flow", "Along Our Way"} {
		if got := trackNamed(t, hook.Library, title).Album; got != "Shadows (Deluxe)" {
			t.Errorf("%s is on %q", title, got)
		}
	}
	contains(t, m, "Shadows (Deluxe)", "Changed 3 tracks")
}

func TestRenameAnArtistMergesItWithAnother(t *testing.T) {
	m, hook := editingModel(t, "Current")
	m = keys(t, m, "left", "left", "left") // album, close it, artist
	if r, _ := m.selectedRow(); r.Kind != kindArtist || r.Label != "Phoria" {
		t.Fatalf("on %+v", r)
	}
	m = keys(t, m, "e")
	contains(t, m, "Rename the artist")
	m = keys(t, m, "ctrl+u")
	m = typed(t, m, "Shawn James")
	m = keys(t, m, "enter")
	if got := trackNamed(t, hook.Library, "Current").Artist; got != "Shawn James" {
		t.Errorf("artist = %q", got)
	}
}

func TestEOnATrackGoesToItsFields(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "e")
	if m.overlay != nil || m.focus != paneInspector {
		t.Errorf("overlay %v, focus %v", m.overlay, m.focus)
	}
}

func TestMovingATrackSwapsItsNumberWithItsNeighbour(t *testing.T) {
	m, hook := editingModel(t, "Flow") // 7 of 12, between 1 and 12
	number := func(title string) int { return trackNamed(t, hook.Library, title).TrackNumber }

	m = keys(t, m, "J")
	if number("Flow") != 12 || number("Along Our Way") != 7 {
		t.Errorf("after J: Flow %d, Along Our Way %d", number("Flow"), number("Along Our Way"))
	}
	if got := selectedLabel(t, m); got != "Flow" {
		t.Errorf("the selection should follow the track, got %q", got)
	}
	m = keys(t, m, "J")
	contains(t, m, "last")

	m = keys(t, m, "K", "K")
	if number("Flow") != 1 || number("The Wanderer") != 7 {
		t.Errorf("after K, K: Flow %d, The Wanderer %d", number("Flow"), number("The Wanderer"))
	}
	m = keys(t, m, "K")
	contains(t, m, "first")

	m = keys(t, m, "u") // undo the last swap: both its numbers return together
	if number("Flow") != 7 || number("The Wanderer") != 1 || number("Along Our Way") != 12 {
		t.Errorf("after undo: Flow %d, The Wanderer %d, Along Our Way %d; want 7, 1, 12", number("Flow"), number("The Wanderer"), number("Along Our Way"))
	}
}

func TestOnlyNumberedAlbumTracksCanBeMoved(t *testing.T) {
	m, hook := editingModel(t, "Wtni") // a single
	m = keys(t, m, "J")
	contains(t, m, "album")
	if hook.manyCalls != 0 {
		t.Error("a single was moved")
	}
}

func TestNumberingAnAlbum(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "n")
	for title, want := range map[string]int{"The Wanderer": 1, "Flow": 2, "Along Our Way": 3} {
		tr := trackNamed(t, hook.Library, title)
		if tr.TrackNumber != want || tr.TrackTotal != 3 {
			t.Errorf("%s = %d of %d, want %d of 3", title, tr.TrackNumber, tr.TrackTotal, want)
		}
	}
	contains(t, m, "Numbered 3 tracks")
	m = keys(t, m, "u")
	if tr := trackNamed(t, hook.Library, "Flow"); tr.TrackNumber != 7 || tr.TrackTotal != 12 {
		t.Errorf("after undo Flow = %d of %d", tr.TrackNumber, tr.TrackTotal)
	}
}

func TestNumberingNeedsTheWholeAlbumInView(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "/")
	m = typed(t, m, "flow")
	m = keys(t, m, "enter", "down", "down", "n") // on the track, inside a filter
	contains(t, m, "filter")
	if hook.manyCalls != 0 {
		t.Error("renumbered a filtered view")
	}
}

func TestTheBulkFormSetsTheReleaseDateOfTheMarkedTracks(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "space", "e")
	contains(t, m, "Released")
	m = keys(t, m, "tab", "tab", "tab", "tab", "tab") // artist, its Latin form, album, its Latin form, tracks on album, released
	m = typed(t, m, "never")
	m = keys(t, m, "enter")
	contains(t, m, "is not a date")
	if hook.manyCalls != 0 {
		t.Error("a bad date was saved")
	}

	m = keys(t, m, "ctrl+u")
	m = typed(t, m, "2012")
	contains(t, m, "Released → 2012")
	m = keys(t, m, "enter")
	if got := trackNamed(t, hook.Library, "Flow"); got.Released != "2012" {
		t.Errorf("saved %+v", got)
	}
	if got := trackNamed(t, hook.Library, "The Wanderer"); got.Released != "" {
		t.Errorf("an unmarked track was changed: %+v", got)
	}
}

func TestRenamingAnArtistCanSetItsLatinFormOnAllItsTracks(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "left", "left", "left", "e") // album, close it, artist
	contains(t, m, "Rename the artist", "Artist (Latin)", "Album (Latin)", "Genre (Latin)")
	m = keys(t, m, "tab")
	m = typed(t, m, "Shawn J")
	contains(t, m, "Artist (Latin) → Shawn J")
	m = keys(t, m, "enter")
	for _, title := range []string{"The Wanderer", "Flow", "Along Our Way"} {
		if got := trackNamed(t, hook.Library, title); got.ArtistLatin != "Shawn J" || got.Artist != "Shawn James" {
			t.Errorf("%s: artist %q, its Latin form %q", title, got.Artist, got.ArtistLatin)
		}
	}
	contains(t, m, "Changed 3 tracks")
}
