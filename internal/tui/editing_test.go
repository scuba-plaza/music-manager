package tui

import (
	"errors"
	"testing"

	"music-manager/internal/app"

	"music-manager/internal/manifest"
)

// editingModel is a library with a few tracks, wrapped so edits can be
// counted or broken, with the track titled title selected.
func editingModel(t *testing.T, title string) (Model, *editHook) {
	t.Helper()
	m, lib := browserModel(t, 120, 36)
	hook := &editHook{Library: lib}
	m.lib = hook
	return selectTrack(t, m, title), hook
}

func TestRightOnATrackFocusesTheInspectorAndLeftComesBack(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "right")
	if m.focus != paneInspector {
		t.Fatal("right did not focus the inspector")
	}
	contains(t, m, "enter edit", "back")
	m = keys(t, m, "left")
	if m.focus != paneBrowser {
		t.Error("left did not go back")
	}
	m = keys(t, m, "enter")
	if m.focus != paneInspector {
		t.Error("enter on a track should focus the inspector too")
	}
	m = keys(t, m, "esc")
	if m.focus != paneBrowser {
		t.Error("esc did not go back")
	}
	m = keys(t, m, "tab", "tab")
	if m.focus != paneBrowser {
		t.Error("tab toggles between the panes")
	}
}

func TestOnlyTracksHaveFieldsToEdit(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "left", "left", "left") // album, close it, artist
	if r, _ := m.selectedRow(); r.Kind != kindArtist {
		t.Fatalf("setup: %v", r.Kind)
	}
	m = keys(t, m, "tab")
	if m.focus != paneBrowser {
		t.Error("an artist has no fields to focus")
	}
}

func TestUpAndDownChooseTheField(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "right")
	if m.field != 0 {
		t.Fatalf("starts on field %d", m.field)
	}
	m = keys(t, m, "down", "down", "j")
	if trackFields[m.field] != fTrack {
		t.Errorf("on %v, want Track", trackFields[m.field])
	}
	m = keys(t, m, "k", "up", "up", "up")
	if m.field != 0 {
		t.Errorf("up stops at the first field, on %d", m.field)
	}
	for range trackFields {
		m = keys(t, m, "down")
	}
	if m.field != len(trackFields)-1 {
		t.Errorf("down stops at the last field, on %d", m.field)
	}
}

func TestEditingATitleSavesItAtOnce(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	lib := hook.Library
	m = keys(t, m, "right", "enter")
	contains(t, m, "enter save", "esc cancel")
	m = clearField(t, m)
	m = typed(t, m, "Flow Again")
	m = keys(t, m, "enter")

	if got := trackNamed(t, lib, "Flow Again"); got.OriginalArtist != "Shawn James" {
		t.Errorf("saved %+v", got)
	}
	if m.editing {
		t.Error("still editing after saving")
	}
	contains(t, m, "Saved", "Flow Again")
	if r, _ := m.selectedRow(); r.Kind != kindTrack || r.Label != "Flow Again" {
		t.Errorf("the selection should stay on the track, got %+v", r)
	}
}

func TestEscapeCancelsAnEdit(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "right", "enter")
	m = typed(t, m, " nonsense")
	m = keys(t, m, "esc")
	if m.editing || hook.calls != 0 {
		t.Errorf("editing %v, library calls %d", m.editing, hook.calls)
	}
	if trackNamed(t, hook.Library, "Flow").OriginalTitle != "Flow" {
		t.Error("a cancelled edit was saved")
	}
	lacks(t, m, "nonsense")
	if m.focus != paneInspector {
		t.Error("cancelling an edit stays in the inspector")
	}
}

func TestTypingCommandLettersWhileEditingJustTypes(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "right", "enter")
	m = clearField(t, m)
	m = typed(t, m, "quiz a/d")
	if m.dr.text != "quiz a/d" {
		t.Errorf("text = %q", m.dr.text)
	}
	if m.focus != paneInspector || !m.editing {
		t.Error("typing changed the mode")
	}
}

func TestAnInvalidValueIsExplainedAndNothingIsSaved(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "right", "enter")
	m = clearField(t, m)
	m = keys(t, m, "enter")
	contains(t, m, "Title cannot be empty")
	if !m.editing || hook.calls != 0 {
		t.Errorf("editing %v, calls %d", m.editing, hook.calls)
	}
	m = typed(t, m, "Fixed")
	lacks(t, m, "cannot be empty") // the message goes away as soon as you type
	m = keys(t, m, "enter")
	if trackNamed(t, hook.Library, "Fixed").OriginalArtist != "Shawn James" {
		t.Error("the corrected value was not saved")
	}
}

