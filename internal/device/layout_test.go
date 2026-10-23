package device_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"music-manager/internal/device"
	"music-manager/internal/layout"
	"music-manager/internal/manifest"
)

func longTrack() manifest.Track {
	return manifest.Track{
		ID: "abc123", Status: manifest.Identified,
		Artist: "فيروز", Album: "روائع", Title: "منجاة أمير المؤمنين في الليلة الطويلة", TrackNumber: 1, TrackTotal: 12,
	}
}

// The player shows 24 characters of a name, the extension included, and moves
// a longer one along when it is hovered: a file name is cut so that all of it
// fits.
func TestTheWolfgangsFileNamesFitWhatItShowsWholeWithTheExtension(t *testing.T) {
	got := layout.PathFor(longTrack(), device.Wolfgang.Layout())
	name := got[strings.LastIndex(got, "/")+1:]
	if n := utf8.RuneCountInString(name); n != 24 {
		t.Errorf("%q: the file name is %d characters with .mp3, want 24", got, n)
	}
}

func TestAGenericFileNameKeepsTheWholeTitle(t *testing.T) {
	got := layout.PathFor(longTrack(), device.Generic.Layout())
	if want := "فيروز/روائع/01 - منجاة أمير المؤمنين في الليلة الطويلة.mp3"; got != want {
		t.Errorf("PathFor = %q, want %q", got, want)
	}
}
