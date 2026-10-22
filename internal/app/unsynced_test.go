package app_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/render"
	"music-manager/internal/testmp3"
)

func unsynced(t *testing.T, a *app.App) (int, bool) {
	t.Helper()
	n, known, err := a.Unsynced()
	if err != nil {
		t.Fatal(err)
	}
	return n, known
}

func TestUnsyncedIsUnknownUntilTheFirstSync(t *testing.T) {
	a := libWithTracks(t, 10)
	if n, known := unsynced(t, a); known || n != 0 {
		t.Errorf("= %d, known %v; want unknown before any sync", n, known)
	}
	mustSync(t, a, app.SyncOptions{})
	if n, known := unsynced(t, a); !known || n != 0 {
		t.Errorf("after sync = %d, known %v", n, known)
	}
}

func TestUnsyncedCountsTracksWhoseOutputWouldChange(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30)
	ts := tracks(t, a)
	mustSync(t, a, app.SyncOptions{})

	identify(t, a, ts[0].ID, "Renamed", "Band", "", 0, 0)
	if n, _ := unsynced(t, a); n != 1 {
		t.Errorf("after one edit = %d, want 1", n)
	}
	identify(t, a, ts[1].ID, "Other", "Band", "", 0, 0)
	if n, _ := unsynced(t, a); n != 2 {
		t.Errorf("after two edits = %d, want 2", n)
	}
	a.Remove(ts[2].ID)
	if n, _ := unsynced(t, a); n != 3 {
		t.Errorf("a removed track should count too: %d, want 3", n)
	}
	mustSync(t, a, app.SyncOptions{})
	if n, known := unsynced(t, a); n != 0 || !known {
		t.Errorf("after syncing = %d, known %v", n, known)
	}
}

func TestUnsyncedNoticesTheSharedAlbumCoverChanging(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	ts := tracks(t, a)
	for i, tr := range ts {
		identify(t, a, tr.ID, "Song", "Band", "Album", i+1, 2)
	}
	mustSync(t, a, app.SyncOptions{})
	if err := a.SetCover(ts[0].ID, testmp3.JPEG); err != nil {
		t.Fatal(err)
	}
	// The first track's own cover changed, and the album folder's cover.jpg with it.
	if n, _ := unsynced(t, a); n != 2 {
		t.Errorf("= %d, want both tracks of the album", n)
	}
}

func TestOnlyASyncIntoTheLibraryFolderCountsAsSynced(t *testing.T) {
	a := libWithTracks(t, 10)
	mustSync(t, a, app.SyncOptions{})
	identify(t, a, tracks(t, a)[0].ID, "New", "Band", "", 0, 0)
	mustSync(t, a, app.SyncOptions{Target: filepath.Join(t.TempDir(), "player")})
	if n, _ := unsynced(t, a); n != 1 {
		t.Errorf("= %d; rendering to a player must not mark the library folder as up to date", n)
	}
}

func TestRecordSyncedAcceptsTheCurrentStateAsInSync(t *testing.T) {
	a := libWithTracks(t, 10)
	if err := a.RecordSynced(); err != nil {
		t.Fatal(err)
	}
	if n, known := unsynced(t, a); n != 0 || !known {
		t.Errorf("= %d, known %v", n, known)
	}
	identify(t, a, tracks(t, a)[0].ID, "Changed", "Band", "", 0, 0)
	if n, _ := unsynced(t, a); n != 1 {
		t.Errorf("= %d, want 1", n)
	}
}

func TestPlanWithPruneMatchesAFreshPlan(t *testing.T) {
	a := libWithTracks(t, 10)
	identify(t, a, tracks(t, a)[0].ID, "Song", "Band", "", 0, 0)
	writeFile(t, filepath.Join(a.Root(), "Old", "x.mp3"), testmp3.Silent(3))

	plain := mustPlan(t, a, app.SyncOptions{})
	pruned := mustPlan(t, a, app.SyncOptions{Prune: true})
	if got := plain.WithPrune(true); !reflect.DeepEqual(got.Ops, pruned.Ops) {
		t.Errorf("WithPrune(true) = %v, want %v", got.Ops, pruned.Ops)
	}
	if got := pruned.WithPrune(false); !reflect.DeepEqual(got.Ops, plain.Ops) {
		t.Errorf("WithPrune(false) = %v, want %v", got.Ops, plain.Ops)
	}
	var kinds []render.Kind
	for _, op := range plain.WithPrune(true).Ops {
		kinds = append(kinds, op.Kind)
	}
	if !reflect.DeepEqual(kinds, []render.Kind{render.Create, render.Remove}) {
		t.Errorf("kinds = %v", kinds)
	}
}

func TestSyncReportsProgress(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30)
	var planned [][2]int
	plan, err := a.PlanSync(app.SyncOptions{OnProgress: func(done, total int) { planned = append(planned, [2]int{done, total}) }})
	if err != nil {
		t.Fatal(err)
	}
	if len(planned) != 3 || planned[2] != [2]int{3, 3} {
		t.Errorf("planning progress = %v, want 1/3, 2/3, 3/3", planned)
	}
	var applied [][2]int
	if err := plan.ApplyWith(func(done, total int) { applied = append(applied, [2]int{done, total}) }); err != nil {
		t.Fatal(err)
	}
	if n := len(plan.Ops); len(applied) != n || applied[n-1] != [2]int{n, n} {
		t.Errorf("apply progress = %v for %d ops", applied, n)
	}
}

func TestUnsyncedCountsPlaylistsToo(t *testing.T) {
	a, ts := syncedPlaylistLib(t)
	mustSync(t, a, app.SyncOptions{})
	if n, _ := unsynced(t, a); n != 0 {
		t.Fatalf("after syncing = %d", n)
	}

	a.CreatePlaylist("Gym")
	if n, _ := unsynced(t, a); n != 1 {
		t.Errorf("a new playlist = %d, want 1", n)
	}
	a.MoveInPlaylist("Study", ts[2].ID, 0)
	if n, _ := unsynced(t, a); n != 2 {
		t.Errorf("and a reordered one = %d, want 2", n)
	}
	a.DeletePlaylist("Gym") // never synced: nothing to undo on the card
	if n, _ := unsynced(t, a); n != 1 {
		t.Errorf("after deleting the new one = %d, want 1", n)
	}
	mustSync(t, a, app.SyncOptions{})
	if n, _ := unsynced(t, a); n != 0 {
		t.Errorf("after syncing = %d", n)
	}
	a.DeletePlaylist("Study")
	if n, _ := unsynced(t, a); n != 1 {
		t.Errorf("a deleted playlist's file is still on the card: %d, want 1", n)
	}
}
