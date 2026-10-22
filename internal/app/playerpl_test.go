package app_test

import (
	"os"
	"reflect"
	"testing"

	"music-manager/internal/app"
)

func TestReadingAPlaylistFileOfThePlayer(t *testing.T) {
	// Real bytes from a Wolfang WMP01 (USERPL1.PL), with the song names
	// replaced: three entries and then the record that ends the list. The
	// player keeps only the first 26 characters of a name.
	data, err := os.ReadFile("testdata/userpl-sample.pl")
	if err != nil {
		t.Fatal(err)
	}
	got, err := app.ReadPlayerPlaylist(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Alpha Song.mp3", "Beta Song.mp3", "Gamma Song With A Very Lo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("names = %q, want %q", got, want)
	}

	if _, err := app.ReadPlayerPlaylist([]byte("not a playlist")); err == nil {
		t.Error("garbage was read as a playlist")
	}
	wrong := append([]byte("MUSIC\x00\x00\x00"), data[8:]...) // another of the player's files
	if _, err := app.ReadPlayerPlaylist(wrong); err == nil {
		t.Error("another kind of file was read as a playlist")
	}
	if names, err := app.ReadPlayerPlaylist(data[:512]); err != nil || len(names) != 0 {
		t.Errorf("an empty playlist = %q, %v", names, err)
	}
}

func TestMatchingTheOldFileNamesToTracks(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30, 40, 50, 60, 70, 80, 90)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Atlek Neranak", "Julia Boutros", "Yawman Ma", 6, 10)
	identify(t, a, ts[1].ID, "Atlek Neranak", "Ali Al Attar", "Atlek Neranak", 1, 4)
	identify(t, a, ts[2].ID, "Moqawem", "Julia Boutros", "Yawman Ma", 7, 10)
	identify(t, a, ts[3].ID, "Moqawem", "Ali Al Attar", "Atlek Neranak", 4, 4)
	identify(t, a, ts[4].ID, "Yakhluf Al Qaida Qada", "Kataib", "Speeches", 6, 6)
	identify(t, a, ts[5].ID, "The Anthem of the Arab Socialist Party", "Various", "Anthems", 1, 1)
	identify(t, a, ts[6].ID, "وين الملايين (Live)", "Julia Boutros", "", 0, 0)
	identify(t, a, ts[7].ID, "Twin", "Band", "One", 1, 2)
	identify(t, a, ts[8].ID, "Twin", "Band", "Two", 1, 2)
	ts = tracks(t, a)

	matches, err := a.MatchPlayerNames([]string{
		"06 Atlek neranak.mp3", // the number tells the two Atlek Neranaks apart
		"07 Moqawem.mp3",
		"Yakhluf.mp3",                // the title has grown since
		"The Anthem of the Arab Soc", // cut at 26 characters by the player
		"01 وين الملايين (Live).mp3", // matches the original title
		"Missing Song.mp3",
		"Twin.mp3", // nothing tells the two apart
	})
	if err != nil {
		t.Fatal(err)
	}

	type outcome struct {
		id      string // the track it is, or ""
		partial bool
		cands   int
	}
	want := []outcome{
		{ts[0].ID, false, 0}, {ts[2].ID, false, 0}, {ts[4].ID, true, 0}, {ts[5].ID, true, 0}, {ts[6].ID, false, 0},
		{"", false, 0}, {"", false, 2},
	}
	if len(matches) != len(want) {
		t.Fatalf("%d matches, want %d", len(matches), len(want))
	}
	for i, w := range want {
		m := matches[i]
		got := outcome{partial: m.Partial, cands: len(m.Candidates)}
		if m.Track != nil {
			got.id = m.Track.ID
		}
		if got != w {
			t.Errorf("%q: got %+v, want %+v", m.Name, got, w)
		}
	}
}
