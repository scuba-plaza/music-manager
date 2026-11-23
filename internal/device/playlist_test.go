package device_test

import (
	"testing"
	"time"

	"music-manager/internal/device"
	"music-manager/internal/manifest"
)

func playlistFixture() (manifest.Playlist, map[string]manifest.Track, map[string]string) {
	tracks := map[string]manifest.Track{
		"a": {ID: "a", Status: manifest.Identified, Artist: "فيروز", Title: "وطني", SourceLength: 156 * time.Second},
		"b": {ID: "b", Status: manifest.Identified, Artist: "Nils Frahm", Title: "Cold", SourceLength: 100 * time.Second, TrimStart: 10 * time.Second},
		"c": {ID: "c", Status: manifest.Unidentified},
	}
	paths := map[string]string{
		"a": "فيروز/Hits/03 - وطني.mp3",
		"b": "Nils Frahm/Cold.mp3",
		"c": "_Inbox/c - 01 x.mp3",
	}
	return manifest.Playlist{Name: "دراسة", Tracks: []string{"a", "b", "gone", "c"}}, tracks, paths
}

func TestAGenericPlaylistIsAnM3U8WithPathsRelativeToIt(t *testing.T) {
	p, tracks, paths := playlistFixture()
	name, data := device.Generic.Playlist(p, tracks, paths)
	if want := "Playlists/دراسة.m3u8"; name != want {
		t.Errorf("name = %q, want %q", name, want)
	}
	want := "#EXTM3U\n" +
		"#EXTINF:156,فيروز - وطني\n../فيروز/Hits/03 - وطني.mp3\n" +
		"#EXTINF:90,Nils Frahm - Cold\n../Nils Frahm/Cold.mp3\n" +
		"#EXTINF:-1,c - 01 x\n../_Inbox/c - 01 x.mp3\n"
	if string(data) != want {
		t.Errorf("playlist =\n%q\nwant\n%q", data, want)
	}
}

func TestTheWolfgangsPlaylistIsAnM3UWithPathsFromTheRoot(t *testing.T) {
	p, tracks, paths := playlistFixture()
	paths["a"] = "Firuz/Hits/03 - Wtni.mp3"
	name, data := wolfgang.Playlist(p, tracks, paths)
	if want := "Playlists/Drasa.m3u"; name != want {
		t.Errorf("name = %q, want %q", name, want)
	}
	want := "\\Firuz\\Hits\\03 - Wtni.mp3\r\n\\Nils Frahm\\Cold.mp3\r\n\\_Inbox\\c - 01 x.mp3\r\n"
	if string(data) != want {
		t.Errorf("playlist =\n%q\nwant\n%q", data, want)
	}
}

// A playlist is called by its true name on a target that can show it, and by its
// Latin name, the one set by hand or else the one worked out, on one that cannot.

func TestAPlaylistFileIsNamedByTheTrueNameOrTheLatinOne(t *testing.T) {
	for _, c := range []struct {
		name, latin string
		generic     string
		wolfgang    string
	}{
		{"Study", "", "Playlists/Study.m3u8", "Playlists/Study.m3u"},
		{"دراسة", "Study", "Playlists/دراسة.m3u8", "Playlists/Study.m3u"},
		{"فيروز", "", "Playlists/فيروز.m3u8", "Playlists/Firuz.m3u"}, // worked out
	} {
		p := manifest.Playlist{Name: c.name, NameLatin: c.latin}
		if got := device.Generic.PlaylistFile(p); got != c.generic {
			t.Errorf("generic file of %+v = %q, want %q", p, got, c.generic)
		}
		if got := wolfgang.PlaylistFile(p); got != c.wolfgang {
			t.Errorf("Wolfgang file of %+v = %q, want %q", p, got, c.wolfgang)
		}
		if got, _ := wolfgang.Playlist(p, nil, nil); got != c.wolfgang {
			t.Errorf("Wolfgang writes %+v to %q, want %q", p, got, c.wolfgang)
		}
	}
}
