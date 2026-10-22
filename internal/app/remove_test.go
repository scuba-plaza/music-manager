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
	// The rendered file is a stray now, and goes with --prune.
	ops := mustSync(t, a, app.SyncOptions{Prune: true})
	if len(ops) != 1 || ops[0] != (render.Op{Kind: render.Remove, Path: "Band/Drop.mp3"}) {
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
