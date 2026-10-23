package app_test

import (
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/device"
	"music-manager/internal/mp3"
)

// The library holds the names as they are; what a place needs of them is
// worked out when writing to it.

func TestAPlainSyncWritesTheTrueNames(t *testing.T) {
	a := libWithTracks(t, 40, 41)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "وطني", "فيروز", "Hits", 3, 12)
	identify(t, a, ts[1].ID, "Dream", "Max Richter", "", 0, 0)
	dest := t.TempDir()

	mustSync(t, a, app.SyncOptions{Target: dest})

	got := mp3.ReadTagSet(readFile(t, dest, "فيروز/Hits/03 - وطني.mp3"))
	if got.Title != "وطني" || got.Artist != "فيروز" || got.Album != "Hits" || got.Track != 3 || got.Total != 12 {
		t.Errorf("tags = %+v, want the true names", got)
	}
	for k := range got.Extra {
		if strings.HasPrefix(k, "ORIGINAL_") {
			t.Errorf("the tag keeps the original names in %s, but they are the names", k)
		}
	}
	if single := mp3.ReadTagSet(readFile(t, dest, "Max Richter/Dream.mp3")); single.Album != "" {
		t.Errorf("a single got the album %q", single.Album)
	}
}

func TestASyncForTheWolfgangKeepsTheTitleTrueAndFoldsTheRest(t *testing.T) {
	a := libWithTracks(t, 40, 41)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "وطني", "فيروز", "Hits", 3, 12)
	identify(t, a, ts[1].ID, "Dream", "Max Richter", "", 0, 0)
	dest := t.TempDir()

	mustSync(t, a, app.SyncOptions{Target: dest, Device: device.Wolfgang})

	got := mp3.ReadTagSet(readFile(t, dest, "Firuz/Hits/03 - Wtni.mp3"))
	if got.Title != "وطني" {
		t.Errorf("title = %q: the player shows only the file name, so the tag can say the true title", got.Title)
	}
	if got.Artist != "Firuz" || got.Album != "Hits" {
		t.Errorf("artist %q, album %q: the player shows these, in Latin letters", got.Artist, got.Album)
	}
	for k := range got.Extra {
		if strings.HasPrefix(k, "ORIGINAL_") {
			t.Errorf("the tag has %s", k)
		}
	}
	if single := mp3.ReadTagSet(readFile(t, dest, "Max Richter/Dream.mp3")); single.Album != "Dream" {
		t.Errorf("a single's album = %q, want its title", single.Album)
	}
}

func TestSearchFindsATrackByTheLatinFormOfItsTrueName(t *testing.T) {
	a := libWithTracks(t, 40, 41)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "وطني", "فيروز", "Hits", 3, 12)
	identify(t, a, ts[1].ID, "Dream", "Max Richter", "", 0, 0)
	m, err := a.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"firuz", "wtni", "فيروز", "richter"} {
		if got := m.Search(q); len(got) != 1 {
			t.Errorf("Search(%q) found %d tracks, want 1", q, len(got))
		}
	}
}
