package browse

import (
	"errors"
	"strings"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/timecode"
)

// BulkValues is what was typed in the form that edits several tracks at once.
// A blank field keeps what each track has; "-" clears it.
type BulkValues struct {
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	Of       string `json:"of"`
	Released string `json:"released"`
	Genre    string `json:"genre"`
	CutStart string `json:"cutStart"`
	CutEnd   string `json:"cutEnd"`
	// The forms of the artist, the album and the genre in Latin letters; "-"
	// takes the one set by hand away.
	ArtistLatin string `json:"artistLatin"`
	AlbumLatin  string `json:"albumLatin"`
	GenreLatin  string `json:"genreLatin"`
}

// clearMark, typed in a field, takes the value away.
const clearMark = "-"

// edit turns the typed values into the change to make to every track.
func (v BulkValues) edit() (app.Edit, error) {
	var e app.Edit
	if s := strings.TrimSpace(v.Artist); s != "" {
		if s == clearMark {
			return e, errors.New("An artist cannot be cleared")
		}
		e.Artist = &s
	}
	if s := strings.TrimSpace(v.Album); s != "" {
		if s == clearMark {
			s = ""
		}
		e.Album = &s
	}
	if s := strings.TrimSpace(v.Of); s != "" {
		n := 0
		if s != clearMark {
			var err error
			if n, err = count(s); err != nil {
				return e, errors.New("Tracks on the album must be a whole number")
			}
		}
		e.TrackTotal = &n
	}
	if s := strings.TrimSpace(v.Released); s != "" {
		if s == clearMark {
			s = ""
		}
		date, err := app.NormalizeDate(s)
		if err != nil {
			return e, err
		}
		e.Released = &date
	}
	if s := strings.TrimSpace(v.Genre); s != "" {
		if s == clearMark {
			s = ""
		}
		e.Genre = &s
	}
	for _, f := range []struct {
		text string
		dst  **string
		name string
	}{{v.ArtistLatin, &e.ArtistLatin, "Artist (Latin)"}, {v.AlbumLatin, &e.AlbumLatin, "Album (Latin)"}, {v.GenreLatin, &e.GenreLatin, "Genre (Latin)"}} {
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
	}{{v.CutStart, &e.TrimStart, "Cut start"}, {v.CutEnd, &e.TrimEnd, "Cut end"}} {
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
		e.Cover != nil && t.CoverSHA256 != *e.Cover,
		e.TrimStart != nil && t.TrimStart != *e.TrimStart,
		e.TrimEnd != nil && t.TrimEnd != *e.TrimEnd:
		return true
	}
	return false
}

// bulkPlan works out what to do to each of tracks: the same edit for those
// it would change, and nothing for those that already have the values. It
// returns the edits by track ID.
func bulkPlan(tracks []manifest.Track, v BulkValues) (map[string]app.Edit, error) {
	e, err := v.edit()
	if err != nil {
		return nil, err
	}
	edits := map[string]app.Edit{}
	for _, t := range tracks {
		if changes(t, e) {
			edits[t.ID] = e
		}
	}
	return edits, nil
}

// Summary says in words what the typed values would do.
func (v BulkValues) Summary() string {
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
	say("Artist", v.Artist, "")
	say("Artist (Latin)", v.ArtistLatin, "worked out")
	say("Album", v.Album, "none (a single)")
	say("Album (Latin)", v.AlbumLatin, "worked out")
	say("Tracks on album", v.Of, "none")
	say("Released", v.Released, "unknown")
	say("Genre", v.Genre, "none")
	say("Genre (Latin)", v.GenreLatin, "worked out")
	say("Cut start", v.CutStart, "none")
	say("Cut end", v.CutEnd, "none")
	return strings.Join(parts, " · ")
}

// BulkPreview says what the values would do to the tracks, or why they cannot be used.
type BulkPreview struct {
	Summary string `json:"summary"`
	Changed int    `json:"changed"`
	Error   string `json:"error,omitempty"`
}

// PreviewBulk checks the values typed for the tracks, without changing anything.
func PreviewBulk(s Snapshot, ids []string, v BulkValues) BulkPreview {
	p := BulkPreview{Summary: v.Summary()}
	edits, err := bulkPlan(s.tracksFor(ids), v)
	if err != nil {
		p.Error = err.Error()
		return p
	}
	p.Changed = len(edits)
	return p
}

// Bulk applies the values to the tracks, as one change.
func (e *Editor) Bulk(ids []string, v BulkValues) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	tracks := s.tracksFor(ids)
	edits, err := bulkPlan(tracks, v)
	if err != nil {
		return Result{}, err
	}
	if len(edits) == 0 {
		return Result{}, errors.New("Nothing to change")
	}
	changed := make([]string, 0, len(edits))
	for _, t := range tracks { // in the order they are listed
		if _, ok := edits[t.ID]; ok {
			changed = append(changed, t.ID)
		}
	}
	return e.saveMany(s, "bulk edit of "+plural(len(changed), "track"), "Changed "+plural(len(changed), "track"), changed,
		func() (map[string]manifest.Track, error) { return e.lib.EditMany(edits) })
}
