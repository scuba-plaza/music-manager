package device_test

import (
	"testing"

	"music-manager/internal/device"
	"music-manager/internal/manifest"
)

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
		if got := device.Wolfgang.PlaylistFile(p); got != c.wolfgang {
			t.Errorf("Wolfgang file of %+v = %q, want %q", p, got, c.wolfgang)
		}
		if got, _ := device.Wolfgang.Playlist(p, nil, nil); got != c.wolfgang {
			t.Errorf("Wolfgang writes %+v to %q, want %q", p, got, c.wolfgang)
		}
	}
}
