// Package browse is what a screen needs to show the library: the tree of
// artists, albums and tracks, the fields of a track, and what is said about
// an album or a playlist. It works out text and order, and has no idea how
// it is drawn: the results are plain values that marshal to JSON.
package browse

import (
	"fmt"
	"strings"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/timecode"
)

// Snapshot is the library as the views are made from it.
type Snapshot struct {
	Tracks    []manifest.Track
	Playlists []manifest.Playlist
}

// Track returns the track with the given ID.
func (s Snapshot) Track(id string) (manifest.Track, bool) {
	for _, t := range s.Tracks {
		if t.ID == id {
			return t, true
		}
	}
	return manifest.Track{}, false
}

// tracksFor returns the tracks with the given IDs, in that order.
func (s Snapshot) tracksFor(ids []string) []manifest.Track {
	byID := make(map[string]manifest.Track, len(s.Tracks))
	for _, t := range s.Tracks {
		byID[t.ID] = t
	}
	out := make([]manifest.Track, 0, len(ids))
	for _, id := range ids {
		if t, ok := byID[id]; ok {
			out = append(out, t)
		}
	}
	return out
}

// Options say what the library view shows.
type Options struct {
	Filter string
	// Sort puts the tracks in one flat list in order of this key; empty is the tree.
	Sort    app.SortKey
	Reverse bool
}

