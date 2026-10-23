package app_test

import (
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/mp3"
)

// The player's index keeps 12 characters of an artist and of an album, so what
// mm writes into the tags has to say its piece within them.

func TestPlayerNameKeepsAShortNameAndShortensALongOne(t *testing.T) {
	cases := []struct{ name, full, want string }{
		{"short", "Fairouz", "Fairouz"},
		{"exactly the room", "Songs (Live)", "Songs (Live)"},
		{"after a whole word", "Abdel Halim Hafez", "Abdel Halim"},
		{"a leading The goes when it must", "The Wanted Stars", "Wanted Stars"},
		{"a leading The stays when all fits", "The Cure", "The Cure"},
		{"a parenthesis goes first", "Yawman ma (Remix)", "Yawman ma"},
		{"one long word is cut in it", "Supercalifragilistic", "Supercalifra"},
		{"a first word that is too short to keep", "Julia Boutros", "Julia Boutro"},
		{"nothing is left dangling", "Fairouz and, Co Band", "Fairouz"},
		{"nor a word that needs the next", "Caught a Black Rabbit", "Caught"},
		{"a connector in the middle stays", "Songs of Bay Al Hikma", "Songs of Bay"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := app.PlayerName(c.full, ""); got != c.want {
				t.Errorf("PlayerName(%q) = %q, want %q", c.full, got, c.want)
			}
		})
	}
}

func TestPlayerNameLetsAnOverrideWin(t *testing.T) {
	if got := app.PlayerName("Abdel Halim Hafez", "A. H. Hafez"); got != "A. H. Hafez" {
		t.Errorf("got %q", got)
	}
	if got := app.PlayerName("Fairouz", "Fairuz Official Channel"); got != "Fairuz Official Channel" {
		t.Errorf("an override is the user's word, even when the player will cut it: got %q", got)
	}
	if got := app.PlayerName("Abdel Halim Hafez", "  "); got != "Abdel Halim" {
		t.Errorf("a blank override is none: got %q", got)
	}
}

func TestSyncWritesTagsTheWholePlayerCanShow(t *testing.T) {
	a := libWithTracks(t, 40, 41, 42)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Bahlam Bek", "Abdel Halim Hafez", "Abd El Halim Hafez", 1, 2)
	identify(t, a, ts[1].ID, "Dream of Me and You", "Max Richter", "", 0, 0)
	identify(t, a, ts[2].ID, "Dream", "Max Richter", "", 0, 0)
	mustSync(t, a, app.SyncOptions{})

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
		if tr.ID == ts[0].ID && (tr.Artist != "Abdel Halim Hafez" || tr.Album != "Abd El Halim Hafez") {
			t.Errorf("the library's names were shortened: %q / %q", tr.Artist, tr.Album)
		}
		if tr.ID == ts[1].ID && tr.Album != "" {
			t.Errorf("a single got the album %q in the library", tr.Album)
		}
	}
}

func TestAPlayerNameSetOnATrackReachesTheTagsAndTheLibraryKeepsTheRest(t *testing.T) {
	a := libWithTracks(t, 40, 41)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Bahlam Bek", "Abdel Halim Hafez", "Abd El Halim Hafez", 1, 2)
	identify(t, a, ts[1].ID, "Dream of Me and You", "Max Richter", "", 0, 0)
	for id, e := range map[string]app.Edit{
		ts[0].ID: {PlayerArtist: ptr("A. H. Hafez"), PlayerAlbum: ptr("Hafez Hits")},
		ts[1].ID: {PlayerAlbum: ptr("Singles")}, // a single may be given an album to show
	} {
		if _, err := a.Edit(id, e); err != nil {
			t.Fatal(err)
		}
	}
	mustSync(t, a, app.SyncOptions{})

	got := mp3.ReadTagSet(readFile(t, a.Root(), "Abdel Halim Hafez/Abd El Halim Hafez/01 - Bahlam Bek.mp3"))
	if got.Artist != "A. H. Hafez" || got.Album != "Hafez Hits" {
		t.Errorf("tags: artist %q, album %q", got.Artist, got.Album)
	}
	if got := mp3.ReadTagSet(readFile(t, a.Root(), "Max Richter/Dream of Me and You.mp3")); got.Album != "Singles" || got.Artist != "Max Richter" {
		t.Errorf("single: artist %q, album %q", got.Artist, got.Album)
	}
	for _, tr := range tracks(t, a) {
		if tr.ID == ts[0].ID && (tr.Artist != "Abdel Halim Hafez" || tr.Album != "Abd El Halim Hafez" || tr.PlayerArtist != "A. H. Hafez") {
			t.Errorf("record: %q / %q / %q", tr.Artist, tr.Album, tr.PlayerArtist)
		}
	}

	// An empty value takes the override away again.
	if _, err := a.Edit(ts[0].ID, app.Edit{PlayerArtist: ptr(""), PlayerAlbum: ptr("")}); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, app.SyncOptions{})
	got = mp3.ReadTagSet(readFile(t, a.Root(), "Abdel Halim Hafez/Abd El Halim Hafez/01 - Bahlam Bek.mp3"))
	if got.Artist != "Abdel Halim" || got.Album != "Abd El Halim" {
		t.Errorf("after clearing: artist %q, album %q", got.Artist, got.Album)
	}
}

func TestAPlayerNameMustBeOnOneLine(t *testing.T) {
	a := libWithTracks(t, 40)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Song", "Band", "", 0, 0)
	if _, err := a.Edit(ts[0].ID, app.Edit{PlayerArtist: ptr("Two\nLines")}); err == nil {
		t.Error("accepted a player name with a line break")
	}
}
