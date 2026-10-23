package device_test

import (
	"strings"
	"testing"

	"music-manager/internal/device"
	"music-manager/internal/manifest"
)

func track(id, artist, album, title string) manifest.Track {
	return manifest.Track{ID: id, Status: manifest.Identified, Artist: artist, Album: album, Title: title}
}

// warned is the messages of the warnings for one track.
func warned(ws []device.Warning, id string) []string {
	var out []string
	for _, w := range ws {
		if w.TrackID == id {
			out = append(out, w.Message)
		}
	}
	return out
}

func hasMessage(msgs []string, part string) bool {
	for _, m := range msgs {
		if strings.Contains(m, part) {
			return true
		}
	}
	return false
}

func TestTheWolfgangWarnsAboutAGenreItCannotShow(t *testing.T) {
	long, short := track("long", "A", "", "S"), track("short", "A", "", "T")
	long.Genre, short.Genre = "Alternative Rock", "Rock"
	ws := device.Wolfgang.Diagnose([]manifest.Track{long, short})
	if got := warned(ws, "long"); !hasMessage(got, "genre") || !hasMessage(got, "12") {
		t.Errorf("long genre: %q", got)
	}
	if got := warned(ws, "short"); len(got) != 0 {
		t.Errorf("a genre that fits drew %q", got)
	}
}

func TestTheWolfgangWarnsAboutANameWithNoLatinForm(t *testing.T) {
	emoji := track("emoji", "🎵🎶", "", "Song")
	plain := track("plain", "Max Richter", "", "Dream")
	ws := device.Wolfgang.Diagnose([]manifest.Track{emoji, plain})
	if got := warned(ws, "emoji"); !hasMessage(got, "artist") || !hasMessage(got, "Unknown") {
		t.Errorf("artist with no Latin form: %q", got)
	}
	if got := warned(ws, "plain"); len(got) != 0 {
		t.Errorf("a plain name drew %q", got)
	}
}

func TestTheWolfgangWarnsWhenCuttingMakesTwoFileNamesAlike(t *testing.T) {
	a := track("aaaaaa", "Artist", "", "A Very Long Song Title Indeed Yes")
	b := track("bbbbbb", "Artist", "", "A Very Long Song Title Indeed No")
	other := track("cccccc", "Artist", "", "A Different Song")
	ws := device.Wolfgang.Diagnose([]manifest.Track{a, b, other})
	for _, id := range []string{"aaaaaa", "bbbbbb"} {
		if got := warned(ws, id); !hasMessage(got, "cut") {
			t.Errorf("%s: %q", id, got)
		}
	}
	if got := warned(ws, "cccccc"); len(got) != 0 {
		t.Errorf("a name that is not cut like another drew %q", got)
	}
}

func TestTheWolfgangDoesNotBlameTheCutForTwoTracksThatAreTheSame(t *testing.T) {
	a, b := track("aaaaaa", "Artist", "", "Same"), track("bbbbbb", "Artist", "", "Same")
	if ws := device.Wolfgang.Diagnose([]manifest.Track{a, b}); len(ws) != 0 {
		t.Errorf("warnings = %+v", ws)
	}
}

func TestTheWolfgangWarnsAboutAnArtistThatLooksLikeSeveral(t *testing.T) {
	duo := track("duo", "Fairuz & Ziad Rahbani", "", "Song")
	ws := device.Wolfgang.Diagnose([]manifest.Track{duo})
	if got := warned(ws, "duo"); !hasMessage(got, "several") {
		t.Errorf("%q", got)
	}
}

func TestAGenericTargetHasNothingToWarnAbout(t *testing.T) {
	long := track("long", "🎵 & Fairuz", "", "A Very Long Song Title Indeed Yes")
	long.Genre = "Alternative Rock"
	if ws := device.Generic.Diagnose([]manifest.Track{long}); len(ws) != 0 {
		t.Errorf("warnings = %+v", ws)
	}
}