// RowView is one line of the tree or of the sorted list. The tree comes flat,
// every row open: Depth and Parent say where a row belongs, so that a screen
// can fold it without asking again.
type RowView struct {
	Kind     string `json:"kind"` // group, artist, album, track, playlist
	Depth    int    `json:"depth"`
	Key      string `json:"key"`
	Parent   string `json:"parent,omitempty"`
	Label    string `json:"label"`
	Artist   string `json:"artist,omitempty"`
	Album    string `json:"album,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Num      string `json:"num,omitempty"`
	Right    string `json:"right,omitempty"`
	Count    int    `json:"count,omitempty"`
	TrackID  string `json:"trackId,omitempty"`
	Length   string `json:"length,omitempty"`
	Editable bool   `json:"editable,omitempty"`
}

// SortChoice is one entry of the menu of ways to order the library.
type SortChoice struct {
	Key      string `json:"key"` // empty is the tree
	Label    string `json:"label"`
	Forward  string `json:"forward"`
	Backward string `json:"backward"`
}

// SortView says how the library is ordered now, and what else it could be.
type SortView struct {
	Key        string       `json:"key"`
	Reverse    bool         `json:"reverse"`
	Descending bool         `json:"descending"`
	Title      string       `json:"title"`
	Choices    []SortChoice `json:"choices"`
}

// PlaylistView is a playlist as the pickers list it.
type PlaylistView struct {
	Name   string `json:"name"`
	Latin  string `json:"latin,omitempty"`
	Count  int    `json:"count"`
	Length string `json:"length"`
}

// LibraryView is everything the left side of the screen needs.
type LibraryView struct {
	Rows      []RowView      `json:"rows"`
	Tracks    int            `json:"tracks"`   // in the whole library
	NeedWork  int            `json:"needWork"` // not identified, in the whole library
	Shown     int            `json:"shown"`    // tracks in the rows
	Sort      SortView       `json:"sort"`
	Playlists []PlaylistView `json:"playlists"`
}

// Library lays out the library for the given options.
func Library(s Snapshot, o Options) LibraryView {
	v := LibraryView{Tracks: len(s.Tracks), Sort: sortView(o.Sort, o.Reverse), Rows: []RowView{}, Playlists: []PlaylistView{}}
	for _, t := range s.Tracks {
		if t.Status != manifest.Identified {
			v.NeedWork++
		}
	}
	var rows []Row
	if o.Sort == "" {
		rows = buildRows(s.Tracks, s.Playlists, o.Filter, func(string) bool { return true })
	} else {
		rows = buildSortedRows(s.Tracks, o.Filter, o.Sort, o.Reverse)
	}
	editable := func(r Row) bool {
		return r.Kind == kindPlaylist || r.Kind == kindArtist || (r.Kind == kindAlbum && r.Album != "")
	}
	for _, r := range rows {
		rv := RowView{
			Kind: r.Kind.String(), Depth: r.Depth, Key: r.Key, Parent: r.Parent,
			Label: r.Label, Artist: r.Artist, Album: r.Album, Detail: r.Detail, Num: r.Num, Right: r.Right,
			Count: r.Count, Editable: editable(r),
		}
		if r.Kind == kindTrack {
			rv.TrackID = r.Track.ID
			rv.Length = timecode.Clock(r.Track.PlayingLength())
			v.Shown++
		}
		v.Rows = append(v.Rows, rv)
	}
	for _, p := range s.Playlists {
		v.Playlists = append(v.Playlists, PlaylistView{
			Name: p.Name, Latin: p.NameLatin, Count: len(p.Tracks),
			Length: timecode.Clock(lengthOf(s.tracksFor(p.Tracks))),
		})
	}
	return v
}

func (k rowKind) String() string {
	return [...]string{"group", "artist", "album", "track", "playlist"}[k]
}

// buildSortedRows lists the tracks matching filter in one flat list, put in
// order by key.
func buildSortedRows(tracks []manifest.Track, filter string, key app.SortKey, reverse bool) []Row {
	if strings.TrimSpace(filter) != "" {
		tracks = (&manifest.Manifest{Tracks: tracks}).Search(filter)
	}
	var rows []Row
	for _, t := range app.SortTracks(tracks, key, reverse) {
		artist, _, album := app.Display(t)
		detail := artist
		if key == app.SortAlbum {
			detail = album
		}
		rows = append(rows, Row{
			Kind: kindTrack, Key: trackKey(t.ID), Label: needsLabel(t), Detail: detail,
			Right: sortedValue(t, key), Track: t,
		})
	}
	return rows
}

// sortedValue is the value shown beside a track in a list sorted by key, for
// the keys whose value is not already on the line.
func sortedValue(t manifest.Track, key app.SortKey) string {
	switch key {
	case app.SortReleased, app.SortAdded, app.SortEdited, app.SortTrack:
		if v := app.SortValue(t, key); v != "" {
			return v
		}
		return "—"
	}
	return ""
}

func sortView(key app.SortKey, reverse bool) SortView {
	if key == "" {
		reverse = false
	}
	v := SortView{Key: string(key), Reverse: reverse, Descending: key.IsDate() != reverse, Title: "Library"}
	if key != "" {
		arrow := "▲"
		if v.Descending {
			arrow = "▼"
		}
		v.Title = "Library · " + key.Label() + " " + arrow
	}
	for _, c := range append([]app.SortKey{""}, app.SortKeys...) {
		label := "Tree"
		if c != "" {
			label = c.Label()
		}
		f, b := direction(c)
		v.Choices = append(v.Choices, SortChoice{Key: string(c), Label: label, Forward: f, Backward: b})
	}
	return v
}

// direction says in words what each way round of a sort key looks like.
func direction(k app.SortKey) (forward, backward string) {
	switch k {
	case app.SortArtist, app.SortAlbum, app.SortTitle:
		return "A → Z", "Z → A"
	case app.SortTrack:
		return "1 → 9", "9 → 1"
	case app.SortLength:
		return "shortest first", "longest first"
	case app.SortReleased, app.SortAdded, app.SortEdited:
		return "newest first", "oldest first"
	}
	return "artist → album → track", "artist → album → track"
}

// plural writes a count with its noun, as "1 track" or "3 tracks".
func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	suffix := "s"
	for _, end := range []string{"ch", "sh", "s", "x"} {
		if strings.HasSuffix(word, end) {
			suffix = "es"
		}
	}
	return fmt.Sprintf("%d %s%s", n, word, suffix)
}
