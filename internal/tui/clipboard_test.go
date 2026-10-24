package tui

import (
	"fmt"
	"slices"
	"testing"

	"music-manager/internal/app"

	"music-manager/internal/testmp3"
)

// c copies attributes of a track and v pastes them on what the cursor is on:
// a track, the marked tracks, an album or an artist.

// withCoverOn gives the track a cover, behind the model's back, and loads it.
func withCoverOn(t *testing.T, m Model, hook *editHook, title string, image []byte) Model {
	t.Helper()
	if err := appOf(t, hook.Library).SetCover(trackNamed(t, hook.Library, title).ID, image); err != nil {
		t.Fatal(err)
	}
	return send(t, m, m.Init())
}

// toTrackField is the model with the cursor of the pane on the field f of the
// track selected.
func toTrackField(t *testing.T, m Model, f fieldKind) Model {
	t.Helper()
	if m.focus != paneInspector {
		m = keys(t, m, "right")
	}
	m = keys(t, m, fmt.Sprint(tabOf(f)+1)) // the tab of the field
	for i := 0; i < len(trackFields) && m.curField() != f; i++ {
		if m.field < slices.Index(trackFields, f) {
			m = keys(t, m, "down")
		} else {
			m = keys(t, m, "up")
		}
	}
	if m.curField() != f {
		t.Fatalf("no field %v", f)
	}
	return m
}

func TestACoverIsCopiedFromOneTrackAndPastedOnAnotherInOneUndoableStep(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = withCoverOn(t, m, hook, "Flow", testmp3.PNG)
	m = selectTrack(t, m, "Flow")
	m = toTrackField(t, m, fCover)

	m = keys(t, m, "c")
	contains(t, m, "Copied the cover of Flow")
	m = keys(t, m, "esc")
	m = selectTrack(t, m, "Along Our Way")
	if coverOf(t, hook.Library, "Along Our Way") != "" {
		t.Fatal("setup: the track has a cover already")
	}
	m = keys(t, m, "v")
	contains(t, m, "Pasted the cover on 1 track", "u undoes it")
	want := coverOf(t, hook.Library, "Flow")
	if got := coverOf(t, hook.Library, "Along Our Way"); got == "" || got != want {
		t.Errorf("the cover is %q, want %q", got, want)
	}
	if hook.manyCalls != 1 {
		t.Errorf("%d saves, want one", hook.manyCalls)
	}
	if got := coverOf(t, hook.Library, "The Wanderer"); got != "" {
		t.Errorf("a track that was not pasted on got the cover: %q", got)
	}

	m = keys(t, m, "u")
	if got := coverOf(t, hook.Library, "Along Our Way"); got != "" {
		t.Errorf("undo left the cover %q", got)
	}
	m = keys(t, m, "ctrl+r")
	if got := coverOf(t, hook.Library, "Along Our Way"); got != want {
		t.Errorf("redo gave %q, want %q", got, want)
	}
}

// copyFrom copies the field f of the track with the title, from the pane, and
// comes back to the tree.
func copyFrom(t *testing.T, m Model, title string, f fieldKind) Model {
	t.Helper()
	m = selectTrack(t, m, title)
	m = toTrackField(t, m, f)
	m = keys(t, m, "c")
	return keys(t, m, "esc")
}

func TestVWithNothingCopiedSaysHowToCopy(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "v")
	contains(t, m, "Nothing is copied yet", "c copies")
	if hook.manyCalls != 0 {
		t.Errorf("%d saves", hook.manyCalls)
	}
}

func TestWhatCannotBeCopiedIsSaidAndKeepsWhatWasCopied(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = withEdits(t, m, hook, map[string]app.Edit{"Flow": {Genre: ptr("Soul")}})
	m = copyFrom(t, m, "Flow", fGenre)

	m = selectTrack(t, m, "Flow")
	m = toTrackField(t, m, fCover) // Flow has no cover
	m = keys(t, m, "c")
	contains(t, m, "no cover to copy")
	m = toTrackField(t, m, fTitle)
	m = keys(t, m, "c")
	contains(t, m, "title belongs to its track")
	m = toTrackField(t, m, fCutStart)
	m = keys(t, m, "c")
	contains(t, m, "belongs to its track")

	// The genre is still what v pastes.
	m = keys(t, m, "esc")
	m = selectTrack(t, m, "Current")
	m = keys(t, m, "v")
	if got := genreOf(t, hook.Library, "Current"); got != "Soul" {
		t.Errorf("the genre is %q", got)
	}
}

