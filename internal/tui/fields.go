package tui

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/mp3"
	"music-manager/internal/timecode"
)

// fieldKind is one editable field of a track, in the order the inspector
// lists them.
type fieldKind int

const (
	fTitle fieldKind = iota
	fArtist
	fAlbum
	fTrack
	fReleased
	fGenre
	fCutStart
	fCutEnd
	fCover
	fVolume
	fFadeIn
	fFadeOut
	fWolfgang // the read-only preview under --wolfgang
)

// An inspectorTab is a page of the inspector: some of the fields, under a name.
type inspectorTab struct {
	name   string
	fields []fieldKind
}

var inspectorTabs = []inspectorTab{
	{"Details", []fieldKind{fTitle, fArtist, fAlbum, fTrack, fReleased, fGenre, fCover}},
	{"Audio", []fieldKind{fCutStart, fCutEnd, fVolume, fFadeIn, fFadeOut}},
	{"Wolfgang", []fieldKind{fWolfgang}},
}

// trackFields is every field, tab by tab, in the order the cursor goes through
// them.
var trackFields = func() []fieldKind {
	var all []fieldKind
	for _, tab := range inspectorTabs {
		all = append(all, tab.fields...)
	}
	return all
}()

// tabOf is the index of the tab that holds f.
func tabOf(f fieldKind) int {
	for i, tab := range inspectorTabs {
		if slices.Contains(tab.fields, f) {
			return i
		}
	}
	return 0
}

// firstFieldOf is the place in trackFields of the first field of a tab.
func firstFieldOf(tab int) int {
	n := 0
	for _, t := range inspectorTabs[:tab] {
		n += len(t.fields)
	}
	return n
}

// tab is the tab the inspector shows: the one that holds the chosen field, so
// that it stays the same from track to track.
func (m Model) tab() int { return tabOf(trackFields[m.field]) }

func (f fieldKind) label() string {
	return [...]string{"Title", "Artist", "Album", "Track", "Released", "Genre", "Cut start", "Cut end", "Cover", "Volume", "Fade in", "Fade out", "Wolfgang"}[f]
}

// isName reports whether the field is the title, the artist or the album.
func (f fieldKind) isName() bool { return f == fTitle || f == fArtist || f == fAlbum }

// name returns the title, artist or album of a track. A track that is not
// identified yet has no title or artist of its own, so the field starts from
// what the file said about itself.
func name(t manifest.Track, f fieldKind) string {
	var s string
	switch f {
	case fTitle:
		s = t.Title
	case fArtist:
		s = t.Artist
	case fAlbum:
		s = t.Album
	}
	if s == "" && t.Status != manifest.Identified {
		switch f {
		case fTitle:
			s = t.Hint.Title
			if s == "" {
				s = leadingNumber.ReplaceAllString(t.Hint.Filename, "")
			}
		case fArtist:
			s = t.Hint.Artist
		case fAlbum:
			s = t.Hint.Album
		}
	}
	return s
}

// leadingNumber matches the "04 " or "04 - " that file names start with.
var leadingNumber = regexp.MustCompile(`^\d+\s*[-._)\]]*\s+`)

// numbers returns a track's number and the number of tracks on its album,
// falling back to the old tags for a track that is not identified.
func numbers(t manifest.Track) (n, of int) {
	n, of = t.TrackNumber, t.TrackTotal
	if t.Status != manifest.Identified && n == 0 {
		n, of = t.Hint.Track, t.Hint.Total
	}
	return n, of
}

