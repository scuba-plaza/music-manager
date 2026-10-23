package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/render"
	"music-manager/internal/testmp3"
)

func TestRemoveDeletesTheRecordAndSyncCollectsItsBlobs(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Keep", "Band", "", 0, 0)
	identify(t, a, ts[1].ID, "Drop", "Band", "", 0, 0)
	a.SetCover(ts[1].ID, testmp3.JPEG)
	mustSync(t, a, app.SyncOptions{})

	removed, err := a.Remove(ts[1].ID)
	if err != nil || removed.ID != ts[1].ID {
		t.Fatalf("Remove = %+v, %v", removed, err)
	}
	if left := tracks(t, a); len(left) != 1 || left[0].ID != ts[0].ID {
		t.Errorf("tracks = %+v", left)
	}
	// The blobs stay until sync, so nothing is lost by a mistaken rm before it.
	for _, p := range []string{a.AudioPath(ts[1].AudioSHA256)} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("blob deleted too early: %v", err)
		}
	}

	plan := mustPlan(t, a, app.SyncOptions{})
	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	if plan.Collected != 2 { // the audio and the cover
		t.Errorf("Collected = %d, want 2", plan.Collected)
	}
	if _, err := os.Stat(a.AudioPath(ts[1].AudioSHA256)); !os.IsNotExist(err) {
		t.Errorf("unreferenced audio survived sync: %v", err)
	}
	if _, err := os.Stat(a.AudioPath(ts[0].AudioSHA256)); err != nil {
		t.Errorf("referenced audio was collected: %v", err)
	}
	// The rendered file and the cover beside it are strays now, and go with --prune.
	ops := mustSync(t, a, app.SyncOptions{Prune: true})
	if len(ops) != 2 || ops[0] != (render.Op{Kind: render.Remove, Path: "Band/Drop.jpg"}) || ops[1] != (render.Op{Kind: render.Remove, Path: "Band/Drop.mp3"}) {
		t.Errorf("ops = %v", ops)
	}
}

func TestRemoveUnknownID(t *testing.T) {
	a := libWithTracks(t, 10)
	if _, err := a.Remove("nope"); err == nil {
		t.Error("removing an unknown id succeeded")
	}
}

func TestSyncKeepsBlobsSharedAcrossTracksAndTargets(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	ts := tracks(t, a)
	a.SetCover(ts[0].ID, testmp3.JPEG)
	a.SetCover(ts[1].ID, testmp3.JPEG) // the same image: one blob, two users
	a.Remove(ts[0].ID)
	target := filepath.Join(t.TempDir(), "player")
	plan := mustPlan(t, a, app.SyncOptions{Target: target})
	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	if plan.Collected != 1 { // only ts[0]'s audio
		t.Errorf("Collected = %d, want 1", plan.Collected)
	}
	if _, err := os.Stat(a.AudioPath(ts[1].AudioSHA256)); err != nil {
		t.Error(err)
	}
}

func TestRemoveManyRemovesAllOrNone(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30)
	ts := tracks(t, a)

	if _, err := a.RemoveMany([]string{ts[0].ID, "nope"}); err == nil {
		t.Error("an unknown id was accepted")
	}
	if len(tracks(t, a)) != 3 {
		t.Fatal("a failed RemoveMany removed something")
	}
	if _, err := a.RemoveMany([]string{ts[0].ID, ts[0].ID}); err == nil {
		t.Error("an id listed twice was accepted")
	}

	got, err := a.RemoveMany([]string{ts[2].ID, ts[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != ts[2].ID || got[1].ID != ts[0].ID {
		t.Errorf("returned %v, want the removed tracks in the order asked", got)
	}
	if left := tracks(t, a); len(left) != 1 || left[0].ID != ts[1].ID {
		t.Errorf("left = %+v", left)
	}
	if _, err := os.Stat(a.AudioPath(ts[0].AudioSHA256)); err != nil {
		t.Errorf("the audio must stay until a sync collects it: %v", err)
	}
	if got, err := a.RemoveMany(nil); err != nil || len(got) != 0 {
		t.Errorf("removing nothing = %v, %v", got, err)
	}
}
