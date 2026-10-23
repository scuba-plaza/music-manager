package device

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"music-manager/internal/artist"
	"music-manager/internal/layout"
	"music-manager/internal/manifest"
)

// Warning is something a target cannot do well for a track: the sync goes
// ahead, but the track will not look on the device the way it does here.
type Warning struct {
	TrackID string
	Message string
}

// Generic writes everything as it is, so it has nothing to warn about.
func (generic) Diagnose([]manifest.Track) []Warning { return nil }

// Diagnose says what the player will make a mess of: a genre longer than it
// shows, a name that has nothing left once it is in Latin letters, two file
// names that are the same only because they are cut, and an artist name that
// looks like several (its 12-character form is made from the whole of it).
func (w wolfgang) Diagnose(tracks []manifest.Track) []Warning {
	var out []Warning
	warn := func(t manifest.Track, format string, args ...any) {
		out = append(out, Warning{t.ID, fmt.Sprintf(format, args...)})
	}
	cut, uncut := w.Layout(), w.Layout()
	uncut.StemMax = 0
	whole := layout.Rules{}
	byName := map[string][]manifest.Track{}
	for _, t := range tracks {
		if t.Status != manifest.Identified {
			continue
		}
		if genre := t.LatinGenre(); utf8.RuneCountInString(genre) > PlayerFieldWidth {
			warn(t, "the genre %q is %d characters, the player shows %d", genre, utf8.RuneCountInString(genre), PlayerFieldWidth)
		}
		for _, name := range []struct{ kind, value, latin string }{{"artist", t.Artist, t.LatinArtist()}, {"album", t.Album, t.LatinAlbum()}, {"title", t.Title, t.LatinTitle()}} {
			if layout.Component(name.value, whole) != "Unknown" && layout.Component(name.latin, cut) == "Unknown" {
				warn(t, "the %s %q has no Latin letters, it becomes \"Unknown\" on the player (set a Latin form for it)", name.kind, name.value)
			}
		}
		if artist.LooksLikeSeveral(t.Artist) {
			warn(t, "the artist %q looks like several artists; the player's name for it is made from all of it", t.Artist)
		}
		key := strings.ToLower(layout.PathFor(t, cut))
		byName[key] = append(byName[key], t)
	}
	for _, same := range byName {
		for _, t := range same {
			var clashes []string
			for _, u := range same {
				if u.ID != t.ID && !strings.EqualFold(layout.PathFor(u, uncut), layout.PathFor(t, uncut)) {
					clashes = append(clashes, u.ID)
				}
			}
			if len(clashes) > 0 {
				warn(t, "its file name is cut at %d characters and becomes the same as that of %s; one of them is renamed with (2)", WolfgangStemWidth, strings.Join(clashes, ", "))
			}
		}
	}
	return out
}
