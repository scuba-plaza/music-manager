package tui

import (
	"strings"
	"testing"

	"music-manager/internal/manifest"
)

// playlistLib is a library with three identified tracks and the playlist
// "Study", which lists the second and then the first.
func playlistLib(t *testing.T) (Library, []manifest.Track) {
	t.Helper()
	a := testApp(t)
	ts := seed(t, a,
		seedTrack{title: "Dream", artist: "Max Richter", album: "Sleep", n: 3, of: 8},
		seedTrack{title: "Flow", artist: "Shawn James", album: "Shadows", n: 7, of: 12},
		seedTrack{title: "Insane", artist: "Shawn James", album: "Shadows", n: 9, of: 12},
	)
	if err := a.CreatePlaylist("Study", ts[1].ID, ts[0].ID); err != nil {
		t.Fatal(err)
	}
	return NewLibrary(a), ts
}

func TestThePlaylistsOfTheLibraryAreInTheTree(t *testing.T) {
	lib, _ := playlistLib(t)
	m := newTestModel(t, lib, 120, 36)
	contains(t, m, "Playlists")
	notInTree(t, m, "Study") // closed

	m = keys(t, m, "right", "down", "right") // open the group, then the playlist
	contains(t, m, "Study", "Flow", "Dream")
	s := screen(m)
	if strings.Index(s, "Flow") > strings.Index(s, "Dream") {
		t.Errorf("Flow should come before Dream, as in the playlist:\n%s", s)
	}

	// The playlist is still selected, so the right pane describes it.
	contains(t, m, "2 tracks", "Playlists/Study.m3u")
}

// playlistTitles lists the titles of the tracks of the playlist, in order.
func playlistTitles(t *testing.T, lib Library, name string) []string {
	t.Helper()
	pls, err := lib.Playlists()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]string{}
	for _, tr := range libTracks(t, lib) {
		byID[tr.ID] = tr.Title
	}
	for _, p := range pls {
		if p.Name == name {
			var titles []string
			for _, id := range p.Tracks {
				titles = append(titles, byID[id])
			}
			return titles
		}
	}
	t.Fatalf("no playlist %q", name)
	return nil
}

func TestPAddsTheSelectedTrackToAnExistingPlaylist(t *testing.T) {
	lib, _ := playlistLib(t)
	m := newTestModel(t, lib, 120, 36)
	m = selectTrack(t, m, "Insane")

	m = keys(t, m, "p")
	contains(t, m, "Add to a playlist", "Study", "New playlist")
	m = keys(t, m, "enter") // the cursor starts on the first playlist

	if got := strings.Join(playlistTitles(t, lib, "Study"), ","); got != "Flow,Dream,Insane" {
		t.Errorf("Study = %s", got)
	}
	contains(t, m, "Added 1 track to Study")
	lacks(t, m, "Add to a playlist") // the dialog is closed
}

func TestPCanMakeANewPlaylistForTheMarkedTracks(t *testing.T) {
	lib, _ := playlistLib(t)
	m := newTestModel(t, lib, 120, 36)
	m = selectTrack(t, m, "Insane")
	m = keys(t, m, "space") // marks it and moves on
	m = selectTrack(t, m, "Dream")
	m = keys(t, m, "space")

	m = keys(t, m, "p", "down") // past Study, onto "New playlist…"
	m = keys(t, m, "enter")
	contains(t, m, "name of the playlist")
	m = typed(t, m, "Gym")
	m = keys(t, m, "enter")

	// The marked tracks, in library order (not the order they were marked in).
	if got := strings.Join(playlistTitles(t, lib, "Gym"), ","); got != "Dream,Insane" {
		t.Errorf("Gym = %s", got)
	}
	contains(t, m, "Made Gym with 2 tracks")
}

func TestTheNameOfANewPlaylistIsAskedForAtOnceWhenThereAreNone(t *testing.T) {
	a := testApp(t)
	seed(t, a, seedTrack{title: "Dream", artist: "Max Richter", album: "Sleep", n: 3, of: 8})
	lib := NewLibrary(a)
	m := newTestModel(t, lib, 120, 36)
	m = selectTrack(t, m, "Dream")

	m = keys(t, m, "p")
	contains(t, m, "name of the playlist")
	m = keys(t, m, "enter") // nothing typed
	contains(t, m, "needs a name")

	m = typed(t, m, "Study")
	m = keys(t, m, "enter")
	if got := strings.Join(playlistTitles(t, lib, "Study"), ","); got != "Dream" {
		t.Errorf("Study = %s", got)
	}
	contains(t, m, "Playlists") // the group appears
}

