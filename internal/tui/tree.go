package tui

import (
	"sort"
	"strconv"
	"strings"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/translit"
)

// The tree on the left of the screen is derived data: it is rebuilt from the
// records whenever they, the filter or the open/closed state change. It has
// no state of its own, which is what makes it easy to test.

type rowKind int

const (
	kindGroup    rowKind = iota // a special group such as "Needs work"
	kindArtist                  // an artist
	kindAlbum                   // an album, or the pseudo-album "Singles"
	kindTrack                   // one track
	kindPlaylist                // a playlist, which opens into its tracks
)

// keyNeedsWork is the key of the group of tracks that are not identified.
const keyNeedsWork = "needs"

func artistKey(artist string) string          { return "artist\x1f" + artist }
func albumRowKey(artist, album string) string { return "album\x1f" + artist + "\x1f" + album }
func singlesKey(artist string) string         { return "singles\x1f" + artist }
func trackKey(id string) string               { return "track\x1f" + id }

// playlistsKey is the key of the group that holds the playlists.
const playlistsKey = "playlists"

// playlistRowPrefix starts the key of every playlist row.
const playlistRowPrefix = "playlist\x1f"

func playlistRowKey(name string) string { return playlistRowPrefix + name }

// playlistTrackKey identifies a track as it appears in one playlist; the
// same track in another playlist, or in the tree of artists, is another row.
func playlistTrackKey(name, id string) string { return "pltrack\x1f" + name + "\x1f" + id }

// Row is one line of the tree.
type Row struct {
	Kind   rowKind
	Depth  int
	Key    string // identifies the row across rebuilds
	Parent string // the key of the row above it in the tree, or ""

	Label  string   // the artist, album or title
	Detail string   // secondary text, e.g. the artist of a track nobody has identified
	Num    string   // a track's number, padded to its album's width, or ""
	Right  string   // shown at the right of a track in place of its length, or ""
	Count  int      // how many tracks are beneath a group, artist or album
	IDs    []string // which tracks, in the order they are listed

	Expanded bool
	Track    manifest.Track // for track rows
}

// buildRows lays out the visible rows. A non-blank filter (same syntax as
// "mm ls") keeps only the matching tracks and opens everything they sit
// under; otherwise only the rows in expanded are open.
func buildRows(tracks []manifest.Track, filter string, expanded map[string]bool) []Row {
	return buildRowsWith(tracks, nil, filter, expanded)
}

