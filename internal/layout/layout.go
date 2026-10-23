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

	"music-manager/internal/manifest"
)

const (
	ext         = ".mp3"
	inbox       = "_Inbox"
	playlistDir = "Playlists"
	playlistExt = ".m3u"
)

// Rules says how a target wants its paths made. The zero value keeps names
// as they are, cuts a path component at 255 bytes and does not shorten file
// names.
type Rules struct {
	// Fold turns a name into what the target keeps of it; nil keeps it as it is.
	Fold func(string) string
	// ComponentMax is the most bytes in one path component; 0 means 255.
	ComponentMax int
	// StemMax is the most characters in a file name before ".mp3"; 0 means no limit.
	StemMax int
}

// PathFor returns the slash-separated path of t relative to the library
// root: Artist/Album/NN - Title.mp3, Artist/Title.mp3 for tracks without an
// album, or _Inbox/<id> - <old name>.mp3 for tracks not yet identified. The
// file name is shortened to r.StemMax characters.
func PathFor(t manifest.Track, r Rules) string { return pathFor(t, "", r) }

// pathFor is PathFor with suffix (" (2)") put after the name, which is
// shortened to leave room for it.
func pathFor(t manifest.Track, suffix string, r Rules) string {
	room := r.StemMax - len(suffix)
	stem := func(s string, room int) string {
		if r.StemMax <= 0 {
			return s
		}
		return Shorten(s, room)
	}
	if t.Status != manifest.Identified {
		name := t.ID
		if hint := clean(t.Hint.Filename, r); hint != "Unknown" {
			name += " - " + hint
		}
		return path.Join(inbox, stem(name, room)+suffix+ext)
	}
	artist, title := clean(t.AsciiArtist, r), clean(t.AsciiTitle, r)
	if t.AsciiAlbum == "" {
		return path.Join(artist, stem(title, room)+suffix+ext)
	}
	number := ""
	if t.TrackNumber > 0 {
		width := max(2, len(strconv.Itoa(t.TrackTotal)))
		number = fmt.Sprintf("%0*d - ", width, t.TrackNumber)
	}
	return path.Join(artist, clean(t.AsciiAlbum, r), number+stem(title, room-len(number))+suffix+ext)
}

// PlaylistPath is where the playlist with this name is rendered:
// Playlists/<name>.m3u, with the name made safe for a file name.
func PlaylistPath(name string, r Rules) string {
	return path.Join(playlistDir, clean(name, r)+playlistExt)
}

// Assign returns the path of every track by ID. Tracks that would share a
// path (compared case-insensitively, as on FAT) get " (2)", " (3)", ...
// before the extension; the oldest track keeps the plain name.
func Assign(tracks []manifest.Track, r Rules) map[string]string {
	ordered := append([]manifest.Track(nil), tracks...)
	sort.Slice(ordered, func(i, j int) bool {
		if !ordered[i].AddedAt.Equal(ordered[j].AddedAt) {
			return ordered[i].AddedAt.Before(ordered[j].AddedAt)
		}
		return ordered[i].ID < ordered[j].ID
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

// clean makes s usable as one path component on any filesystem.
func clean(s string, r Rules) string {
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

func truncate(s string, max int) string {
	if max <= 0 {
		max = 255
	}
	if len(s) > max {
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
	"in": true, "on": true, "at": true, "for": true, "with": true, "by": true, "wa": true,
}

// withoutConnectors drops the connectors at the end of s, but never its only word.
func withoutConnectors(s string) string {
	words := strings.Fields(s)
	for len(words) > 1 && connectors[strings.ToLower(words[len(words)-1])] {
		words = words[:len(words)-1]
	}
	return strings.TrimRight(strings.Join(words, " "), dangling)
}
