package device_test

import (
	"os"
	"reflect"
	"testing"

	"music-manager/internal/device"
	"music-manager/internal/manifest"
)

func TestReadingAPlaylistFileOfThePlayer(t *testing.T) {
	// Real bytes from a Wolfgang WMP01 (USERPL1.PL), with the song names
	// replaced: three entries and then the record that ends the list. The
	// player keeps only the first 26 characters of a name.
	data, err := os.ReadFile("testdata/userpl-sample.pl")
	if err != nil {
		t.Fatal(err)
	}
	got, err := device.ReadWolfgangPlaylist(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Alpha Song.mp3", "Beta Song.mp3", "Gamma Song With A Very Lo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("names = %q, want %q", got, want)
	}

	if _, err := device.ReadWolfgangPlaylist([]byte("not a playlist")); err == nil {
		t.Error("garbage was read as a playlist")
	}
	wrong := append([]byte("MUSIC\x00\x00\x00"), data[8:]...) // another of the player's files
	if _, err := device.ReadWolfgangPlaylist(wrong); err == nil {
		t.Error("another kind of file was read as a playlist")
	}
	if names, err := device.ReadWolfgangPlaylist(data[:512]); err != nil || len(names) != 0 {
		t.Errorf("an empty playlist = %q, %v", names, err)
	}
}

func TestMatchingTheOldFileNamesToTracks(t *testing.T) {
	lib := func(id, title, artist, album string, n, total int) manifest.Track {
		return manifest.Track{ID: id, Status: manifest.Identified, Title: title, Artist: artist, Album: album, TrackNumber: n, TrackTotal: total}
	}
	ts := []manifest.Track{
		lib("t0", "Atlek Neranak", "Julia Boutros", "Yawman Ma", 6, 10),
		lib("t1", "Atlek Neranak", "Ali Al Attar", "Atlek Neranak", 1, 4),
		lib("t2", "Moqawem", "Julia Boutros", "Yawman Ma", 7, 10),
		lib("t3", "Moqawem", "Ali Al Attar", "Atlek Neranak", 4, 4),
		lib("t4", "Yakhluf Al Qaida Qada", "Kataib", "Speeches", 6, 6),
		lib("t5", "The Anthem of the Arab Socialist Party", "Various", "Anthems", 1, 1),
		lib("t6", "وين الملايين (Live)", "Julia Boutros", "", 0, 0),
		lib("t7", "Twin", "Band", "One", 1, 2),
		lib("t8", "Twin", "Band", "Two", 1, 2),
	}

	matches := device.MatchWolfgangNames([]string{
		"06 Atlek neranak.mp3", // the number tells the two Atlek Neranaks apart
		"07 Moqawem.mp3",
		"Yakhluf.mp3",                // the title has grown since
		"The Anthem of the Arab Soc", // cut at 26 characters by the player
		"01 وين الملايين (Live).mp3", // matches the original title
		"Missing Song.mp3",
		"Twin.mp3", // nothing tells the two apart
	}, ts)

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
