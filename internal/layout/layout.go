// Package layout decides where each track lives in the rendered music tree.
// The path is a pure function of the record, so it can never go stale.
package layout

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"music-manager/internal/manifest"
)

const (
	ext         = ".mp3"
	inbox       = "_Inbox"
	playlistDir = "Playlists"
)

// Rules says how a target wants its paths made. The zero value keeps names
// as they are, cuts a path component at 255 bytes and does not shorten file
// names.
type Rules struct {
	// Latin makes the names the Latin forms of the track's names (the ones set by
	// hand, or worked out) instead of the true ones.
	Latin bool
	// Fold turns a name into what the target keeps of it; nil keeps it as it is.
	Fold func(string) string
	// ComponentMax is the most bytes in one path component; 0 means 255.
	ComponentMax int
	// StemMax is the most characters in a file name before ".mp3", the track
	// number and the " (2)" of a collision included: a longer title is cut
	// there, as the player shows no more. 0 means no limit.
	StemMax int
}

// PathFor returns the slash-separated path of t relative to the library
// root: Artist/Album/NN - Title.mp3, Artist/Title.mp3 for tracks without an
// album, or _Inbox/<id> - <old name>.mp3 for tracks not yet identified. The
// file name is cut at r.StemMax characters.
func PathFor(t manifest.Track, r Rules) string { return pathFor(t, "", r) }

// pathFor is PathFor with suffix (" (2)") put after the name, which is cut to
// leave room for it.
func pathFor(t manifest.Track, suffix string, r Rules) string {
	room := r.StemMax - len(suffix)
	stem := func(s string, room int) string {
		if r.StemMax <= 0 {
			return s
		}
		return cut(s, room)
	}
	if t.Status != manifest.Identified {
		name := t.ID
		if hint := clean(t.Hint.Filename, r); hint != "Unknown" {
			name += " - " + hint
		}
		return path.Join(inbox, stem(name, room)+suffix+ext)
	}
	artistName, albumName, titleName := t.Artist, t.Album, t.Title
	if r.Latin {
		artistName, albumName, titleName = t.LatinArtist(), t.LatinAlbum(), t.LatinTitle()
	}
	artist, title := clean(artistName, r), clean(titleName, r)
	if t.Album == "" {
		return path.Join(artist, stem(title, room)+suffix+ext)
	}
	number := ""
	if t.TrackNumber > 0 {
		width := max(2, len(strconv.Itoa(t.TrackTotal)))
		number = fmt.Sprintf("%0*d - ", width, t.TrackNumber)
	}
	return path.Join(artist, clean(albumName, r), number+stem(title, room-len(number))+suffix+ext)
}

// PlaylistPath is where the playlist with this name is rendered:
// Playlists/<name><ext>, with the name made safe for a file name.
func PlaylistPath(name, ext string, r Rules) string {
	return path.Join(playlistDir, clean(name, r)+ext)
}

// Assign returns the path of every track by ID. Tracks that would share a
// path (compared case-insensitively, as on FAT) get " (2)", " (3)", ...
// before the extension; the oldest track keeps the plain name.
func Assign(tracks []manifest.Track, r Rules) map[string]string {
	ordered := append([]manifest.Track(nil), tracks...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].AddedBefore(ordered[j])
	})
	taken := map[string]bool{}
	out := make(map[string]string, len(ordered))
	for _, t := range ordered {
		p := PathFor(t, r)
		for n := 2; taken[strings.ToLower(p)]; n++ {
			p = pathFor(t, fmt.Sprintf(" (%d)", n), r)
		}
		taken[strings.ToLower(p)] = true
		out[t.ID] = p
	}
	return out
}

// Component is s as one path component under the rules: what a folder or the
// name of a file is made of it, "Unknown" if nothing is left.
func Component(s string, r Rules) string { return clean(s, r) }

// clean makes s usable as one path component on any filesystem, with its
// accents composed so that one name has one spelling.
func clean(s string, r Rules) string {
	s = norm.NFC.String(s)
	if r.Fold != nil {
		s = r.Fold(s)
	}
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < 0x20 || r == 0x7f {
			return '-'
		}
		return r
	}, s)
	s = strings.Trim(truncate(s, r.ComponentMax), " .'")
	if strings.Trim(s, "-") == "" {
		return "Unknown"
	}
	return s
}

// cut keeps the first n characters of s and no more, as the player does, and
// leaves nothing at its end that a file name cannot end with.
func cut(s string, n int) string {
	if runes := []rune(s); len(runes) > max(n, 1) {
		s = strings.TrimRight(string(runes[:max(n, 1)]), " .'")
	}
	return s
}

func truncate(s string, max int) string {
	if max <= 0 {
		max = 255
	}
	if len(s) > max {
		for max > 0 && !utf8.RuneStart(s[max]) { // not in the middle of a character
			max--
		}
		s = strings.TrimRight(s[:max], " .")
	}
	return s
}

// parenthetical is a "(Remix)" or "[Live]" with the space before it.
var parenthetical = regexp.MustCompile(`\s*[(\[][^()\[\]]*[)\]]`)

// Shorten makes s at most n characters long without cutting a word in half if
// it can help it. It first drops what is in parentheses, then cuts after the
// last whole word that still leaves at least half of n, else in the middle of
// a word. A cut at a word does not end with a small word that goes with the
// next ("of", "and"), nor with punctuation.
func Shorten(s string, n int) string {
	n = max(n, 1)
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	if bare := strings.TrimSpace(parenthetical.ReplaceAllString(s, "")); bare != "" {
		s = bare
		if utf8.RuneCountInString(s) <= n {
			return s
		}
	}
	runes := []rune(s)
	for i := n; i >= n/2 && i > 0; i-- { // the last space with room before it
		if runes[i] == ' ' {
			return withoutConnectors(strings.TrimRight(string(runes[:i]), dangling))
		}
	}
	return strings.TrimRight(string(runes[:n]), dangling)
}

const dangling = " .,-&:;'"

// connectors are the small words that hang on the next one: a name cut short
// does not end with them.
var connectors = map[string]bool{
	"a": true, "an": true, "the": true, "of": true, "and": true, "or": true, "to": true,
	"in": true, "on": true, "at": true, "for": true, "with": true, "by": true, "wa": true, "al": true, "el": true,
}

// withoutConnectors drops the connectors at the end of s, but never its only word.
func withoutConnectors(s string) string {
	words := strings.Fields(s)
	for len(words) > 1 && connectors[strings.ToLower(words[len(words)-1])] {
		words = words[:len(words)-1]
	}
	return strings.TrimRight(strings.Join(words, " "), dangling)
}
