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
	"music-manager/internal/translit"
)

const (
	ext          = ".mp3"
	inbox        = "_Inbox"
	playlistDir  = "Playlists"
	playlistExt  = ".m3u"
	maxComponent = 100 // bytes, well under the 255 most filesystems allow
	maxName      = 22  // characters of a file name before ".mp3": the player keeps 26 with it
)

// PathFor returns the slash-separated path of t relative to the library
// root: Artist/Album/NN - Title.mp3, Artist/Title.mp3 for tracks without an
// album, or _Inbox/<id> - <old name>.mp3 for tracks not yet identified. The
// file name is shortened to what the player keeps of it (see maxName).
func PathFor(t manifest.Track) string { return pathFor(t, "") }

// pathFor is PathFor with suffix (" (2)") put after the name, which is
// shortened to leave room for it.
func pathFor(t manifest.Track, suffix string) string {
	room := maxName - len(suffix)
	if t.Status != manifest.Identified {
		name := t.ID
		if hint := clean(t.Hint.Filename); hint != "Unknown" {
			name += " - " + hint
		}
		return path.Join(inbox, Shorten(name, room)+suffix+ext)
	}
	artist, title := clean(t.Artist), clean(t.Title)
	if t.Album == "" {
		return path.Join(artist, Shorten(title, room)+suffix+ext)
	}
	number := ""
	if t.TrackNumber > 0 {
		width := max(2, len(strconv.Itoa(t.TrackTotal)))
		number = fmt.Sprintf("%0*d - ", width, t.TrackNumber)
	}
	return path.Join(artist, clean(t.Album), number+Shorten(title, room-len(number))+suffix+ext)
}

// PlaylistPath is where the playlist with this name is rendered:
// Playlists/<name>.m3u, with the name made safe for a file name.
func PlaylistPath(name string) string {
	return path.Join(playlistDir, clean(name)+playlistExt)
}

// Assign returns the path of every track by ID. Tracks that would share a
// path (compared case-insensitively, as on FAT) get " (2)", " (3)", ...
// before the extension; the oldest track keeps the plain name.
func Assign(tracks []manifest.Track) map[string]string {
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
		p := PathFor(t)
		for n := 2; taken[strings.ToLower(p)]; n++ {
			p = pathFor(t, fmt.Sprintf(" (%d)", n))
		}
		taken[strings.ToLower(p)] = true
		out[t.ID] = p
	}
	return out
}

// clean makes s usable as one path component on any filesystem.
func clean(s string) string {
	s = translit.ToASCII(s)
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < 0x20 || r == 0x7f {
			return '-'
		}
		return r
	}, s)
	s = strings.Trim(truncate(s), " .'")
	if strings.Trim(s, "-") == "" {
		return "Unknown"
	}
	return s
}

func truncate(s string) string {
	if len(s) > maxComponent {
		s = strings.TrimRight(s[:maxComponent], " .")
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
