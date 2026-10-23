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
	if album.Artist != "A. H. Hafez" || album.Album != "Abd El Halim" {
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
		if tr.ID == ts[0].ID && (tr.Artist != "Abdel Halim Hafez" || tr.Album != "Abd El Halim Hafez") {
			t.Errorf("the library's names were shortened: %q / %q", tr.Artist, tr.Album)
		}
		if tr.ID == ts[1].ID && tr.Album != "" {
			t.Errorf("a single got the album %q in the library", tr.Album)
		}
	}
}

// A plan says what its device will not do well, so that it can be read before
// anything is written.
func TestPlanSyncCarriesTheWarningsOfItsDevice(t *testing.T) {
	a := libWithTracks(t, 40)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Song", "Band", "", 0, 0)
	if _, err := a.Edit(ts[0].ID, app.Edit{Genre: ptr("Alternative Rock")}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		dev  device.Target
		want int
	}{{device.Generic, 0}, {device.Wolfgang, 1}} {
		plan, err := a.PlanSync(app.SyncOptions{Target: t.TempDir(), Device: c.dev})
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Warnings) != c.want {
			t.Errorf("%s: warnings = %+v, want %d", c.dev.Name(), plan.Warnings, c.want)
		}
		if c.want == 1 && plan.Warnings[0].TrackID != ts[0].ID {
			t.Errorf("the warning is for %q, want %q", plan.Warnings[0].TrackID, ts[0].ID)
		}
	}
}
