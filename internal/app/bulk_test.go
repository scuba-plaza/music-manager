package app_test

import (
	"os"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

func TestRestoreBringsBackAnEditedTrack(t *testing.T) {
	a := libWithTracks(t, 10)
	tr := tracks(t, a)[0]
	identify(t, a, tr.ID, "Before", "Band", "Album", 1, 2)
	before := tracks(t, a)[0]

	identify(t, a, tr.ID, "After", "Other", "", 0, 0)
	if err := a.Restore(before); err != nil {
		t.Fatal(err)
	}
	got := tracks(t, a)[0]
	if got.OriginalTitle != "Before" || got.OriginalArtist != "Band" || got.OriginalAlbum != "Album" || got.TrackNumber != 1 || !got.UpdatedAt.Equal(before.UpdatedAt) {
		t.Errorf("restored = %+v, want %+v", got, before)
	}
}

func TestRestoreBringsBackARemovedTrackUntilSyncCollectsItsAudio(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	ts := tracks(t, a)
	removed, err := a.Remove(ts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Restore(removed); err != nil {
		t.Fatal(err)
	}
	if got := tracks(t, a); len(got) != 2 {
		t.Fatalf("%d tracks after restore, want 2", len(got))
	}

	// Once a sync has deleted the unused audio, restoring would create a broken record.
	a.Remove(ts[0].ID)
	mustSync(t, a, app.SyncOptions{})
	if _, err := os.Stat(a.AudioPath(ts[0].AudioSHA256)); !os.IsNotExist(err) {
		t.Fatal("setup: the audio should be collected")
	}
	if err := a.Restore(removed); err == nil {
		t.Error("restored a track whose audio is gone")
	}
	if len(tracks(t, a)) != 1 {
		t.Error("a failed restore changed the library")
	}
}

func TestEditManyAppliesEverythingOrNothing(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30)
	ts := tracks(t, a)
	got, err := a.EditMany(map[string]app.Edit{
		ts[0].ID: {Title: strp("One"), Artist: strp("Band"), Album: strp("Album"), TrackNumber: ptr(1)},
		ts[1].ID: {Title: strp("Two"), Artist: strp("Band"), Album: strp("Album"), TrackNumber: ptr(2)},
	})
	if err != nil || len(got) != 2 || got[ts[1].ID].OriginalTitle != "Two" {
		t.Fatalf("%+v, %v", got, err)
	}
	if now := tracks(t, a); now[0].OriginalTitle != "One" || now[1].OriginalTitle != "Two" || now[2].Status != manifest.Unidentified {
		t.Errorf("tracks = %+v", now)
	}

	before := tracks(t, a)
	for name, edits := range map[string]map[string]app.Edit{
		"unknown id":   {ts[0].ID: {Title: strp("X")}, "nope": {Title: strp("Y")}},
		"invalid trim": {ts[0].ID: {Title: strp("X")}, ts[1].ID: {TrimStart: ptr(time.Hour)}},
	} {
		if _, err := a.EditMany(edits); err == nil {
			t.Errorf("%s: accepted", name)
		}
		after := tracks(t, a)
		if after[0].OriginalTitle != before[0].OriginalTitle || after[1].TrimStart != before[1].TrimStart {
			t.Errorf("%s: a rejected batch changed the library", name)
		}
	}
}

func TestRenumberNumbersTracksInTheGivenOrder(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30)
	ts := tracks(t, a)
	for _, tr := range ts {
		identify(t, a, tr.ID, "Song "+tr.ID, "Band", "Album", 9, 99)
	}
	got, err := a.Renumber([]string{ts[2].ID, ts[0].ID, ts[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{ts[2].ID, ts[0].ID, ts[1].ID} {
		if got[id].TrackNumber != i+1 || got[id].TrackTotal != 3 {
			t.Errorf("%s = %d of %d, want %d of 3", id, got[id].TrackNumber, got[id].TrackTotal, i+1)
		}
	}
	if _, err := a.Renumber([]string{ts[0].ID, "nope"}); err == nil {
		t.Error("unknown id accepted")
	}
	if tracks(t, a)[0].TrackNumber != 2 {
		t.Error("a rejected renumber changed the library")
	}
}
