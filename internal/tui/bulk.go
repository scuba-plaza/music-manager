package tui

import (
	"errors"
	"strings"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/timecode"
)

// bulkValues is what was typed in the form that edits several tracks at once.
// A blank field keeps what each track has; "-" clears it.
type bulkValues struct {
	artist, album, of, released, genre, cutStart, cutEnd string
	// The forms of the artist, the album and the genre in Latin letters; "-"
	// takes the one set by hand away.
	artistLatin, albumLatin, genreLatin string
}

const clearMark = "-"

// edit turns the typed values into the change to make to every track.
func (v bulkValues) edit() (app.Edit, error) {
	var e app.Edit
	if s := strings.TrimSpace(v.artist); s != "" {
		if s == clearMark {
			return e, errors.New("An artist cannot be cleared")
		}
		e.Artist = &s
	}
	if s := strings.TrimSpace(v.album); s != "" {
		if s == clearMark {
			s = ""
		}
		e.Album = &s
	}
	if s := strings.TrimSpace(v.of); s != "" {
		n := 0
		if s != clearMark {
			var err error
			if n, err = count(s); err != nil {
				return e, errors.New("Tracks on the album must be a whole number")
			}
		}
		e.TrackTotal = &n
	}
	if s := strings.TrimSpace(v.released); s != "" {
		if s == clearMark {
			s = ""
		}
		date, err := app.NormalizeDate(s)
		if err != nil {
			return e, err
		}
		e.Released = &date
	}
	if s := strings.TrimSpace(v.genre); s != "" {
		if s == clearMark {
			s = ""
		}
		e.Genre = &s
	}
	for _, f := range []struct {
		text string
		dst  **string
		name string
	}{{v.artistLatin, &e.ArtistLatin, "Artist (Latin)"}, {v.albumLatin, &e.AlbumLatin, "Album (Latin)"}, {v.genreLatin, &e.GenreLatin, "Genre (Latin)"}} {
		s := strings.TrimSpace(f.text)
		switch {
		case s == "":
			continue
		case strings.ContainsAny(s, "\r\n"):
			return e, errors.New(f.name + " must be on one line")
		case s == clearMark:
			s = ""
		}
		*f.dst = &s
	}
	for _, c := range []struct {
		text string
		dst  **time.Duration
		name string
	}{{v.cutStart, &e.TrimStart, "Cut start"}, {v.cutEnd, &e.TrimEnd, "Cut end"}} {
		s := strings.TrimSpace(c.text)
		if s == "" {
			continue
		}
		var d time.Duration
		if s != clearMark {
			var err error
			if d, err = timecode.Parse(s); err != nil {
				return e, errors.New(c.name + ": " + err.Error())
			}
		}
		*c.dst = &d
	}
	return e, nil
}

// changes reports whether e would alter t.
func changes(t manifest.Track, e app.Edit) bool {
	switch {
	case e.Artist != nil && t.Artist != *e.Artist,
		e.Album != nil && t.Album != *e.Album,
		e.TrackTotal != nil && t.TrackTotal != *e.TrackTotal,
		e.Released != nil && t.Released != *e.Released,
		e.Genre != nil && t.Genre != *e.Genre,
		e.ArtistLatin != nil && t.ArtistLatin != *e.ArtistLatin,
		e.AlbumLatin != nil && t.AlbumLatin != *e.AlbumLatin,
		e.GenreLatin != nil && t.GenreLatin != *e.GenreLatin,
		e.TrimStart != nil && t.TrimStart != *e.TrimStart,
		e.TrimEnd != nil && t.TrimEnd != *e.TrimEnd:
		return true
	}
	return false
}

// bulkPlan works out what to do to each of tracks: the same edit for those
// it would change, and nothing for those that already have the values. It
// returns the edits by track ID and how many tracks change.
func bulkPlan(tracks []manifest.Track, v bulkValues) (map[string]app.Edit, int, error) {
	e, err := v.edit()
	if err != nil {
		return nil, 0, err
	}
	edits := map[string]app.Edit{}
	for _, t := range tracks {
		if changes(t, e) {
			edits[t.ID] = e
		}
	}
	return edits, len(edits), nil
}

// bulkSummary says in words what the typed values would do.
func bulkSummary(v bulkValues) string {
	var parts []string
	say := func(name, text, cleared string) {
		switch s := strings.TrimSpace(text); s {
		case "":
		case clearMark:
			parts = append(parts, name+" → "+cleared)
		default:
			parts = append(parts, name+" → "+s)
		}
	}
	say("Artist", v.artist, "")
	say("Artist (Latin)", v.artistLatin, "worked out")
	say("Album", v.album, "none (a single)")
	say("Album (Latin)", v.albumLatin, "worked out")
	say("Tracks on album", v.of, "none")
	say("Released", v.released, "unknown")
	say("Genre", v.genre, "none")
	say("Genre (Latin)", v.genreLatin, "worked out")
	say("Cut start", v.cutStart, "none")
	say("Cut end", v.cutEnd, "none")
	return strings.Join(parts, " · ")
}
