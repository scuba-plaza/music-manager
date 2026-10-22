package app_test

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/render"
)

// playlists reads the playlists back from disk, through a fresh App, so that
// what is checked is what was saved.
func playlists(t *testing.T, a *app.App) []manifest.Playlist {
	t.Helper()
	b, err := app.Open(a.Root())
	if err != nil {
		t.Fatal(err)
	}
	m, err := b.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	return m.Playlists
}

func idsOf(ts ...manifest.Track) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.ID
	}
	return out
}

func TestAPlaylistKeepsItsTracksInOrderAcrossReloads(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30)
	ts := tracks(t, a)

	if err := a.CreatePlaylist("Study"); err != nil {
		t.Fatal(err)
	}
	added, err := a.AddToPlaylist("Study", idsOf(ts[2], ts[0]))
	if err != nil || added != 2 {
		t.Fatalf("AddToPlaylist = %d, %v", added, err)
	}

	want := []manifest.Playlist{{Name: "Study", Tracks: idsOf(ts[2], ts[0])}}
	if got := playlists(t, a); !reflect.DeepEqual(got, want) {
		t.Errorf("playlists = %+v, want %+v", got, want)
	}
}

func TestAddingAnUnknownPlaylistOrTrackFailsAndChangesNothing(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	ts := tracks(t, a)
	if err := a.CreatePlaylist("Study"); err != nil {
		t.Fatal(err)
	}

	if _, err := a.AddToPlaylist("Nope", idsOf(ts[0])); err == nil {
		t.Error("adding to a playlist that does not exist succeeded")
	}
	if n, err := a.AddToPlaylist("Study", []string{ts[0].ID, "zzzzzz"}); err == nil || n != 0 {
		t.Errorf("adding an unknown track = %d, %v; want an error and 0", n, err)
	}
	if got := playlists(t, a); len(got[0].Tracks) != 0 {
		t.Errorf("a failed add left %v in the playlist", got[0].Tracks)
	}
}

func TestATrackIsInAPlaylistOnlyOnce(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30)
	ts := tracks(t, a)
	a.CreatePlaylist("Study")
	a.AddToPlaylist("Study", idsOf(ts[0]))

	// ts[0] is already there; ts[1] is named twice in the same request.
	added, err := a.AddToPlaylist("Study", []string{ts[0].ID, ts[1].ID, ts[1].ID, ts[2].ID})
	if err != nil || added != 2 {
		t.Fatalf("AddToPlaylist = %d, %v; want 2 added", added, err)
	}
	if got := playlists(t, a)[0].Tracks; !reflect.DeepEqual(got, idsOf(ts[0], ts[1], ts[2])) {
		t.Errorf("tracks = %v", got)
	}
}

func TestPlaylistNamesMustBeUsableAndDistinct(t *testing.T) {
	a := openLib(t)
	if err := a.CreatePlaylist("Study"); err != nil {
		t.Fatal(err)
	}
	// Names that are empty, or that would be the same file as an existing
	// playlist (the card ignores case).
	for _, bad := range []string{"", "   ", "study", "STUDY "} {
		if err := a.CreatePlaylist(bad); err == nil {
			t.Errorf("CreatePlaylist(%q) succeeded", bad)
		}
	}
	// "Study?" is written as Study-.m3u: a different file, so it is fine, and
	// then "Study-" is the one that clashes.
	if err := a.CreatePlaylist("Study?"); err != nil {
		t.Errorf("CreatePlaylist(Study?) = %v", err)
	}
	if err := a.CreatePlaylist("Study-"); err == nil {
		t.Error("CreatePlaylist(Study-) succeeded, but it is the same file as Study?")
	}
	if got := playlists(t, a); len(got) != 2 {
		t.Errorf("playlists = %+v, want only Study and Study?", got)
	}
}