func TestAnArtistIsPastedWithItsLatinFormAndAnOldFormGoesWithTheOldName(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = withEdits(t, m, hook, map[string]app.Edit{
		"Flow":    {ArtistLatin: ptr("Shawn J")},
		"Current": {ArtistLatin: ptr("Foria")},
	})
	m = copyFrom(t, m, "Flow", fArtist)
	m = selectTrack(t, m, "Current")
	m = keys(t, m, "v")
	if got := trackNamed(t, hook.Library, "Current"); got.Artist != "Shawn James" || got.ArtistLatin != "Shawn J" {
		t.Errorf("artist %q, its form %q", got.Artist, got.ArtistLatin)
	}

	// A name without a form of its own takes the target's old form away.
	m = withEdits(t, m, hook, map[string]app.Edit{"Along Our Way": {Artist: ptr("Plain"), ArtistLatin: ptr("")}, "Wtni": {ArtistLatin: ptr("Old form")}})
	m = copyFrom(t, m, "Along Our Way", fArtist)
	m = selectTrack(t, m, "Wtni")
	m = keys(t, m, "v")
	if got := trackNamed(t, hook.Library, "Wtni"); got.Artist != "Plain" || got.ArtistLatin != "" {
		t.Errorf("artist %q, its form %q: the form of the old name stayed", got.Artist, got.ArtistLatin)
	}
}

func TestTheAlbumReleasedAndNumberOfTracksAreCopiedToo(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = withEdits(t, m, hook, map[string]app.Edit{"Flow": {Released: ptr("2015-07")}})
	m = copyFrom(t, m, "Flow", fReleased)
	m = selectTrack(t, m, "Current")
	m = keys(t, m, "v")
	if got := trackNamed(t, hook.Library, "Current").Released; got != "2015-07" {
		t.Errorf("released %q", got)
	}

	m = copyFrom(t, m, "Flow", fTrack) // "7 of 12": the 12 is copied
	m = selectTrack(t, m, "Wtni")
	m = keys(t, m, "v")
	contains(t, m, "tracks on album")
	if got := trackNamed(t, hook.Library, "Wtni"); got.TrackTotal != 12 || got.TrackNumber == 7 {
		t.Errorf("track %d of %d: the number of the track must stay its own", got.TrackNumber, got.TrackTotal)
	}

	m = copyFrom(t, m, "Flow", fAlbum)
	m = selectTrack(t, m, "Current")
	m = keys(t, m, "v")
	if got := trackNamed(t, hook.Library, "Current"); got.Album != "Shadows" {
		t.Errorf("album %q", got.Album)
	}
}

func TestPastingWhatTheTrackAlreadyHasChangesNothing(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = withEdits(t, m, hook, map[string]app.Edit{"Flow": {Genre: ptr("Soul")}})
	m = copyFrom(t, m, "Flow", fGenre)
	m = selectTrack(t, m, "Flow")
	m = keys(t, m, "v")
	contains(t, m, "Nothing to paste", "already has the genre")
	if hook.manyCalls != 0 {
		t.Errorf("%d saves", hook.manyCalls)
	}
}

func TestVWorksInTheFieldsOfATrackToo(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = withEdits(t, m, hook, map[string]app.Edit{"Flow": {Genre: ptr("Soul")}})
	m = copyFrom(t, m, "Flow", fGenre)
	m = selectTrack(t, m, "Current")
	m = keys(t, m, "right", "v")
	if m.focus != paneInspector {
		t.Error("pasting left the fields")
	}
	if got := genreOf(t, hook.Library, "Current"); got != "Soul" {
		t.Errorf("the genre is %q", got)
	}
}

func TestMarkedTracksAreWhatIsPastedOnNotTheTrackUnderTheCursor(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = withEdits(t, m, hook, map[string]app.Edit{"Flow": {Genre: ptr("Soul")}})
	m = copyFrom(t, m, "Flow", fGenre)
	m = selectTrack(t, m, "Current")
	m = keys(t, m, "space")
	m = selectTrack(t, m, "Wtni")
	m = keys(t, m, "space")
	m = selectTrack(t, m, "The Wanderer") // not marked
	m = keys(t, m, "v")
	contains(t, m, "on 2 tracks")
	for _, title := range []string{"Current", "Wtni"} {
		if got := genreOf(t, hook.Library, title); got != "Soul" {
			t.Errorf("%s: genre %q", title, got)
		}
	}
	if got := genreOf(t, hook.Library, "The Wanderer"); got != "" {
		t.Errorf("the track under the cursor got the genre: %q", got)
	}
}

