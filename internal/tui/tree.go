package tui

import (
	"sort"
	"strconv"
	"strings"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

// The tree on the left of the screen is derived data: it is rebuilt from the
// records whenever they, the filter or the open/closed state change. It has
// no state of its own, which is what makes it easy to test.

type rowKind int

const (
	kindGroup  rowKind = iota // a special group such as "Needs work"
	kindArtist                // an artist
	kindAlbum                 // an album, or the pseudo-album "Singles"
	kindTrack                 // one track
)

// keyNeedsWork is the key of the group of tracks that are not identified.
const keyNeedsWork = "needs"

func artistKey(artist string) string          { return "artist\x1f" + artist }
func albumRowKey(artist, album string) string { return "album\x1f" + artist + "\x1f" + album }
func singlesKey(artist string) string         { return "singles\x1f" + artist }
func trackKey(id string) string               { return "track\x1f" + id }

// Row is one line of the tree.
type Row struct {
	Kind   rowKind
	Depth  int
	Key    string // identifies the row across rebuilds
	Parent string // the key of the row above it in the tree, or ""

	Label  string // the artist, album or title
	Detail string // secondary text, e.g. the artist of a track nobody has identified
	Num    string // a track's number, padded to its album's width, or ""
	Count  int    // how many tracks are beneath a group, artist or album

	Expanded bool
	Track    manifest.Track // for track rows
}

// buildRows lays out the visible rows. A non-blank filter (same syntax as
// "mm ls") keeps only the matching tracks and opens everything they sit
// under; otherwise only the rows in expanded are open.
func buildRows(tracks []manifest.Track, filter string, expanded map[string]bool) []Row {
	filtering := strings.TrimSpace(filter) != ""
	if filtering {
		tracks = (&manifest.Manifest{Tracks: tracks}).Search(filter)
	}
	open := func(key string) bool { return filtering || expanded[key] }

	var needs []manifest.Track
	byArtist := map[string][]manifest.Track{}
	for _, t := range tracks {
		if t.Status != manifest.Identified {
			needs = append(needs, t)
			continue
		}
		byArtist[t.Artist] = append(byArtist[t.Artist], t)
	}

	var rows []Row
	if len(needs) > 0 {
		isOpen := open(keyNeedsWork)
		rows = append(rows, Row{Kind: kindGroup, Key: keyNeedsWork, Label: "Needs work", Count: len(needs), Expanded: isOpen})
		if isOpen {
			sort.SliceStable(needs, func(i, j int) bool { return lower(needsLabel(needs[i])) < lower(needsLabel(needs[j])) })
			for _, t := range needs {
				artist, _, _ := app.Display(t)
				rows = append(rows, Row{
					Kind: kindTrack, Depth: 1, Key: trackKey(t.ID), Parent: keyNeedsWork,
					Label: needsLabel(t), Detail: artist, Track: t,
				})
			}
		}
	}

	for _, artist := range sortedNames(byArtist) {
		ts := byArtist[artist]
		akey := artistKey(artist)
		isOpen := open(akey)
		rows = append(rows, Row{Kind: kindArtist, Key: akey, Label: artist, Count: len(ts), Expanded: isOpen})
		if !isOpen {
			continue
		}

		byAlbum := map[string][]manifest.Track{}
		for _, t := range ts {
			byAlbum[t.Album] = append(byAlbum[t.Album], t)
		}
		for _, album := range albumOrder(byAlbum) {
			at := byAlbum[album]
			bkey, label := albumRowKey(artist, album), album
			if album == "" {
				bkey, label = singlesKey(artist), "Singles"
			}
			albumOpen := open(bkey)
			rows = append(rows, Row{Kind: kindAlbum, Depth: 1, Key: bkey, Parent: akey, Label: label, Count: len(at), Expanded: albumOpen})
			if !albumOpen {
				continue
			}
			sortTracks(at)
			width := numberWidth(at)
			for _, t := range at {
				num := ""
				if album != "" && t.TrackNumber > 0 {
					num = padNumber(t.TrackNumber, width)
				}
				rows = append(rows, Row{Kind: kindTrack, Depth: 2, Key: trackKey(t.ID), Parent: bkey, Label: t.Title, Num: num, Track: t})
			}
		}
	}
	return rows
}

// needsLabel is how a track that is not identified is named: by what the
// file said about itself, since it has no title of its own.
func needsLabel(t manifest.Track) string {
	if _, title, _ := app.Display(t); title != "" {
		return title
	}
	return "?"
}

func lower(s string) string { return strings.ToLower(s) }

// sortedNames returns the keys of m, ignoring case.
func sortedNames[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if li, lj := lower(names[i]), lower(names[j]); li != lj {
			return li < lj
		}
		return names[i] < names[j]
	})
	return names
}

// albumOrder lists albums alphabetically, with the singles ("") last.
func albumOrder(byAlbum map[string][]manifest.Track) []string {
	names := sortedNames(byAlbum)
	if len(names) > 0 && names[0] == "" { // "" sorts first; move it to the end
		names = append(names[1:], "")
	}
	return names
}

// sortTracks orders an album's tracks by number (unnumbered ones last), then title.
func sortTracks(ts []manifest.Track) {
	sort.SliceStable(ts, func(i, j int) bool {
		a, b := ts[i], ts[j]
		if (a.TrackNumber > 0) != (b.TrackNumber > 0) {
			return a.TrackNumber > 0
		}
		if a.TrackNumber != b.TrackNumber {
			return a.TrackNumber < b.TrackNumber
		}
		if la, lb := lower(a.Title), lower(b.Title); la != lb {
			return la < lb
		}
		return a.ID < b.ID
	})
}

// numberWidth is how many digits an album's track numbers are padded to.
func numberWidth(ts []manifest.Track) int {
	most := 0
	for _, t := range ts {
		most = max(most, t.TrackNumber)
	}
	return max(2, len(strconv.Itoa(most)))
}

func padNumber(n, width int) string {
	s := strconv.Itoa(n)
	return strings.Repeat("0", max(width-len(s), 0)) + s
}

// ancestorKeys lists the rows that must be open for t to be visible.
func ancestorKeys(t manifest.Track) []string {
	if t.Status != manifest.Identified {
		return []string{keyNeedsWork}
	}
	if t.Album == "" {
		return []string{artistKey(t.Artist), singlesKey(t.Artist)}
	}
	return []string{artistKey(t.Artist), albumRowKey(t.Artist, t.Album)}
}
