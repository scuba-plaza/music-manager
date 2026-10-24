package browse

import (
	"fmt"
	"strings"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/device"
	"music-manager/internal/manifest"
	"music-manager/internal/timecode"
)

// ListRow is a line of the list under a peek: what is inside the row.
type ListRow struct {
	// Key is the row of the tree this line is, so that it can be gone to.
	Key  string   `json:"key,omitempty"`
	Left string   `json:"left"`
	Cols []string `json:"cols,omitempty"`
	Dim  bool     `json:"dim,omitempty"` // the left text is dim
}

// PeekView is what the inspector shows for a row that is not a track: a
// summary and the list of what is inside.
type PeekView struct {
	Kind     string    `json:"kind"`
	Title    string    `json:"title"`
	Label    string    `json:"label"`
	Editable bool      `json:"editable"`
	Info     []string  `json:"info"`
	Path     string    `json:"path,omitempty"` // where a playlist is written
	List     []ListRow `json:"list"`
}

// Peek describes the row with the given key as the library view for the same
// filter lays it out.
func Peek(s Snapshot, key, filter string) (PeekView, bool) {
	var r Row
	found := false
	for _, row := range buildRows(s.Tracks, s.Playlists, filter, func(string) bool { return true }) {
		if row.Key == key {
			r, found = row, true
			break
		}
	}
	if !found || r.Kind == kindTrack {
		return PeekView{}, false
	}
	v := PeekView{Kind: r.Kind.String(), Label: r.Label, Title: peekTitle(r), Info: []string{}, List: []ListRow{}}
	v.Editable = r.Kind == kindPlaylist || r.Kind == kindArtist || (r.Kind == kindAlbum && r.Album != "")
	switch r.Kind {
	case kindGroup:
		if r.Key == playlistsKey {
			v.Info = []string{"Playlists are lists of tracks in an order of your own. Sync writes each one as an .m3u8 file in the Playlists folder (an .m3u for the Wolfgang)."}
			v.List = playlistsList(s)
		} else {
			v.Info = []string{"These tracks have not been identified yet, so they have no proper title or artist and sit in the _Inbox folder."}
			v.List = needsList(s, r)
		}
	case kindArtist:
		v.Info, v.List = artistPeek(s, r)
	case kindAlbum:
		v.Info, v.List = albumPeek(s, r)
	case kindPlaylist:
		v.Info, v.List = playlistPeek(s, r)
		v.Path = device.Generic.PlaylistFile(manifest.Playlist{Name: r.Label})
	}
	return v, true
}

func peekTitle(r Row) string {
	switch r.Kind {
	case kindGroup:
		return r.Label
	case kindArtist:
		return "Artist"
	case kindAlbum:
		return "Album"
	case kindPlaylist:
		return "Playlist"
	}
	return "Details"
}

func lengthOf(ts []manifest.Track) time.Duration {
	var d time.Duration
	for _, t := range ts {
		d += t.PlayingLength()
	}
	return d
}

func artistPeek(s Snapshot, r Row) (info []string, list []ListRow) {
	ts := s.tracksFor(r.IDs)
	type group struct {
		name   string
		tracks []manifest.Track
	}
	var albums []*group
	byName := map[string]*group{}
	var singles []manifest.Track
	for _, t := range ts {
		if t.Album == "" {
			singles = append(singles, t)
			continue
		}
		g, ok := byName[t.Album]
		if !ok {
			g = &group{name: t.Album}
			byName[t.Album] = g
			albums = append(albums, g)
		}
		g.tracks = append(g.tracks, t)
	}
	summary := plural(r.Count, "track")
	if len(albums) > 0 {
		summary += " · " + plural(len(albums), "album")
	}
	info = []string{summary + " · " + timecode.Clock(lengthOf(ts))}
	for _, g := range albums {
		list = append(list, ListRow{
			Key: albumRowKey(r.Artist, g.name), Left: g.name,
			Cols: []string{fmt.Sprint(len(g.tracks)), timecode.Clock(lengthOf(g.tracks))},
		})
	}
	if len(singles) > 0 {
		list = append(list, ListRow{
			Key: singlesKey(r.Artist), Left: "Singles", Dim: true,
			Cols: []string{fmt.Sprint(len(singles)), timecode.Clock(lengthOf(singles))},
		})
	}
	return info, list
}

func albumPeek(s Snapshot, r Row) (info []string, list []ListRow) {
	ts := s.tracksFor(r.IDs)
	artist := ""
	if len(ts) > 0 {
		artist, _, _ = app.Display(ts[0])
	}
	info = []string{strings.TrimPrefix(artist+" · ", " · ") + plural(r.Count, "track") + " · " + timecode.Clock(lengthOf(ts))}
	numberWidth := 0
	for _, t := range ts {
		numberWidth = max(numberWidth, len(numberText(t.TrackNumber)))
	}
	for _, t := range ts {
		_, title, _ := app.Display(t)
		num := ""
		if numberWidth > 0 {
			num = fmt.Sprintf("%*s  ", numberWidth, numberText(t.TrackNumber))
		}
		list = append(list, ListRow{Key: trackKey(t.ID), Left: num + title, Cols: []string{timecode.Clock(t.PlayingLength())}})
	}
	return info, list
}

func playlistPeek(s Snapshot, r Row) (info []string, list []ListRow) {
	ts := s.tracksFor(r.IDs)
	info = []string{plural(r.Count, "track") + " · " + timecode.Clock(lengthOf(ts)) + " plays"}
	numberWidth := len(fmt.Sprint(len(ts)))
	for i, t := range ts {
		artist, title, _ := app.Display(t)
		list = append(list, ListRow{
			Key:  playlistTrackKey(r.Label, t.ID),
			Left: fmt.Sprintf("%*d  %s", numberWidth, i+1, title),
			Cols: []string{artist, timecode.Clock(t.PlayingLength())},
		})
	}
	return info, list
}

func playlistsList(s Snapshot) []ListRow {
	var list []ListRow
	for _, p := range s.Playlists {
		list = append(list, ListRow{
			Key: playlistRowKey(p.Name), Left: p.Name,
			Cols: []string{fmt.Sprint(len(p.Tracks)), timecode.Clock(lengthOf(s.tracksFor(p.Tracks)))},
		})
	}
	return list
}

func needsList(s Snapshot, r Row) []ListRow {
	var list []ListRow
	for _, t := range s.tracksFor(r.IDs) {
		var lacking []string
		if t.Title == "" {
			lacking = append(lacking, "no title")
		}
		if t.Artist == "" {
			lacking = append(lacking, "no artist")
		}
		list = append(list, ListRow{
			Key: trackKey(t.ID), Left: needsLabel(t),
			Cols: []string{strings.Join(lacking, ", "), timecode.Clock(t.PlayingLength())},
		})
	}
	return list
}