func TestAProblemFoundByTheLibraryIsShownAndEditingContinues(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	hook.err = errors.New("trims (5s + 5s) leave nothing of a 2.6s track")
	m = keys(t, m, "right", "enter")
	m = typed(t, m, "!")
	m = keys(t, m, "enter")
	contains(t, m, "leave nothing")
	if !m.editing {
		t.Error("a failed save must not throw the edit away")
	}
	hook.err = nil
	m = keys(t, m, "enter")
	if m.editing {
		t.Error("the retry should have saved")
	}
}

func TestSavingWithoutChangingAnythingDoesNotTouchTheLibrary(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "right", "enter", "enter")
	if hook.calls != 0 || m.editing {
		t.Errorf("calls %d, editing %v", hook.calls, m.editing)
	}
}

func TestClearingTheAlbumMakesItASingleAndTheSelectionFollows(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "right", "down", "down", "enter")
	m = clearField(t, m)
	m = keys(t, m, "enter")
	if got := trackNamed(t, hook.Library, "Flow"); got.OriginalAlbum != "" {
		t.Fatalf("album = %q", got.OriginalAlbum)
	}
	if r, _ := m.selectedRow(); r.Kind != kindTrack || r.Track.OriginalTitle != "Flow" {
		t.Errorf("the selection lost the track: %+v", r)
	}
	contains(t, m, "Singles", "Shawn James/Flow.mp3")
}

func TestEditingTheTrackNumberAndTheAlbumSize(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "right", "down", "down", "down", "enter")
	contains(t, m, "of")
	m = clearField(t, m)
	m = typed(t, m, "9")
	m = keys(t, m, "tab")
	m = clearField(t, m)
	m = typed(t, m, "14")
	m = keys(t, m, "enter")
	got := trackNamed(t, hook.Library, "Flow")
	if got.TrackNumber != 9 || got.TrackTotal != 14 {
		t.Errorf("= %d of %d", got.TrackNumber, got.TrackTotal)
	}
	contains(t, m, "9 of 14")
}

func TestEditingACut(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = focusField(t, m, fCutStart)
	m = keys(t, m, "enter")
	m = typed(t, m, "0:01")
	m = keys(t, m, "enter")
	if got := trackNamed(t, hook.Library, "Flow"); got.TrimStart.Seconds() != 1 {
		t.Errorf("cut start = %v", got.TrimStart)
	}

	m = keys(t, m, "down", "enter")
	m = typed(t, m, "banana")
	m = keys(t, m, "enter")
	contains(t, m, "invalid")
}

func TestThePathPreviewFollowsWhatYouType(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	contains(t, m, "Shawn James/Shadows/07 - Flow.mp3")
	m = keys(t, m, "right", "down", "down", "enter")
	m = clearField(t, m)
	m = typed(t, m, "Other")
	contains(t, m, "Shawn James/Other/07 - Flow.mp3")
	m = keys(t, m, "esc")
	contains(t, m, "Shawn James/Shadows/07 - Flow.mp3")
}

func TestUndoAndRedo(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	lib := hook.Library
	m = keys(t, m, "u")
	contains(t, m, "Nothing to undo")

	m = keys(t, m, "right", "enter")
	m = typed(t, m, "!")
	m = keys(t, m, "enter")
	trackNamed(t, lib, "Flow!")

	m = keys(t, m, "u")
	trackNamed(t, lib, "Flow")
	contains(t, m, "Undid")
	if r, _ := m.selectedRow(); r.Label != "Flow" {
		t.Errorf("selection after undo: %q", r.Label)
	}

	m = keys(t, m, "ctrl+r")
	trackNamed(t, lib, "Flow!")
	contains(t, m, "Redid")
	m = keys(t, m, "ctrl+r")
	contains(t, m, "Nothing to redo")

	// A new change forgets what could have been redone.
	m = keys(t, m, "u", "enter", "x", "enter") // undo, then edit again
	m = typed(t, m, "")
	m = keys(t, m, "ctrl+r")
	contains(t, m, "Nothing to redo")
}

func TestUndoWorksFromTheBrowserToo(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "right", "enter")
	m = typed(t, m, "?")
	m = keys(t, m, "enter", "left", "u")
	trackNamed(t, hook.Library, "Flow")
}