func TestRemovingTracksFromAPlaylist(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30)
	ts := tracks(t, a)
	a.CreatePlaylist("Study")
	a.AddToPlaylist("Study", idsOf(ts[0], ts[1], ts[2]))

	if err := a.RemoveFromPlaylist("Study", idsOf(ts[1])); err != nil {
		t.Fatal(err)
	}
	if got := playlists(t, a)[0].Tracks; !reflect.DeepEqual(got, idsOf(ts[0], ts[2])) {
		t.Errorf("tracks = %v, want the first and third", got)
	}

	// ts[1] is no longer in it: the whole request fails, ts[0] stays.
	if err := a.RemoveFromPlaylist("Study", idsOf(ts[0], ts[1])); err == nil {
		t.Error("removing a track that is not in the playlist succeeded")
	}
	if got := playlists(t, a)[0].Tracks; !reflect.DeepEqual(got, idsOf(ts[0], ts[2])) {
		t.Errorf("a failed removal changed the playlist: %v", got)
	}
	if err := a.RemoveFromPlaylist("Nope", idsOf(ts[0])); err == nil {
		t.Error("removing from an unknown playlist succeeded")
	}
	// The tracks themselves are untouched.
	if len(tracks(t, a)) != 3 {
		t.Error("removing from a playlist removed a track from the library")
	}
}

func TestMovingATrackWithinAPlaylist(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30, 40)
	ts := tracks(t, a)
	a.CreatePlaylist("Study")
	a.AddToPlaylist("Study", idsOf(ts[0], ts[1], ts[2]))

	steps := []struct {
		id   string
		to   int
		want []string
	}{
		{ts[2].ID, 0, idsOf(ts[2], ts[0], ts[1])}, // to the front
		{ts[2].ID, 2, idsOf(ts[0], ts[1], ts[2])}, // to the very end
		{ts[0].ID, 1, idsOf(ts[1], ts[0], ts[2])}, // into the middle
		{ts[0].ID, 1, idsOf(ts[1], ts[0], ts[2])}, // where it already is
	}
	for _, s := range steps {
		if err := a.MoveInPlaylist("Study", s.id, s.to); err != nil {
			t.Fatalf("move %s to %d: %v", s.id, s.to, err)
		}
		if got := playlists(t, a)[0].Tracks; !reflect.DeepEqual(got, s.want) {
			t.Fatalf("after moving %s to %d: %v, want %v", s.id, s.to, got, s.want)
		}
	}

	for _, bad := range []struct {
		name, id string
		to       int
	}{
		{"Study", ts[0].ID, -1}, {"Study", ts[0].ID, 3}, // outside 0..2
		{"Study", ts[3].ID, 0}, // a track that is not in it
		{"Nope", ts[0].ID, 0},
	} {
		if err := a.MoveInPlaylist(bad.name, bad.id, bad.to); err == nil {
			t.Errorf("MoveInPlaylist(%q, %s, %d) succeeded", bad.name, bad.id, bad.to)
		}
	}
	if got := playlists(t, a)[0].Tracks; !reflect.DeepEqual(got, idsOf(ts[1], ts[0], ts[2])) {
		t.Errorf("failed moves changed the playlist: %v", got)
	}
}

func TestRenamingAPlaylistKeepsItsTracksAndPlace(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	ts := tracks(t, a)
	for _, n := range []string{"First", "Study", "Last"} {
		a.CreatePlaylist(n)
	}
	a.AddToPlaylist("Study", idsOf(ts[1], ts[0]))

	if err := a.RenamePlaylist("Study", "Exams"); err != nil {
		t.Fatal(err)
	}
	got := playlists(t, a)
	if names := []string{got[0].Name, got[1].Name, got[2].Name}; !reflect.DeepEqual(names, []string{"First", "Exams", "Last"}) {
		t.Errorf("names = %v", names)
	}
	if !reflect.DeepEqual(got[1].Tracks, idsOf(ts[1], ts[0])) {
		t.Errorf("tracks = %v", got[1].Tracks)
	}

	// Only the case may change for the same file; another playlist's name may not be taken.
	if err := a.RenamePlaylist("Exams", "EXAMS"); err != nil {
		t.Errorf("changing only the case failed: %v", err)
	}
	for _, bad := range [][2]string{{"EXAMS", "first"}, {"EXAMS", " "}, {"Nope", "Other"}} {
		if err := a.RenamePlaylist(bad[0], bad[1]); err == nil {
			t.Errorf("RenamePlaylist(%q, %q) succeeded", bad[0], bad[1])
		}
	}
	if got := playlists(t, a); got[1].Name != "EXAMS" || len(got) != 3 {
		t.Errorf("failed renames changed the playlists: %+v", got)
	}
}

