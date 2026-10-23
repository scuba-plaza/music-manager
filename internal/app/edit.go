package app

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"

	"music-manager/internal/manifest"
	"music-manager/internal/mp3"
	"music-manager/internal/timecode"
)

// Edit describes changes to a track. A nil field is left alone.
//
// Title, Artist and Album are the true names, in whatever script they are
// written.
type Edit struct {
	Title, Artist, Album    *string
	TrackNumber, TrackTotal *int
	TrimStart, TrimEnd      *time.Duration
	// Released is when the track first came out: 1999, 1999-05 or 1999-05-17,
	// or "" for unknown.
	Released *string
	// SourceURL is the link the track came from, or "" for none.
	SourceURL *string
	// Genre is free text, one line, or "" for none.
	Genre *string
	// The Latin forms are how the names and the genre read in Latin letters,
	// each on one line; "" takes one away and the form worked out from the
	// name is used again.
	TitleLatin, ArtistLatin, AlbumLatin, GenreLatin *string
	// GainDB makes the track louder or quieter by about this much, rounded to
	// the nearest step the file allows (mp3.StepDB); 0 for no change.
	GainDB *float64
	// FadeIn and FadeOut are stretches of the original audio; an empty one
	// takes the fade away.
	FadeIn, FadeOut *manifest.Span
}

// MaxGainSteps is the most a track may be made louder or quieter, in steps.
const MaxGainSteps = 16 // 24 dB

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
	for _, f := range []struct {
		in  *string
		out *string
	}{{e.Title, &t.Title}, {e.Artist, &t.Artist}, {e.Album, &t.Album}} {
		if f.in != nil {
			*f.out = strings.TrimSpace(*f.in)
		}
	}
	for _, f := range []struct {
		name    string
		in, out *string
	}{{"title", e.TitleLatin, &t.TitleLatin}, {"artist", e.ArtistLatin, &t.ArtistLatin}, {"album", e.AlbumLatin, &t.AlbumLatin}, {"genre", e.GenreLatin, &t.GenreLatin}} {
		if f.in == nil {
			continue
		}
		if s := strings.TrimSpace(*f.in); strings.ContainsAny(s, "\r\n") {
			return t, fmt.Errorf("the Latin form of the %s, %q, must be on one line", f.name, s)
		} else {
			*f.out = s
		}
	}
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
	if e.Genre != nil {
		g := strings.TrimSpace(*e.Genre)
		if strings.ContainsAny(g, "\r\n") {
			return t, fmt.Errorf("genre %q must be on one line", g)
		}
		t.Genre = g
	}
	if e.GainDB != nil {
		t.GainSteps = int(math.Round(*e.GainDB / mp3.StepDB))
	}
	if e.FadeIn != nil {
		t.FadeIn = *e.FadeIn
	}
	if e.FadeOut != nil {
		t.FadeOut = *e.FadeOut
	}
	if err := validate(t); err != nil {
		return t, err
	}
	forgetLatinOfEmptyNames(&t)

	if t.Title != "" && t.Artist != "" {
		t.Status = manifest.Identified
		t.Hint = manifest.Hint{}
	} else {
		t.Status = manifest.Unidentified
	}
	t.UpdatedAt = time.Now().UTC()
	return t, nil
}

// forgetLatinOfEmptyNames drops the Latin form of a name that is empty: it
// would be a form of nothing.
func forgetLatinOfEmptyNames(t *manifest.Track) {
	for _, n := range []struct {
		name  string
		latin *string
	}{{t.Title, &t.TitleLatin}, {t.Artist, &t.ArtistLatin}, {t.Album, &t.AlbumLatin}, {t.Genre, &t.GenreLatin}} {
		if n.name == "" {
			*n.latin = ""
		}
	}
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
	case t.GainSteps > MaxGainSteps || t.GainSteps < -MaxGainSteps:
		return fmt.Errorf("a volume change of %+.1f dB is more than the %.0f dB allowed", float64(t.GainSteps)*mp3.StepDB, MaxGainSteps*mp3.StepDB)
	}
	for _, f := range []struct {
		name string
		span manifest.Span
	}{{"fade-in", t.FadeIn}, {"fade-out", t.FadeOut}} {
		if f.span == (manifest.Span{}) {
			continue
		}
		if f.span.Start < 0 || f.span.End <= f.span.Start || f.span.End > t.SourceLength {
			return fmt.Errorf("the %s (%s to %s) must start inside the track and end after it starts, within its %s",
				f.name, timecode.Format(f.span.Start), timecode.Format(f.span.End), timecode.Format(t.SourceLength.Round(time.Millisecond)))
		}
	}
	if t.FadeIn != (manifest.Span{}) && t.FadeOut != (manifest.Span{}) && t.FadeOut.Start < t.FadeIn.End {
		return fmt.Errorf("the fade-out (from %s) must not start before the fade-in ends (%s)",
			timecode.Format(t.FadeOut.Start), timecode.Format(t.FadeIn.End))
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