func TestAlbumsAndArtistsAreAddedWholeAndTracksAlreadyThereAreSkipped(t *testing.T) {
	lib, _ := playlistLib(t)
	m := newTestModel(t, lib, 120, 36)
	m = selectTrack(t, m, "Insane")
	m = keys(t, m, "left") // up to the album Shadows: Flow and Insane

	m = keys(t, m, "p", "enter") // into Study, which has Flow already
	if got := strings.Join(playlistTitles(t, lib, "Study"), ","); got != "Flow,Dream,Insane" {
		t.Errorf("Study = %s", got)
	}
	contains(t, m, "Added 1 track to Study (1 already there)")
}

func TestPNeedsSomethingToAdd(t *testing.T) {
	lib, _ := playlistLib(t)
	m := newTestModel(t, lib, 120, 36) // the cursor is on the Playlists group
	m = keys(t, m, "p")
	contains(t, m, "Select tracks to add")
	lacks(t, m, "Add to a playlist")
}

// openStudy opens the group and the playlist Study, with the cursor on the
// playlist.
func openStudy(t *testing.T, m Model) Model {
	t.Helper()
	return keys(t, m, "right", "down", "right")
}

func TestDTakesATrackOutOfAPlaylistButNotOutOfTheLibrary(t *testing.T) {
	lib, _ := playlistLib(t)
	m := openStudy(t, newTestModel(t, lib, 120, 36))
	m = keys(t, m, "down") // Flow, the first track of Study

	m = keys(t, m, "d")
	if got := strings.Join(playlistTitles(t, lib, "Study"), ","); got != "Dream" {
		t.Errorf("Study = %s, want only Dream", got)
	}
	if len(libTracks(t, lib)) != 3 {
		t.Error("a track was removed from the library")
	}
	contains(t, m, "Took “Flow” out of Study")
	lacks(t, m, "Are you sure")
}

func TestDInAPlaylistActsOnTheMarkedTracksThatAreInIt(t *testing.T) {
	lib, _ := playlistLib(t)
	m := newTestModel(t, lib, 120, 36)
	m = selectTrack(t, m, "Insane")
	m = keys(t, m, "space") // a marked track that is not in Study
	m = selectTrack(t, m, "Dream")
	m = keys(t, m, "space")
	m = selectTrack(t, m, "Flow")
	m = keys(t, m, "space")

	m = keys(t, m, "g", "right", "down", "right", "down", "d") // into Study, onto its first track
	if got := playlistTitles(t, lib, "Study"); len(got) != 0 {
		t.Errorf("Study = %v, want it empty: both its tracks were marked", got)
	}
	if len(libTracks(t, lib)) != 3 {
		t.Error("a track was removed from the library")
	}
}

func TestDOnAPlaylistDeletesItAfterAskingAndLeavesTheTracks(t *testing.T) {
	lib, _ := playlistLib(t)
	m := openStudy(t, newTestModel(t, lib, 120, 36))

	m = keys(t, m, "d")
	contains(t, m, "Delete the playlist “Study”?", "stay in the library")
	m = keys(t, m, "esc")
	if len(playlistTitles(t, lib, "Study")) != 2 {
		t.Error("cancelling deleted the playlist")
	}

	m = keys(t, m, "d", "y")
	if pls, _ := lib.Playlists(); len(pls) != 0 {
		t.Errorf("playlists = %+v, want none", pls)
	}
	if len(libTracks(t, lib)) != 3 {
		t.Error("deleting the playlist removed tracks from the library")
	}
	lacks(t, m, "Playlists") // no playlists left, so no group either
	contains(t, m, "Deleted the playlist Study")
}

func TestDOnTheGroupOfPlaylistsDoesNothing(t *testing.T) {
	lib, _ := playlistLib(t)
	m := newTestModel(t, lib, 120, 36) // the cursor is on the group
	m = keys(t, m, "d")
	contains(t, m, "Select a playlist")
	lacks(t, m, "Are you sure")
	if len(libTracks(t, lib)) != 3 {
		t.Error("a track was removed from the library")
	}
}

func TestUndoAndRedoCoverChangesToPlaylists(t *testing.T) {
	lib, _ := playlistLib(t)
	m := newTestModel(t, lib, 120, 36)
	m = selectTrack(t, m, "Insane")

	m = keys(t, m, "p", "enter") // Insane into Study
	if got := strings.Join(playlistTitles(t, lib, "Study"), ","); got != "Flow,Dream,Insane" {
		t.Fatalf("Study = %s", got)
	}
	m = keys(t, m, "u")
	if got := strings.Join(playlistTitles(t, lib, "Study"), ","); got != "Flow,Dream" {
		t.Errorf("after undo Study = %s", got)
	}
	contains(t, m, "Undid: adding to Study")
	m = keys(t, m, "ctrl+r")
	if got := strings.Join(playlistTitles(t, lib, "Study"), ","); got != "Flow,Dream,Insane" {
		t.Errorf("after redo Study = %s", got)
	}

	// A deleted playlist comes back whole, with its tracks in order.
	m = keys(t, m, "g", "right", "down", "d", "y")
	if pls, _ := lib.Playlists(); len(pls) != 0 {
		t.Fatalf("playlists = %+v, want none", pls)
	}
	m = keys(t, m, "u")
	if got := strings.Join(playlistTitles(t, lib, "Study"), ","); got != "Flow,Dream,Insane" {
		t.Errorf("after undoing the deletion Study = %s", got)
	}
	contains(t, m, "Playlists") // and it is in the tree again
}