func TestDeletingAPlaylistLeavesTheTracks(t *testing.T) {
	a := libWithTracks(t, 10)
	ts := tracks(t, a)
	a.CreatePlaylist("Study")
	a.CreatePlaylist("Gym")
	a.AddToPlaylist("Study", idsOf(ts[0]))

	if err := a.DeletePlaylist("Study"); err != nil {
		t.Fatal(err)
	}
	if got := playlists(t, a); len(got) != 1 || got[0].Name != "Gym" {
		t.Errorf("playlists = %+v, want only Gym", got)
	}
	if len(tracks(t, a)) != 1 {
		t.Error("deleting a playlist removed a track")
	}
	if err := a.DeletePlaylist("Study"); err == nil {
		t.Error("deleting a playlist twice succeeded")
	}
}

func TestRemovingATrackFromTheLibraryTakesItOutOfEveryPlaylist(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30, 40)
	ts := tracks(t, a)
	for _, n := range []string{"Study", "Gym"} {
		a.CreatePlaylist(n)
		a.AddToPlaylist(n, idsOf(ts[0], ts[1], ts[2], ts[3]))
	}

	if _, err := a.Remove(ts[1].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RemoveMany(idsOf(ts[0], ts[3])); err != nil {
		t.Fatal(err)
	}
	for _, p := range playlists(t, a) {
		if !reflect.DeepEqual(p.Tracks, idsOf(ts[2])) {
			t.Errorf("%s = %v, want only the third track", p.Name, p.Tracks)
		}
	}
}