func TestAnAlbumAndAnArtistGetItOnAllTheirTracksWhateverTheFilterShows(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = withCoverOn(t, m, hook, "Current", testmp3.PNG)
	m = copyFrom(t, m, "Current", fCover)

	m = keys(t, m, "/", "f", "l", "o", "w", "enter") // only Flow is in view
	m = selectTrack(t, m, "Flow")
	m = keys(t, m, "up") // its album
	if r, _ := m.selectedRow(); r.Kind != kindAlbum {
		t.Fatalf("setup: on a %v", r.Kind)
	}
	m = keys(t, m, "v")
	contains(t, m, "on 3 tracks")
	for _, title := range []string{"The Wanderer", "Flow", "Along Our Way"} {
		if got := coverOf(t, hook.Library, title); got == "" || got != coverOf(t, hook.Library, "Current") {
			t.Errorf("%s: cover %q", title, got)
		}
	}
	if got := coverOf(t, hook.Library, "Wtni"); got != "" {
		t.Errorf("another artist's track got the cover: %q", got)
	}

	// An artist: all its tracks as well.
	m = keys(t, m, "esc", "esc") // clear the filter
	m = withEdits(t, m, hook, map[string]app.Edit{"Flow": {Genre: ptr("Soul")}})
	m = copyFrom(t, m, "Flow", fGenre)
	m = selectTrack(t, m, "Flow")
	m = keys(t, m, "up", "up", "up") // The Wanderer, the album, the artist
	if r, _ := m.selectedRow(); r.Kind != kindArtist {
		t.Fatalf("setup: on a %v", r.Kind)
	}
	m = keys(t, m, "v")
	for _, title := range []string{"The Wanderer", "Along Our Way"} {
		if got := genreOf(t, hook.Library, title); got != "Soul" {
			t.Errorf("%s: genre %q", title, got)
		}
	}
}

func TestVOnAPlaylistOrAGroupRowSaysWhatToSelect(t *testing.T) {
	lib, _ := playlistLib(t)
	hook := &editHook{Library: lib}
	m := newTestModel(t, hook, 120, 36)
	if err := appOf(t, lib).CreatePlaylist("Gym"); err != nil {
		t.Fatal(err)
	}
	if _, err := appOf(t, lib).Edit(trackNamed(t, lib, "Flow").ID, app.Edit{Genre: ptr("Soul")}); err != nil {
		t.Fatal(err)
	}
	m = send(t, m, m.Init())
	m = copyFrom(t, m, "Flow", fGenre)
	m = keys(t, m, "g") // the group of playlists
	m = keys(t, m, "v")
	contains(t, m, "Select a track, an album or an artist")
	m = keys(t, m, "right", "down", "v") // a playlist
	contains(t, m, "Select a track, an album or an artist")
	if hook.manyCalls != 0 {
		t.Errorf("%d saves", hook.manyCalls)
	}
}

// ---- the checklist

func TestCInTheTreeListsWhatTheTrackHasAndEnterCopiesAllThatIsTicked(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = withEdits(t, m, hook, map[string]app.Edit{"Flow": {Genre: ptr("Soul"), Released: ptr("2015"), ArtistLatin: ptr("Shawn J")}})
	m = withCoverOn(t, m, hook, "Flow", testmp3.PNG)
	m = selectTrack(t, m, "Flow")

	m = keys(t, m, "c")
	contains(t, m, "Copy from Flow", "[x] Cover", "[x] Artist", "Shawn James", "Shawn J", "[x] Album", "Shadows",
		"[x] Tracks on album", "12", "[x] Released", "2015", "[x] Genre", "Soul", "space ticks", "enter copies")
	m = keys(t, m, "enter")
	if m.overlay != nil {
		t.Fatal("enter did not close the checklist")
	}
	contains(t, m, "Copied cover, artist, album, tracks on album, released and genre from Flow")

	m = selectTrack(t, m, "Current")
	m = keys(t, m, "v")
	got := trackNamed(t, hook.Library, "Current")
	flow := trackNamed(t, hook.Library, "Flow")
	if got.Artist != "Shawn James" || got.ArtistLatin != "Shawn J" || got.Album != "Shadows" || got.TrackTotal != 12 ||
		got.Released != "2015" || got.Genre != "Soul" || got.CoverSHA256 == "" || got.CoverSHA256 != flow.CoverSHA256 {
		t.Errorf("pasted %+v", got)
	}
	if got.Title != "Current" || got.TrackNumber == flow.TrackNumber {
		t.Errorf("the title or the number of the track was pasted: %q, %d", got.Title, got.TrackNumber)
	}
	if hook.manyCalls != 1 {
		t.Errorf("%d saves, want one for everything", hook.manyCalls)
	}
}