func TestUndoingTheRemovalOfATrackPutsItBackInItsPlaylists(t *testing.T) {
	lib, _ := playlistLib(t)
	m := newTestModel(t, lib, 120, 36)
	m = selectTrack(t, m, "Flow")

	m = keys(t, m, "d", "y") // out of the library altogether
	if got := strings.Join(playlistTitles(t, lib, "Study"), ","); got != "Dream" {
		t.Fatalf("Study = %s, want Flow gone from it", got)
	}
	m = keys(t, m, "u")
	if got := strings.Join(playlistTitles(t, lib, "Study"), ","); got != "Flow,Dream" {
		t.Errorf("after undo Study = %s, want Flow back first", got)
	}
	m = keys(t, m, "ctrl+r")
	if got := strings.Join(playlistTitles(t, lib, "Study"), ","); got != "Dream" {
		t.Errorf("after redo Study = %s", got)
	}
}

func TestJAndKMoveATrackWithinAPlaylist(t *testing.T) {
	lib, _ := playlistLib(t)
	m := openStudy(t, newTestModel(t, lib, 120, 36))
	m = keys(t, m, "down", "down") // Dream, the second track

	m = keys(t, m, "K")
	if got := strings.Join(playlistTitles(t, lib, "Study"), ","); got != "Dream,Flow" {
		t.Errorf("after K Study = %s", got)
	}
	if r, _ := m.selectedRow(); r.Track.Title != "Dream" || r.Num != "01" {
		t.Errorf("the selection should follow Dream to the top: %+v", r)
	}
	m = keys(t, m, "K")
	contains(t, m, "first track already")

	m = keys(t, m, "J", "J")
	if got := strings.Join(playlistTitles(t, lib, "Study"), ","); got != "Flow,Dream" {
		t.Errorf("after J J Study = %s", got)
	}
	contains(t, m, "last track already")

	// It is one step to undo.
	m = keys(t, m, "u")
	if got := strings.Join(playlistTitles(t, lib, "Study"), ","); got != "Dream,Flow" {
		t.Errorf("after undo Study = %s", got)
	}
}

func TestERenamesThePlaylistUnderTheCursor(t *testing.T) {
	lib, _ := playlistLib(t)
	a := appOf(t, lib)
	a.CreatePlaylist("Gym")
	m := openStudy(t, newTestModel(t, lib, 120, 36))
	m = send(t, m, loadedMsgFor(t, lib)) // pick up Gym

	m = keys(t, m, "e")
	contains(t, m, "Rename the playlist", "Study") // the box starts with the old name
	m = clearField(t, m)
	m = typed(t, m, "gym")
	m = keys(t, m, "enter")
	contains(t, m, "Rename the playlist", "already a playlist") // refused, and still open

	m = clearField(t, m)
	m = typed(t, m, "Exams")
	m = keys(t, m, "enter")
	lacks(t, m, "Rename the playlist")
	pls, _ := lib.Playlists()
	if len(pls) != 2 || pls[0].Name != "Exams" || len(pls[0].Tracks) != 2 {
		t.Errorf("playlists = %+v", pls)
	}
	contains(t, m, "Exams")
}

func TestEOnTheGroupOfPlaylistsSaysWhatToSelect(t *testing.T) {
	lib, _ := playlistLib(t)
	m := keys(t, newTestModel(t, lib, 120, 36), "e")
	contains(t, m, "Select a playlist")
	lacks(t, m, "Rename")
}

func TestEditingATrackInAPlaylistKeepsYouInThePlaylist(t *testing.T) {
	lib, _ := playlistLib(t)
	m := openStudy(t, newTestModel(t, lib, 120, 36))
	m = keys(t, m, "down", "right", "enter") // Flow, into its fields, editing the title
	m = clearField(t, m)
	m = typed(t, m, "Flowing")
	m = keys(t, m, "enter")

	r, ok := m.selectedRow()
	if !ok || r.Track.Title != "Flowing" || r.Parent != playlistRowKey("Study") {
		t.Errorf("the selection left the playlist: %+v", r)
	}
	if got := strings.Join(playlistTitles(t, lib, "Study"), ","); got != "Flowing,Dream" {
		t.Errorf("Study = %s", got)
	}
}