func numberText(n int) string {
	if n <= 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// releasedText is a track's release date, falling back to the one in the old
// tags of a track that is not identified.
func releasedText(t manifest.Track) string {
	if t.Released == "" && t.Status != manifest.Identified {
		return t.Hint.Date
	}
	return t.Released
}

// genreText is a track's genre, falling back to the one in the old tags of a
// track that is not identified.
func genreText(t manifest.Track) string {
	if t.Genre == "" && t.Status != manifest.Identified {
		return t.Hint.Genre
	}
	return t.Genre
}

func cutText(f fieldKind, t manifest.Track) string {
	d := t.TrimStart
	if f == fCutEnd {
		d = t.TrimEnd
	}
	if d <= 0 {
		return ""
	}
	return timecode.Format(d)
}

// displayValue is how a field reads when it is not being edited.
func displayValue(t manifest.Track, f fieldKind) string {
	switch f {
	case fTitle, fArtist, fAlbum:
		return name(t, f)
	case fTrack:
		n, of := numbers(t)
		switch {
		case n > 0 && of > 0:
			return fmt.Sprintf("%d of %d", n, of)
		case n > 0:
			return strconv.Itoa(n)
		}
		return ""
	case fReleased:
		return releasedText(t)
	case fGenre:
		return genreText(t)
	case fVolume:
		return app.VolumeText(t.GainSteps)
	case fFadeIn:
		return app.FadeText(t.FadeIn, true, t)
	case fFadeOut:
		return app.FadeText(t.FadeOut, false, t)
	}
	if s := cutText(f, t); s != "" {
		return s
	}
	return "0:00"
}

// draft is a field while it is being edited: its text, and for the track
// number the number of tracks.
type draft struct {
	field  fieldKind
	text   string
	second string
	// track is the track being edited, for the fades, which are typed as lengths
	// from its cuts.
	track manifest.Track
}

func newDraft(t manifest.Track, f fieldKind) draft {
	d := draft{field: f, track: t}
	switch {
	case f.isName():
		d.text = name(t, f)
	case f == fTrack:
		n, of := numbers(t)
		d.text, d.second = numberText(n), numberText(of)
	case f == fCover:
		// the text is the path of a new image
	case f == fReleased:
		d.text = releasedText(t)
	case f == fGenre:
		d.text = genreText(t)
	case f == fVolume:
		if t.GainSteps != 0 {
			d.text = fmt.Sprintf("%+.1f", float64(t.GainSteps)*mp3.StepDB)
		}
	case f == fFadeIn:
		d.text = app.FadeText(t.FadeIn, true, t)
	case f == fFadeOut:
		d.text = app.FadeText(t.FadeOut, false, t)
	default:
		d.text = cutText(f, t)
	}
	return d
}

// edit validates the draft and says what to change.
func (d draft) edit() (app.Edit, error) {
	var e app.Edit
	switch d.field {
	case fTitle, fArtist, fAlbum:
		text := strings.TrimSpace(d.text)
		if text == "" && d.field != fAlbum {
			return e, fmt.Errorf("%s cannot be empty", d.field.label())
		}
		switch d.field {
		case fTitle:
			e.Title = &text
		case fArtist:
			e.Artist = &text
		case fAlbum:
			e.Album = &text
		}
	case fTrack:
		n, err := count(d.text)
		if err != nil {
			return e, errors.New("Track must be a whole number, or empty")
		}
		of, err := count(d.second)
		if err != nil {
			return e, errors.New("Tracks on the album must be a whole number, or empty")
		}
		e.TrackNumber, e.TrackTotal = &n, &of
	case fReleased:
		date, err := app.NormalizeDate(d.text)
		if err != nil {
			return e, err
		}
		e.Released = &date
	case fGenre:
		genre := strings.TrimSpace(d.text)
		e.Genre = &genre
	case fVolume:
		db, err := app.ParseGain(d.text)
		if err != nil {
			return e, err
		}
		e.GainDB = &db
	case fFadeIn, fFadeOut:
		span, err := app.ParseFade(d.text, d.field == fFadeIn, d.track)
		if err != nil {
			return e, err
		}
		if d.field == fFadeIn {
			e.FadeIn = &span
		} else {
			e.FadeOut = &span
		}
	case fCutStart, fCutEnd:
		var cut time.Duration
		if s := strings.TrimSpace(d.text); s != "" {
			v, err := timecode.Parse(s)
			if err != nil {
				return e, fmt.Errorf("%s: %w", d.field.label(), err)
			}
			cut = v
		}
		if d.field == fCutStart {
			e.TrimStart = &cut
		} else {
			e.TrimEnd = &cut
		}
	}
	return e, nil
}

func count(s string) (int, error) {
	if s = strings.TrimSpace(s); s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, errors.New("not a count")
	}
	return n, nil
}

// previewTrack is t as it would be after committing the draft, which is
// enough to work out where its file would go.
func previewTrack(t manifest.Track, d draft) manifest.Track {
	switch d.field {
	case fTitle, fArtist, fAlbum:
		text := strings.TrimSpace(d.text)
		switch d.field {
		case fTitle:
			t.Title = text
		case fArtist:
			t.Artist = text
		case fAlbum:
			t.Album = text
		}
	case fTrack:
		t.TrackNumber, _ = count(d.text)
		t.TrackTotal, _ = count(d.second)
	}
	if t.Title != "" && t.Artist != "" {
		t.Status = manifest.Identified
	}
	return t
}