func TestSpaceUnticksAndOnlyWhatIsStillTickedIsCopied(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = withEdits(t, m, hook, map[string]app.Edit{"Flow": {Genre: ptr("Soul"), Released: ptr("2015")}})
	m = selectTrack(t, m, "Flow")
	m = keys(t, m, "c")
	// Flow has no cover, so the list starts on the artist: untick it and the
	// next three, which leaves the genre.
	m = keys(t, m, "space", "down", "space", "down", "space", "down", "space")
	contains(t, m, "[ ] Artist", "[ ] Album", "[ ] Tracks on album", "[ ] Released", "[x] Genre")
	m = keys(t, m, "enter")
	contains(t, m, "Copied the genre of Flow")

	m = selectTrack(t, m, "Current")
	m = keys(t, m, "v")
	if got := trackNamed(t, hook.Library, "Current"); got.Genre != "Soul" || got.Artist != "Phoria" || got.Released != "" {
		t.Errorf("pasted %+v", got)
	}
}

func TestWhatTheTrackHasNoneOfCannotBeTicked(t *testing.T) {
	m, _ := editingModel(t, "Flow") // no cover, genre or date
	m = keys(t, m, "c")
	contains(t, m, "[x] Artist", "Cover", "Genre", "—")
	lacks(t, m, "[x] Cover", "[ ] Cover", "[x] Genre", "[ ] Genre", "[x] Released", "[ ] Released")
}

func TestNothingTickedAsksToTickSomething(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "c")
	for i := 0; i < 4; i++ { // artist, album, number of tracks: all there is
		m = keys(t, m, "space", "down")
	}
	m = keys(t, m, "enter")
	if m.overlay == nil {
		t.Fatal("the checklist closed with nothing ticked")
	}
	contains(t, m, "Tick what to copy")
}

func TestEscLeavesWhatWasCopiedBefore(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = withEdits(t, m, hook, map[string]app.Edit{"Flow": {Genre: ptr("Soul")}})
	m = copyFrom(t, m, "Flow", fGenre)
	m = selectTrack(t, m, "Current")
	m = keys(t, m, "c", "esc", "v")
	if got := genreOf(t, hook.Library, "Current"); got != "Soul" {
		t.Errorf("the genre is %q: the cancelled copy replaced the clipboard", got)
	}
}

func TestCOnAnArtistOrAGroupSaysToSelectATrack(t *testing.T) {
	m, _ := toArtist(t)
	m = keys(t, m, "c")
	if m.overlay != nil {
		t.Error("a checklist for an artist")
	}
	contains(t, m, "Select a track to copy from")
}

// ---- the hints

func TestTheFooterOffersCopyOnATrackAndPasteOnceSomethingIsCopied(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = withEdits(t, m, hook, map[string]app.Edit{"Flow": {Genre: ptr("Soul")}})
	m = selectTrack(t, m, "Flow")
	contains(t, m, "c copy")
	lacks(t, m, "v paste")

	m = copyFrom(t, m, "Flow", fGenre)
	m = selectTrack(t, m, "Current")
	contains(t, m, "v paste genre") // the first to go when the footer is full is not this
	m = keys(t, m, "right")
	contains(t, m, "c copy", "v paste genre") // in the fields as well

	m = keys(t, m, "esc", "up", "up") // an artist: nothing to copy from, but to paste on
	if r, _ := m.selectedRow(); r.Kind != kindArtist {
		t.Fatalf("setup: on a %v", r.Kind)
	}
	contains(t, m, "v paste genre")
	lacks(t, m, "c copy")
}
