package app_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/mp3"
	"music-manager/internal/render"
	"music-manager/internal/testmp3"
)

func TestIngestKeepsEmbeddedArtworkAndSyncWritesIt(t *testing.T) {
	a := openLib(t)
	f := filepath.Join(t.TempDir(), "x.mp3")
	writeFile(t, f, testmp3.WithCover(testmp3.Silent(10), "image/jpeg", testmp3.JPEG))
	if _, err := a.Ingest([]string{f}); err != nil {
		t.Fatal(err)
	}
	tr := tracks(t, a)[0]
	if tr.CoverSHA256 == "" {
		t.Fatal("embedded cover was dropped at ingest")
	}

	identify(t, a, tr.ID, "Song", "Band", "Album", 1, 1)
	mustSync(t, a, app.SyncOptions{})

	tags := mp3.ReadTagSet(readFile(t, a.Root(), "Band/Album/01 - Song.mp3"))
	if !bytes.Equal(tags.Cover, testmp3.JPEG) || tags.CoverMIME != "image/jpeg" {
		t.Errorf("embedded cover = %q (%s)", tags.Cover, tags.CoverMIME)
	}
	if got := readFile(t, a.Root(), "Band/Album/cover.jpg"); !bytes.Equal(got, testmp3.JPEG) {
		t.Errorf("cover.jpg = %q", got)
	}
}

func TestAlbumFolderCoverComesFromTheOldestTrackThatHasOne(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30)
	ts := tracks(t, a)
	for _, tr := range ts {
		identify(t, a, tr.ID, "Song "+tr.ID, "Band", "Album", 1, 3)
	}
	if err := a.SetCover(ts[1].ID, testmp3.PNG); err != nil {
		t.Fatal(err)
	}
	if err := a.SetCover(ts[2].ID, testmp3.JPEG); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, app.SyncOptions{})
	if got := readFile(t, a.Root(), "Band/Album/cover.png"); !bytes.Equal(got, testmp3.PNG) {
		t.Errorf("cover.png = %q", got)
	}
	if ops := mustSync(t, a, app.SyncOptions{}); len(ops) != 0 {
		t.Errorf("second sync wants %v", ops)
	}
}

func TestSinglesAndInboxTracksGetNoCoverFile(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Single", "Band", "", 0, 0)
	a.SetCover(ts[0].ID, testmp3.JPEG)
	a.SetCover(ts[1].ID, testmp3.JPEG)
	ops := mustSync(t, a, app.SyncOptions{})
	for _, op := range ops {
		if op.Kind == render.Create && filepath.Ext(op.Path) != ".mp3" {
			t.Errorf("unexpected image file %s", op.Path)
		}
	}
	if got := mp3.ReadTagSet(readFile(t, a.Root(), "Band/Single.mp3")).Cover; !bytes.Equal(got, testmp3.JPEG) {
		t.Error("a single should still embed its cover")
	}
}

func TestChangingAndRemovingACover(t *testing.T) {
	a := libWithTracks(t, 10)
	tr := tracks(t, a)[0]
	identify(t, a, tr.ID, "Song", "Band", "Album", 1, 1)
	a.SetCover(tr.ID, testmp3.JPEG)
	mustSync(t, a, app.SyncOptions{})

	a.SetCover(tr.ID, testmp3.JPEG2)
	mustSync(t, a, app.SyncOptions{})
	if got := readFile(t, a.Root(), "Band/Album/cover.jpg"); !bytes.Equal(got, testmp3.JPEG2) {
		t.Errorf("cover.jpg = %q after replacing", got)
	}

	if err := a.SetCover(tr.ID, nil); err != nil {
		t.Fatal(err)
	}
	ops := mustSync(t, a, app.SyncOptions{Prune: true})
	removed := 0
	for _, op := range ops {
		if op.Kind == render.Remove && op.Path == "Band/Album/cover.jpg" {
			removed++
		}
	}
	if removed != 1 || len(mp3.ReadTagSet(readFile(t, a.Root(), "Band/Album/01 - Song.mp3")).Cover) != 0 {
		t.Errorf("cover not removed everywhere: ops %v", ops)
	}
}

func TestSetCoverRejectsNonImagesAndUnknownTracks(t *testing.T) {
	a := libWithTracks(t, 10)
	id := tracks(t, a)[0].ID
	if err := a.SetCover(id, []byte("just some text")); err == nil {
		t.Error("accepted a text file as a cover")
	}
	if err := a.SetCover("nope", testmp3.JPEG); err == nil {
		t.Error("accepted an unknown track")
	}
	if tracks(t, a)[0].CoverSHA256 != "" {
		t.Error("a rejected cover was recorded")
	}
}