func TestTheHintsTeachPlaylistsAndChangeInsideOne(t *testing.T) {
	lib, _ := playlistLib(t)
	m := newTestModel(t, lib, 120, 36)
	m = selectTrack(t, m, "Insane")
	contains(t, m, "p playlist")
	lacks(t, m, "take out")

	// Inside a playlist, d takes a track out of it and J K reorder.
	m = openStudy(t, newTestModel(t, lib, 120, 36))
	m = keys(t, m, "down")
	contains(t, m, "J K move", "d take out")

	m = keys(t, m, "?")
	contains(t, m, "add to a playlist", "take out of a playlist")
}

func TestOHasNothingToDoWithTheGroupOfPlaylists(t *testing.T) {
	a := testApp(t)
	ts := seed(t, a, seedTrack{title: "Dream", artist: "Max Richter", album: "Sleep", n: 3, of: 8})
	seedTagged(t, a, "old", "Old Song", "Old Artist", "Old Album", "1") // unidentified, with old tags
	a.CreatePlaylist("Study", ts[0].ID)
	m := newTestModel(t, NewLibrary(a), 120, 36) // the cursor is on the group of playlists
	m = keys(t, m, "o")
	contains(t, m, "Select a track that needs work")
}

func TestTheRenameBoxHasTheLatinNameOfThePlaylistToo(t *testing.T) {
	lib, _ := playlistLib(t)
	m := openStudy(t, newTestModel(t, lib, 120, 36))

	m = keys(t, m, "e")
	contains(t, m, "Rename the playlist", "Latin name")
	m = keys(t, m, "tab") // to the Latin name, which starts as the name reads in Latin
	m = clearField(t, m)
	m = typed(t, m, "Study hall")
	m = keys(t, m, "enter")
	lacks(t, m, "Rename the playlist")
	pls, _ := lib.Playlists()
	if pls[0].Name != "Study" || pls[0].NameLatin != "Study hall" || len(pls[0].Tracks) != 2 {
		t.Errorf("playlists = %+v", pls)
	}
	contains(t, m, "Latin name of Study")

	// An empty Latin name takes the one set by hand away.
	m = keys(t, m, "e", "tab")
	m = clearField(t, m)
	m = keys(t, m, "enter")
	if pls, _ = lib.Playlists(); pls[0].NameLatin != "" {
		t.Errorf("an empty Latin name should be forgotten: %+v", pls[0])
	}
	m = keys(t, m, "u") // and undo puts it back
	if pls, _ = lib.Playlists(); pls[0].NameLatin != "Study hall" {
		t.Errorf("undo gave %+v", pls[0])
	}
}

func TestRenamingAPlaylistLeavesItsLatinNameToBeWorkedOutUnlessAsked(t *testing.T) {
	lib, _ := playlistLib(t)
	m := openStudy(t, newTestModel(t, lib, 120, 36))
	m = keys(t, m, "e")
	m = clearField(t, m)
	m = typed(t, m, "Exams")
	m = keys(t, m, "enter")
	if pls, _ := lib.Playlists(); pls[0].Name != "Exams" || pls[0].NameLatin != "" {
		t.Errorf("the form shown, worked out from the old name, was kept as if set by hand: %+v", pls[0])
	}

	m = keys(t, m, "e")
	m = clearField(t, m)
	m = typed(t, m, "Finals")
	m = keys(t, m, "tab")
	m = clearField(t, m)
	m = typed(t, m, "Nihaiyat")
	m = keys(t, m, "enter")
	if pls, _ := lib.Playlists(); pls[0].Name != "Finals" || pls[0].NameLatin != "Nihaiyat" {
		t.Errorf("playlists = %+v", pls)
	}
}

func TestALatinNameThatWouldBeTheFileOfAnotherPlaylistIsRefusedAndNothingChanges(t *testing.T) {
	lib, _ := playlistLib(t)
	a := appOf(t, lib)
	a.CreatePlaylist("Gym")
	m := openStudy(t, newTestModel(t, lib, 120, 36))
	m = send(t, m, loadedMsgFor(t, lib))

	m = keys(t, m, "e")
	m = clearField(t, m)
	m = typed(t, m, "Exams")
	m = keys(t, m, "tab")
	m = clearField(t, m)
	m = typed(t, m, "gym")
	m = keys(t, m, "enter")
	contains(t, m, "Rename the playlist", "same file")
	if pls, _ := lib.Playlists(); pls[0].Name != "Study" {
		t.Errorf("the playlist was renamed although its Latin name was refused: %+v", pls)
	}
}
