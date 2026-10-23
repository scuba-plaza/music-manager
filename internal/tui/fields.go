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
	"music-manager/internal/timecode"
	"music-manager/internal/translit"
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
	fPlayerArtist
	fPlayerAlbum
)

// An inspectorTab is a page of the inspector: some of the fields, under a name.
type inspectorTab struct {
	name   string
	fields []fieldKind
}

var inspectorTabs = []inspectorTab{
	{"Details", []fieldKind{fTitle, fArtist, fAlbum, fTrack, fReleased, fGenre, fCover}},
	{"Audio", []fieldKind{fCutStart, fCutEnd}},
	{"Player", []fieldKind{fPlayerArtist, fPlayerAlbum}},
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
	return [...]string{"Title", "Artist", "Album", "Track", "Released", "Genre", "Cut start", "Cut end", "Cover", "Artist", "Album"}[f]
}

// isName reports whether the field is text that also has an ASCII form.
func (f fieldKind) isName() bool { return f == fTitle || f == fArtist || f == fAlbum }

// names returns the original text of a name field and its ASCII form. A
// track that is not identified yet has no title or artist of its own, so the
// field starts from what the file said about itself.
func names(t manifest.Track, f fieldKind) (original, ascii string) {
	switch f {
	case fTitle:
		original, ascii = t.OriginalTitle, t.Title
	case fArtist:
		original, ascii = t.OriginalArtist, t.Artist
	case fAlbum:
		original, ascii = t.OriginalAlbum, t.Album
	}
	if original == "" && t.Status != manifest.Identified {
		switch f {
		case fTitle:
			original = t.Hint.Title
			if original == "" {
				original = leadingNumber.ReplaceAllString(t.Hint.Filename, "")
			}
		case fArtist:
			original = t.Hint.Artist
		case fAlbum:
			original = t.Hint.Album
		}
	}
	if ascii == "" {
		ascii = translit.ToASCII(original)
	}
	return original, ascii
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
		original, _ := names(t, f)
		return original
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
	case fPlayerArtist, fPlayerAlbum:
		if t.Status != manifest.Identified {
			return ""
		}
		shown, _ := playerShown(t, f)
		return shown
	}
	if s := cutText(f, t); s != "" {
		return s
	}
	return "0:00"
}

// draft is a field while it is being edited: its text, and for a name the
// ASCII form beside it, for the track number the number of tracks.
type draft struct {
	field  fieldKind
	text   string
	second string
	// touched: the user wrote the second input themselves.
	touched bool
	// auto: what the player shows when the field is left to itself, for the
	// player names: text equal to it is not an override.
	auto string
	// follow: the ASCII form was the automatic one, so it keeps following
	// the text until touched. A custom form stays put.
	follow bool
}

func newDraft(t manifest.Track, f fieldKind) draft {
	d := draft{field: f}
	switch {
	case f.isName():
		d.text, d.second = names(t, f)
		d.follow = d.second == translit.ToASCII(d.text)
	case f == fTrack:
		n, of := numbers(t)
		d.text, d.second = numberText(n), numberText(of)
	case f == fCover:
		// the text is the path of a new image
	case f == fReleased:
		d.text = releasedText(t)
	case f == fGenre:
		d.text = genreText(t)
	case f == fPlayerArtist || f == fPlayerAlbum:
		d.text, d.auto = playerShown(t, f)
	default:
		d.text = cutText(f, t)
	}
	return d
}

func (d *draft) setText(s string) {
	d.text = s
	if d.field.isName() && d.follow && !d.touched {
		d.second = translit.ToASCII(s)
	}
}

func (d *draft) setSecond(s string) {
	d.second = s
	d.touched = true
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
		var ascii *string
		if d.touched {
			v := strings.TrimSpace(d.second)
			if v == "" || v == translit.ToASCII(text) {
				v = "" // the automatic form
			}
			ascii = &v
		}
		switch d.field {
		case fTitle:
			e.Title, e.TitleASCII = &text, ascii
		case fArtist:
			e.Artist, e.ArtistASCII = &text, ascii
		case fAlbum:
			e.Album, e.AlbumASCII = &text, ascii
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
	case fPlayerArtist, fPlayerAlbum:
		name := strings.TrimSpace(d.text)
		if name == d.auto {
			name = "" // the automatic one is not stored
		}
		if d.field == fPlayerArtist {
			e.PlayerArtist = &name
		} else {
			e.PlayerAlbum = &name
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
		ascii := d.second
		if d.touched {
			ascii = strings.TrimSpace(d.second)
		}
		if ascii == "" || (d.follow && !d.touched) {
			ascii = translit.ToASCII(text)
		}
		switch d.field {
		case fTitle:
			t.OriginalTitle, t.Title = text, ascii
		case fArtist:
			t.OriginalArtist, t.Artist = text, ascii
		case fAlbum:
			t.OriginalAlbum, t.Album = text, ascii
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

// playerShown is what the player shows for the artist or album of t, and what
// it would show if the track had no name of its own for it.
func playerShown(t manifest.Track, f fieldKind) (shown, auto string) {
	artist, album := app.PlayerTags(t)
	t.PlayerArtist, t.PlayerAlbum = "", ""
	autoArtist, autoAlbum := app.PlayerTags(t)
	if f == fPlayerArtist {
		return artist, autoArtist
	}
	return album, autoAlbum
}
