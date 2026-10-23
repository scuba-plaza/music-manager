package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"music-manager/internal/manifest"
	"music-manager/internal/timecode"
	"music-manager/internal/translit"
)

// SortKey is a field tracks can be put in order by.
type SortKey string

const (
	SortArtist   SortKey = "artist"
	SortAlbum    SortKey = "album"
	SortTitle    SortKey = "title"
	SortTrack    SortKey = "track"
	SortLength   SortKey = "length"
	SortReleased SortKey = "released"
	SortAdded    SortKey = "added"
	SortEdited   SortKey = "edited"
)

// SortKeys lists every key, in the order a menu shows them.
var SortKeys = []SortKey{SortArtist, SortAlbum, SortTitle, SortTrack, SortLength, SortReleased, SortAdded, SortEdited}

// ParseSortKey reads a key by its name.
func ParseSortKey(s string) (SortKey, error) {
	for _, k := range SortKeys {
		if string(k) == strings.ToLower(strings.TrimSpace(s)) {
			return k, nil
		}
	}
	names := make([]string, len(SortKeys))
	for i, k := range SortKeys {
		names[i] = string(k)
	}
	return "", fmt.Errorf("cannot sort by %q: choose one of %s", s, strings.Join(names, ", "))
}

// Label is the key's name as shown to people.
func (k SortKey) Label() string {
	switch k {
	case SortTrack:
		return "Track number"
	case SortAdded:
		return "Date added"
	case SortEdited:
		return "Date edited"
	case SortReleased:
		return "Release date"
	}
	return strings.ToUpper(string(k)[:1]) + string(k)[1:]
}

// IsDate reports whether the key is one of the three dates.
func (k SortKey) IsDate() bool { return k == SortReleased || k == SortAdded || k == SortEdited }

// dateLayout is how the dates of adding and editing are shown.
const dateLayout = "2006-01-02"

// SortValue is what a track has for the key, as text to show beside it. It
// is empty when the track has none.
func SortValue(t manifest.Track, k SortKey) string {
	artist, title, album := Display(t)
	switch k {
	case SortArtist:
		return artist
	case SortTitle:
		return title
	case SortAlbum:
		return album
	case SortTrack:
		if n := trackNumber(t); n > 0 {
			return strconv.Itoa(n)
		}
	case SortLength:
		return timecode.Format(t.PlayingLength())
	case SortReleased:
		return t.Released
	case SortAdded:
		if !t.AddedAt.IsZero() {
			return t.AddedAt.Local().Format(dateLayout)
		}
	case SortEdited:
		if !t.UpdatedAt.IsZero() {
			return t.UpdatedAt.Local().Format(dateLayout)
		}
	}
	return ""
}

// trackNumber is the number of a track on its album, taken from the old
// tags for a track that is not identified yet.
func trackNumber(t manifest.Track) int {
	if t.Status != manifest.Identified && t.TrackNumber == 0 {
		return t.Hint.Track
	}
	return t.TrackNumber
}

// SortTracks returns a copy of ts in the order of the key. Text and numbers
// go from low to high and dates from newest to oldest; reverse turns that
// around. Tracks that have no value for the key come last either way, and
// tracks that tie are ordered by artist, album, track number and title, so
// the order never depends on how the tracks happened to arrive.
func SortTracks(ts []manifest.Track, k SortKey, reverse bool) []manifest.Track {
	out := append([]manifest.Track(nil), ts...)
	if k.IsDate() {
		reverse = !reverse // newest first is the natural order of a date
	}
	sort.SliceStable(out, func(i, j int) bool {
		c, missing := compareBy(out[i], out[j], k)
		if reverse && !missing {
			c = -c
		}
		if c != 0 {
			return c < 0
		}
		return tieBreak(out[i], out[j]) < 0
	})
	return out
}

// compareBy compares a and b by the key: negative if a comes first, positive
// if b does, zero if they tie. missing is true when the order was decided by
// one of them having no value for the key (or neither having one), which
// puts it last however the order is turned.
func compareBy(a, b manifest.Track, k SortKey) (c int, missing bool) {
	has := func(t manifest.Track) bool {
		switch k {
		case SortArtist, SortTitle, SortAlbum:
			return SortValue(t, k) != ""
		case SortTrack:
			return trackNumber(t) > 0
		case SortReleased:
			return t.Released != ""
		case SortAdded:
			return !t.AddedAt.IsZero()
		case SortEdited:
			return !t.UpdatedAt.IsZero()
		}
		return true
	}
	switch ha, hb := has(a), has(b); {
	case ha && !hb:
		return -1, true
	case !ha && hb:
		return 1, true
	case !ha && !hb:
		return 0, true
	}
	switch k {
	case SortArtist, SortTitle, SortAlbum:
		return strings.Compare(translit.SortKey(SortValue(a, k)), translit.SortKey(SortValue(b, k))), false
	case SortTrack:
		return trackNumber(a) - trackNumber(b), false
	case SortLength:
		return compareInt(int64(a.PlayingLength()), int64(b.PlayingLength())), false
	case SortReleased:
		return strings.Compare(a.Released, b.Released), false
	case SortAdded:
		return a.AddedAt.Compare(b.AddedAt), false
	case SortEdited:
		return a.UpdatedAt.Compare(b.UpdatedAt), false
	}
	return 0, false
}

func compareInt(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// tieBreak orders tracks that are equal in the key being sorted by.
func tieBreak(a, b manifest.Track) int {
	for _, k := range []SortKey{SortArtist, SortAlbum, SortTrack, SortTitle} {
		if c, _ := compareBy(a, b, k); c != 0 {
			return c
		}
	}
	return strings.Compare(a.ID, b.ID)
}
