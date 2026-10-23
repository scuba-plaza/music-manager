package app_test

import (
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/device"
	"music-manager/internal/mp3"
)

// The player's index keeps 12 characters of an artist and of an album, so what
// mm writes into the tags has to say its piece within them.

func TestSyncWritesTagsTheWholePlayerCanShow(t *testing.T) {
	a := libWithTracks(t, 40, 41, 42)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Bahlam Bek", "Abdel Halim Hafez", "Abd El Halim Hafez", 1, 2)
	identify(t, a, ts[1].ID, "Dream of Me and You", "Max Richter", "", 0, 0)
	identify(t, a, ts[2].ID, "Dream", "Max Richter", "", 0, 0)
	mustSync(t, a, app.SyncOptions{Device: device.Wolfgang})

	album := mp3.ReadTagSet(readFile(t, a.Root(), "Abdel Halim Hafez/Abd El Halim Hafez/01 - Bahlam Bek.mp3"))
	if album.Artist != "Abdel Halim" || album.Album != "Abd El Halim" {
		t.Errorf("album track: artist %q, album %q", album.Artist, album.Album)
	}

	// A single has no album, which the player shows as "Unknown Album": its
	// title stands in.
	single := mp3.ReadTagSet(readFile(t, a.Root(), "Max Richter/Dream of Me and You.mp3"))
	if single.Album != "Dream of Me" {
		t.Errorf("a single's album = %q, want its title shortened", single.Album)
	}
	if short := mp3.ReadTagSet(readFile(t, a.Root(), "Max Richter/Dream.mp3")); short.Album != "Dream" {
		t.Errorf("a single with a short title: album %q", short.Album)
	}

	// The library keeps the whole names.
	for _, tr := range tracks(t, a) {
		if tr.ID == ts[0].ID && (tr.OriginalArtist != "Abdel Halim Hafez" || tr.OriginalAlbum != "Abd El Halim Hafez") {
			t.Errorf("the library's names were shortened: %q / %q", tr.OriginalArtist, tr.OriginalAlbum)
		}
		if tr.ID == ts[1].ID && tr.OriginalAlbum != "" {
			t.Errorf("a single got the album %q in the library", tr.OriginalAlbum)
		}
	}
}
