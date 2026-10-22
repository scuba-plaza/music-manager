// Package layout decides where each track lives in the rendered music tree.
// The path is a pure function of the record, so it can never go stale.
package layout

import (
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"music-manager/internal/manifest"
	"music-manager/internal/translit"
)

const (
	ext          = ".mp3"
	inbox        = "_Inbox"
	playlistDir  = "Playlists"
	playlistExt  = ".m3u"
	maxComponent = 100 // bytes, well under the 255 most filesystems allow
)

// PathFor returns the slash-separated path of t relative to the library
// root: Artist/Album/NN - Title.mp3, Artist/Title.mp3 for tracks without an
// album, or _Inbox/<id> - <old name>.mp3 for tracks not yet identified.
func PathFor(t manifest.Track) string {
	if t.Status != manifest.Identified {
		name := t.ID
		if hint := clean(t.Hint.Filename); hint != "Unknown" {
			name += " - " + hint
		}
		return path.Join(inbox, truncate(name)+ext)
	}
	artist, title := clean(t.Artist), clean(t.Title)
	if t.Album == "" {
		return path.Join(artist, title+ext)
	}
	if t.TrackNumber > 0 {
		width := max(2, len(strconv.Itoa(t.TrackTotal)))
		title = fmt.Sprintf("%0*d - %s", width, t.TrackNumber, title)
	}
	return path.Join(artist, clean(t.Album), truncate(title)+ext)
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
			p = strings.TrimSuffix(PathFor(t), ext) + fmt.Sprintf(" (%d)", n) + ext
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