func TestATrackNobodyIdentifiedStartsFromWhatTheFileSaid(t *testing.T) {
	a := testApp(t)
	seed(t, a, seedTrack{"Anchor", "Band", "Album", 1, 1, 0})
	seedTagged(t, a, "03 Old Name", "Old Title", "Old Artist", "Old Album", "3/9")
	lib := NewLibrary(a)
	m := newTestModel(t, lib, 120, 36)
	m = selectTrack(t, m, "Old Title")
	contains(t, m, "Not identified")

	m = keys(t, m, "right", "enter")
	if m.dr.text != "Old Title" {
		t.Fatalf("the field starts as %q", m.dr.text)
	}
	m = keys(t, m, "enter") // accept the old title as it is
	if got := trackNamed(t, lib, "Old Title"); got.Status != manifest.Unidentified || got.OriginalTitle != "Old Title" {
		t.Errorf("after the title: %+v", got)
	}
	m = keys(t, m, "down", "enter", "enter") // and the old artist
	got := trackNamed(t, lib, "Old Title")
	if got.Status != manifest.Identified || got.OriginalArtist != "Old Artist" {
		t.Fatalf("after title and artist: %+v", got)
	}
	if r, _ := m.selectedRow(); r.Track.ID != got.ID {
		t.Errorf("the selection should follow the track out of Needs work, got %+v", r)
	}
	lacks(t, m, "Not identified")
}

// focusField opens the inspector on the selected track and moves to field f.
func focusField(t *testing.T, m Model, f fieldKind) Model {
	t.Helper()
	m = keys(t, m, "right")
	for trackFields[m.field] != f {
		before := m.field
		m = keys(t, m, "down")
		if m.field == before {
			t.Fatalf("no field %v", f)
		}
	}
	return m
}

func TestEditingTheReleaseDate(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = focusField(t, m, fReleased)
	contains(t, m, "Released")
	m = keys(t, m, "enter")
	contains(t, m, "1999-05-17") // the format is explained
	m = typed(t, m, "2012-03")
	m = keys(t, m, "enter")

	if got := trackNamed(t, hook.Library, "Flow"); got.Released != "2012-03" {
		t.Errorf("saved %+v", got)
	}
	contains(t, m, "Saved", "2012-03")

	m = keys(t, m, "enter")
	m = clearField(t, m)
	m = keys(t, m, "enter")
	if got := trackNamed(t, hook.Library, "Flow"); got.Released != "" {
		t.Errorf("an empty date should forget it: %q", got.Released)
	}
}

func TestABadReleaseDateIsExplainedAndNothingIsSaved(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = focusField(t, m, fReleased)
	m = typed(t, keys(t, m, "enter"), "next spring")
	m = keys(t, m, "enter")
	if !m.editing || hook.calls != 0 {
		t.Errorf("editing %v, library calls %d", m.editing, hook.calls)
	}
	contains(t, m, "is not a date", "1999-05-17")
}

func TestTheInspectorSaysWhenATrackWasAddedAndLastEdited(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	tr := trackNamed(t, hook.Library, "Flow")
	contains(t, m, "added "+app.SortValue(tr, app.SortAdded), "edited "+app.SortValue(tr, app.SortEdited))
}

func TestEditingTheGenre(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = focusField(t, m, fGenre)
	contains(t, m, "Genre")
	m = keys(t, m, "enter")
	contains(t, m, "Folk", "empty for none") // the kind of thing that goes here is shown
	m = typed(t, m, "Neoclassical")
	m = keys(t, m, "enter")

	if got := trackNamed(t, hook.Library, "Flow"); got.Genre != "Neoclassical" {
		t.Errorf("saved %+v", got)
	}
	contains(t, m, "Saved", "Neoclassical")

	m = keys(t, m, "enter")
	m = clearField(t, m)
	m = keys(t, m, "enter")
	if got := trackNamed(t, hook.Library, "Flow"); got.Genre != "" {
		t.Errorf("an empty genre should forget it: %q", got.Genre)
	}
}

func TestTheGenreSitsBetweenTheReleaseDateAndTheCover(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = focusField(t, m, fReleased)
	m = keys(t, m, "down")
	if trackFields[m.field] != fGenre {
		t.Errorf("below Released is %v, want Genre", trackFields[m.field])
	}
	m = keys(t, m, "down")
	if trackFields[m.field] != fCover {
		t.Errorf("below Genre is %v, want Cover", trackFields[m.field])
	}
}
