package app_test

import (
	"os"
	"path/filepath"
	"strings"
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
	if err := plan.ApplyWith(nil); err != nil {
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

func TestSyncKeepsBlobsSharedAcrossTracksAndTargets(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	ts := tracks(t, a)
	a.SetCover(ts[0].ID, testmp3.JPEG)
	a.SetCover(ts[1].ID, testmp3.JPEG) // the same image: one blob, two users
	a.Remove(ts[0].ID)
	target := filepath.Join(t.TempDir(), "player")
	plan := mustPlan(t, a, app.SyncOptions{Target: target})
	if err := plan.ApplyWith(nil); err != nil {
		t.Fatal(err)
	}
	if plan.Collected != 1 { // only ts[0]'s audio
		t.Errorf("Collected = %d, want 1", plan.Collected)
	}
	if _, err := os.Stat(a.AudioPath(ts[1].AudioSHA256)); err != nil {
		t.Error(err)
	}
}

func TestRemoveManyReturnsTheRemovedTracksInTheOrderGiven(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30)
	ts := tracks(t, a)

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

func TestRemoveManyRefusesAnUnknownOrRepeatedIDAndRemovesNothing(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	ts := tracks(t, a)
	for _, c := range []struct {
		name, named string
		ids         []string
	}{
		{"an unknown id", "nope", []string{ts[0].ID, "nope"}},
		{"a repeated id", ts[0].ID, []string{ts[0].ID, ts[1].ID, ts[0].ID}},
	} {
		got, err := a.RemoveMany(c.ids)
		if err == nil || !strings.Contains(err.Error(), c.named) {
			t.Errorf("%s: err = %v, want one that names %s", c.name, err, c.named)
		}
		if len(got) != 0 {
			t.Errorf("%s: RemoveMany returned %v together with its error", c.name, got)
		}
		if left := tracks(t, a); len(left) != 2 {
			t.Errorf("%s: %d tracks left, want both", c.name, len(left))
		}
	}
}
