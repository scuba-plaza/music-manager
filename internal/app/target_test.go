package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/device"
	"music-manager/internal/layout"
	"music-manager/internal/manifest"
	"music-manager/internal/mp3"
	"music-manager/internal/testmp3"
)

// fakeTarget does every part of the writing its own way, so that a test can
// tell which parts of a sync ask the target.
type fakeTarget struct{ device.Target }

func (fakeTarget) Name() string { return "fake" }

func (fakeTarget) Layout() layout.Rules { return layout.Rules{Fold: strings.ToUpper} }

func (fakeTarget) Tags(t manifest.Track) mp3.TagSet {
	return mp3.TagSet{Title: "tag of the fake", Artist: "the fake artist"}
}

func (fakeTarget) Cover([]byte) ([]byte, string, error) {
	return []byte("fake cover"), "image/x-fake", nil
}

func (fakeTarget) SingleCovers() bool { return false }

func (fakeTarget) PlaylistFile(name string) string { return "Playlists/fake.m3u" }

func (fakeTarget) Playlist(p manifest.Playlist, tracks map[string]manifest.Track, paths map[string]string) (string, []byte) {
	return "Playlists/fake.m3u", []byte("fake playlist of " + p.Name)
}

func TestASyncWritesWhatTheGivenTargetSays(t *testing.T) {
	a := libWithTracks(t, 40, 41)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Song", "Band", "Album", 1, 1)
	identify(t, a, ts[1].ID, "Single", "Band", "", 0, 0)
	for _, tr := range ts {
		if err := a.SetCover(tr.ID, testmp3.JPEG); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.CreatePlaylist("Mine", ts[0].ID); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "fake")

	mustSync(t, a, app.SyncOptions{Target: dest, Device: fakeTarget{device.Generic}})

	file := readFile(t, dest, "BAND/ALBUM/01 - SONG.mp3")
	if got := mp3.ReadTagSet(file); got.Title != "tag of the fake" || got.Artist != "the fake artist" || string(got.Cover) != "fake cover" || got.CoverMIME != "image/x-fake" {
		t.Errorf("the file's tag is not the target's: %+v", got)
	}
	if got := readFile(t, dest, "BAND/ALBUM/cover.jpg"); string(got) != "fake cover" {
		t.Errorf("the album's cover file = %q", got)
	}
	if got := readFile(t, dest, "Playlists/fake.m3u"); string(got) != "fake playlist of Mine" {
		t.Errorf("the playlist file = %q", got)
	}
	readFile(t, dest, "BAND/SINGLE.mp3")
	if _, err := os.Stat(filepath.Join(dest, "BAND", "SINGLE.jpg")); !os.IsNotExist(err) {
		t.Error("a single got a cover file although the target does not want one")
	}
}