// syncedPlaylistLib is a library with three tracks (two identified, one not)
// and the playlist "Study" listing all three, in an order of its own.
func syncedPlaylistLib(t *testing.T) (*app.App, []manifest.Track) {
	t.Helper()
	a := libWithTracks(t, 10, 20, 30)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Atlek Neranak", "Ali Al Attar", "Atlek Neranak", 1, 4)
	identify(t, a, ts[1].ID, "Dream", "Max Richter", "", 0, 0)
	ts = tracks(t, a)
	if err := a.CreatePlaylist("Study"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddToPlaylist("Study", idsOf(ts[1], ts[0], ts[2])); err != nil {
		t.Fatal(err)
	}
	return a, ts
}

func TestSyncWritesAPlaylistTheWolfangPlayerCanRead(t *testing.T) {
	a, ts := syncedPlaylistLib(t)

	ops := mustSync(t, a, app.SyncOptions{})
	if len(ops) != 4 { // three tracks and the playlist
		t.Fatalf("ops = %v, want 4 creates", ops)
	}

	// Found by experiment on the player (WMP01): paths from the card's root,
	// with backslashes, one per line, Windows line endings, plain UTF-8.
	want := "\\Max Richter\\Dream.mp3\r\n" +
		"\\Ali Al Attar\\Atlek Neranak\\01 - Atlek Neranak.mp3\r\n" +
		"\\_Inbox\\" + ts[2].ID + " - c.mp3\r\n"
	if got := string(readFile(t, a.Root(), "Playlists/Study.m3u")); got != want {
		t.Errorf("Study.m3u =\n%q\nwant\n%q", got, want)
	}
	if ops := mustSync(t, a, app.SyncOptions{}); len(ops) != 0 {
		t.Errorf("a second sync wants %v, want nothing", ops)
	}
}

func TestPlaylistFilesFollowTheLibrary(t *testing.T) {
	a, ts := syncedPlaylistLib(t)
	mustSync(t, a, app.SyncOptions{})

	// A renamed track moves, so the playlist that names it changes too.
	title := "Dreams"
	if _, err := a.Edit(ts[1].ID, app.Edit{Title: &title}); err != nil {
		t.Fatal(err)
	}
	ops := mustSync(t, a, app.SyncOptions{})
	var updated []string
	for _, op := range ops {
		if op.Kind == render.Update {
			updated = append(updated, op.Path)
		}
	}
	if !reflect.DeepEqual(updated, []string{"Playlists/Study.m3u"}) {
		t.Errorf("ops = %v, want the playlist updated", ops)
	}
	if got := string(readFile(t, a.Root(), "Playlists/Study.m3u")); !strings.HasPrefix(got, "\\Max Richter\\Dreams.mp3\r\n") {
		t.Errorf("Study.m3u = %q", got)
	}

	// A deleted playlist leaves its file behind as a stray; --prune removes it.
	if err := a.DeletePlaylist("Study"); err != nil {
		t.Fatal(err)
	}
	if ops := mustSync(t, a, app.SyncOptions{}); !slices.Contains(ops, render.Op{Kind: render.Stray, Path: "Playlists/Study.m3u"}) {
		t.Errorf("ops = %v, want the playlist file reported as a stray", ops)
	}
	mustSync(t, a, app.SyncOptions{Prune: true})
	if _, err := os.Stat(filepath.Join(a.Root(), "Playlists")); !os.IsNotExist(err) {
		t.Errorf("the playlist file or its empty folder is still there: %v", err)
	}
}

func TestCreatingAPlaylistWithTracksIsAllOrNothing(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	ts := tracks(t, a)

	if err := a.CreatePlaylist("Study", ts[1].ID, ts[0].ID); err != nil {
		t.Fatal(err)
	}
	if got := playlists(t, a); !reflect.DeepEqual(got, []manifest.Playlist{{Name: "Study", Tracks: idsOf(ts[1], ts[0])}}) {
		t.Errorf("playlists = %+v", got)
	}

	if err := a.CreatePlaylist("Gym", ts[0].ID, "zzzzzz"); err == nil {
		t.Error("creating a playlist with an unknown track succeeded")
	}
	if got := playlists(t, a); len(got) != 1 {
		t.Errorf("a failed create left a playlist behind: %+v", got)
	}
}

func TestReplacingThePlaylistsPutsBackAnEarlierState(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	ts := tracks(t, a)
	a.CreatePlaylist("Study", ts[1].ID, ts[0].ID)
	a.CreatePlaylist("Gym")
	before := playlists(t, a)

	a.DeletePlaylist("Study")
	a.AddToPlaylist("Gym", idsOf(ts[0]))
	if err := a.ReplacePlaylists(before); err != nil {
		t.Fatal(err)
	}
	if got := playlists(t, a); !reflect.DeepEqual(got, before) {
		t.Errorf("playlists = %+v, want %+v", got, before)
	}

	// A snapshot that does not make sense is refused, and nothing changes.
	for name, bad := range map[string][]manifest.Playlist{
		"unknown track": {{Name: "X", Tracks: []string{"zzzzzz"}}},
		"empty name":    {{Name: " ", Tracks: []string{}}},
		"same file":     {{Name: "A?", Tracks: []string{}}, {Name: "a-", Tracks: []string{}}},
		"a track twice": {{Name: "X", Tracks: idsOf(ts[0], ts[0])}},
	} {
		if err := a.ReplacePlaylists(bad); err == nil {
			t.Errorf("%s: ReplacePlaylists succeeded", name)
		}
	}
	if got := playlists(t, a); !reflect.DeepEqual(got, before) {
		t.Errorf("refused snapshots changed the playlists: %+v", got)
	}

	// An empty snapshot clears them all.
	if err := a.ReplacePlaylists(nil); err != nil {
		t.Fatal(err)
	}
	if got := playlists(t, a); len(got) != 0 {
		t.Errorf("playlists = %+v, want none", got)
	}
}
