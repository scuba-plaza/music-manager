package app

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"music-manager/internal/manifest"
	"music-manager/internal/translit"
)

// Edit describes changes to a track. A nil field is left alone.
//
// Title, Artist and Album are the original (possibly non-ASCII) values; their
// ASCII forms follow automatically. A *ASCII field overrides the derived
// form and sticks across later changes of the original; setting it to ""
// goes back to deriving.
type Edit struct {
	Title, Artist, Album                *string
	TitleASCII, ArtistASCII, AlbumASCII *string
	TrackNumber, TrackTotal             *int
	TrimStart, TrimEnd                  *time.Duration
	// Released is when the track first came out: 1999, 1999-05 or 1999-05-17,
	// or "" for unknown.
	Released *string
	// SourceURL is the link the track came from, or "" for none.
	SourceURL *string
}

// Edit applies e to the track with the given ID and returns the result. A
// track with a title and an artist is identified.
func (a *App) Edit(id string, e Edit) (manifest.Track, error) {
	m, err := a.store.Load()
	if err != nil {
		return manifest.Track{}, err
	}
	t, ok := m.Get(id)
	if !ok {
		return manifest.Track{}, fmt.Errorf("no track with id %q", id)
	}

	t, err = applyEdit(t, e)
	if err != nil {
		return manifest.Track{}, err
	}
	m.Update(t)
	return t, a.store.Save(m)
}

// applyEdit returns t with e applied and its status recomputed. It changes
// nothing if the result would be invalid.
func applyEdit(t manifest.Track, e Edit) (manifest.Track, error) {
	t.Title, t.OriginalTitle = setName(t.Title, t.OriginalTitle, e.Title, e.TitleASCII)
	t.Artist, t.OriginalArtist = setName(t.Artist, t.OriginalArtist, e.Artist, e.ArtistASCII)
	t.Album, t.OriginalAlbum = setName(t.Album, t.OriginalAlbum, e.Album, e.AlbumASCII)
	if e.TrackNumber != nil {
		t.TrackNumber = *e.TrackNumber
	}
	if e.TrackTotal != nil {
		t.TrackTotal = *e.TrackTotal
	}
	if e.TrimStart != nil {
		t.TrimStart = *e.TrimStart
	}
	if e.TrimEnd != nil {
		t.TrimEnd = *e.TrimEnd
	}
	if e.Released != nil {
		d, err := NormalizeDate(*e.Released)
		if err != nil {
			return t, err
		}
		t.Released = d
	}
	if e.SourceURL != nil {
		u, err := NormalizeSourceURL(*e.SourceURL)
		if err != nil {
			return t, err
		}
		t.SourceURL = u
	}
	if err := validate(t); err != nil {
		return t, err
	}

	if t.Title != "" && t.Artist != "" {
		t.Status = manifest.Identified
		t.Hint = manifest.Hint{}
	} else {
		t.Status = manifest.Unidentified
	}
	t.UpdatedAt = time.Now().UTC()
	return t, nil
}

// setName returns the new ASCII and original forms of a name field.
func setName(ascii, orig string, newOrig, newASCII *string) (string, string) {
	overridden := ascii != translit.ToASCII(orig)
	if newOrig != nil {
		orig = strings.TrimSpace(*newOrig)
		if !overridden || orig == "" {
			ascii = translit.ToASCII(orig)
		}
	}
	if newASCII != nil {
		if *newASCII == "" {
			ascii = translit.ToASCII(orig)
		} else {
			ascii = strings.TrimSpace(*newASCII)
		}
	}
	return ascii, orig
}

// NormalizeDate checks a release date and returns it trimmed. It may be as
// precise as a year (1999), a month (1999-05) or a day (1999-05-17); empty
// means unknown.
func NormalizeDate(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	for _, layout := range []string{"2006", "2006-01", "2006-01-02"} {
		if d, err := time.Parse(layout, s); err == nil && d.Year() >= 1000 {
			return s, nil
		}
	}
	return "", fmt.Errorf("release date %q is not a date: use 1999, 1999-05 or 1999-05-17", s)
}

func validate(t manifest.Track) error {
	switch {
	case t.TrimStart < 0 || t.TrimEnd < 0:
		return fmt.Errorf("trims cannot be negative")
	case t.TrimStart+t.TrimEnd >= t.SourceLength:
		return fmt.Errorf("trims (%v + %v) leave nothing of a %v track", t.TrimStart, t.TrimEnd, t.SourceLength.Round(time.Millisecond))
	case t.TrackNumber < 0 || t.TrackTotal < 0:
		return fmt.Errorf("track numbers cannot be negative")
	}
	return nil
}

var leadingDigits = regexp.MustCompile(`^\d{4}(-\d{2}(-\d{2})?)?`)

// leadingDate is the date that s starts with, as precise as it is valid:
// tag editors write "2015-09-04T10:00:00", and "2015-02-30" still means
// 2015. It is empty if s does not start with a date.
func leadingDate(s string) string {
	s = leadingDigits.FindString(strings.TrimSpace(s))
	for _, n := range []int{len(s), 7, 4} {
		if n > len(s) {
			continue
		}
		if d, err := NormalizeDate(s[:n]); err == nil {
			return d
		}
	}
	return ""
}

// NormalizeSourceURL checks the link a track came from and returns it
// trimmed. It must be a web address (http or https); empty means none.
func NormalizeSourceURL(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	u, err := url.Parse(s)
	if err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
		return s, nil
	}
	return "", fmt.Errorf("source %q is not a web link: use something like https://www.youtube.com/watch?v=...", s)
}