// buildRowsWith is buildRows with the playlists, which form a group of their
// own ahead of the artists. A filter keeps only the tracks that match, in
// the playlists too, and the playlists that still have any.
func buildRowsWith(tracks []manifest.Track, playlists []manifest.Playlist, filter string, expanded map[string]bool) []Row {
	filtering := strings.TrimSpace(filter) != ""
	all := tracks
	var matched map[string]bool
	if filtering {
		tracks = (&manifest.Manifest{Tracks: tracks}).Search(filter)
		matched = map[string]bool{}
		for _, t := range tracks {
			matched[t.ID] = true
		}
	}
	open := func(key string) bool { return filtering || expanded[key] }

	var needs []manifest.Track
	byArtist := map[string][]manifest.Track{}
	for _, t := range tracks {
		if t.Status != manifest.Identified {
			needs = append(needs, t)
			continue
		}
		byArtist[t.OriginalArtist] = append(byArtist[t.OriginalArtist], t)
	}

	rows := playlistRows(all, playlists, matched, open)
	if len(needs) > 0 {
		isOpen := open(keyNeedsWork)
		sort.SliceStable(needs, func(i, j int) bool {
			return translit.SortKey(needsLabel(needs[i])) < translit.SortKey(needsLabel(needs[j]))
		})
		rows = append(rows, Row{Kind: kindGroup, Key: keyNeedsWork, Label: "Needs work", Count: len(needs), IDs: idsOf(needs), Expanded: isOpen})
		if isOpen {
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

		byAlbum := map[string][]manifest.Track{}
		for _, t := range ts {
			byAlbum[t.OriginalAlbum] = append(byAlbum[t.OriginalAlbum], t)
		}
		var ids []string
		albums := albumOrder(byAlbum)
		for _, album := range albums {
			sortTracks(byAlbum[album])
			ids = append(ids, idsOf(byAlbum[album])...)
		}
		rows = append(rows, Row{Kind: kindArtist, Key: akey, Label: artist, Count: len(ts), IDs: ids, Expanded: isOpen})
		if !isOpen {
			continue
		}

		for _, album := range albums {
			at := byAlbum[album]
			bkey, label := albumRowKey(artist, album), album
			if album == "" {
				bkey, label = singlesKey(artist), "Singles"
			}
			albumOpen := open(bkey)
			rows = append(rows, Row{Kind: kindAlbum, Depth: 1, Key: bkey, Parent: akey, Label: label, Count: len(at), IDs: idsOf(at), Expanded: albumOpen})
			if !albumOpen {
				continue
			}
			width := numberWidth(at)
			for _, t := range at {
				num := ""
				if album != "" && t.TrackNumber > 0 {
					num = padNumber(t.TrackNumber, width)
				}
				rows = append(rows, Row{Kind: kindTrack, Depth: 2, Key: trackKey(t.ID), Parent: bkey, Label: t.OriginalTitle, Num: num, Track: t})
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

func idsOf(ts []manifest.Track) []string {
	ids := make([]string, len(ts))
	for i, t := range ts {
		ids[i] = t.ID
	}
	return ids
}

// sortedNames returns the keys of m in the order of their Latin forms, ignoring case.
func sortedNames[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if li, lj := translit.SortKey(names[i]), translit.SortKey(names[j]); li != lj {
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
		if la, lb := translit.SortKey(a.OriginalTitle), translit.SortKey(b.OriginalTitle); la != lb {
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
	if t.OriginalAlbum == "" {
		return []string{artistKey(t.OriginalArtist), singlesKey(t.OriginalArtist)}
	}
	return []string{artistKey(t.OriginalArtist), albumRowKey(t.OriginalArtist, t.OriginalAlbum)}
}

// playlistRows is the group of playlists with those open shown in full. When
// matched is not nil, only those tracks are shown, and only the playlists
// that have some.
func playlistRows(tracks []manifest.Track, playlists []manifest.Playlist, matched map[string]bool, open func(string) bool) []Row {
	byID := make(map[string]manifest.Track, len(tracks))
	for _, t := range tracks {
		byID[t.ID] = t
	}
	type shown struct {
		name string
		ts   []manifest.Track
	}
	var pls []shown
	var groupIDs []string
	seen := map[string]bool{}
	for _, p := range playlists {
		s := shown{name: p.Name}
		for _, id := range p.Tracks {
			if t, ok := byID[id]; ok && (matched == nil || matched[id]) {
				s.ts = append(s.ts, t)
				if !seen[id] {
					seen[id] = true
					groupIDs = append(groupIDs, id)
				}
			}
		}
		if matched == nil || len(s.ts) > 0 {
			pls = append(pls, s)
		}
	}
	if len(pls) == 0 {
		return nil
	}

	groupOpen := open(playlistsKey)
	rows := []Row{{Kind: kindGroup, Key: playlistsKey, Label: "Playlists", Count: len(pls), IDs: groupIDs, Expanded: groupOpen}}
	if !groupOpen {
		return rows
	}
	for _, p := range pls {
		pkey := playlistRowKey(p.name)
		plOpen := open(pkey)
		rows = append(rows, Row{Kind: kindPlaylist, Depth: 1, Key: pkey, Parent: playlistsKey, Label: p.name, Count: len(p.ts), IDs: idsOf(p.ts), Expanded: plOpen})
		if !plOpen {
			continue
		}
		width := max(2, len(strconv.Itoa(len(p.ts))))
		for i, t := range p.ts {
			artist, _, _ := app.Display(t)
			rows = append(rows, Row{
				Kind: kindTrack, Depth: 2, Key: playlistTrackKey(p.name, t.ID), Parent: pkey,
				Label: t.OriginalTitle, Detail: artist, Num: padNumber(i+1, width), Track: t,
			})
		}
	}
	return rows
}
