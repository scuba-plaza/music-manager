package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"music-manager/internal/app"
	"music-manager/internal/device"
	"music-manager/internal/layout"
	"music-manager/internal/manifest"
)

// The pane on the right shows what the cursor is on, as the preview column of
// a file manager does: for a track its details, for anything that holds
// tracks what is inside, so that the tree can stay closed and still be read.

// peekTitle is the name of the pane for a row.
func peekTitle(r Row, ok bool) string {
	if !ok {
		return "Details"
	}
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

// peekLines draws the pane for a row that holds tracks, in a pane w wide and
// h high.
func (m Model) peekLines(r Row, w, h int) []string {
	head := []string{"", " " + styles.bold.Render(fit(r.Label, w-2)), rule(w), ""}
	var info []string // under the rule
	var list []string // the contents
	switch r.Kind {
	case kindGroup:
		if r.Key == playlistsKey {
			info = wrapped("Playlists are lists of tracks in an order of your own. Sync writes each one to the player as an .m3u file in the Playlists folder.", w)
			list = m.playlistsList(w)
		} else {
			info = wrapped("These tracks have not been identified yet, so they have no proper title or artist and sit in the _Inbox folder.", w)
			list = m.needsList(r, w)
		}
	case kindArtist:
		info, list = m.artistPeek(r, w)
	case kindAlbum:
		info, list = m.albumPeek(r, w)
	case kindPlaylist:
		info, list = m.playlistPeek(r, w)
	}
	lines := append(head, info...)
	if len(list) > 0 {
		lines = append(lines, "")
		lines = append(lines, cutList(list, h-len(lines))...)
	}
	return lines
}

// wrapped is text in the pane, wrapped to its width.
func wrapped(text string, w int) []string {
	var lines []string
	for _, l := range wrap(text, w-2) {
		lines = append(lines, " "+l)
	}
	return lines
}

// cutList keeps the first lines of a list that fit in room lines; if some do
// not, the last line says how many.
func cutList(list []string, room int) []string {
	if room <= 0 {
		return nil
	}
	if len(list) <= room {
		return list
	}
	return append(list[:max(room-1, 0)], " "+styles.dim.Render(fmt.Sprintf("… %d more", len(list)-room+1)))
}

// lengthOf is how long the tracks play together.
func lengthOf(ts []manifest.Track) time.Duration {
	var d time.Duration
	for _, t := range ts {
		d += t.PlayingLength()
	}
	return d
}

// listRow is a line of a list: the text on the left, and the columns at the
// right edge, each right-aligned in the width it is given.
func listRow(w int, left string, columns ...string) string {
	right := strings.Join(columns, "  ")
	room := max(w-2-ansi.StringWidth(right)-2, 1)
	return " " + fit(left, room) + "  " + right
}

// artistPeek describes an artist and lists its albums, then its singles.
func (m Model) artistPeek(r Row, w int) (info, list []string) {
	ts := m.tracksFor(r.IDs)
	type group struct {
		name   string
		tracks []manifest.Track
	}
	var albums []*group
	byName := map[string]*group{}
	var singles []manifest.Track
	for _, t := range ts {
		if t.OriginalAlbum == "" {
			singles = append(singles, t)
			continue
		}
		g, ok := byName[t.OriginalAlbum]
		if !ok {
			g = &group{name: t.OriginalAlbum}
			byName[t.OriginalAlbum] = g
			albums = append(albums, g)
		}
		g.tracks = append(g.tracks, t)
	}
	summary := plural(r.Count, "track")
	if len(albums) > 0 {
		summary += " · " + plural(len(albums), "album")
	}
	info = []string{" " + summary + " · " + clock(lengthOf(ts))}
	for _, g := range albums {
		list = append(list, listRow(w, g.name, fmt.Sprintf("%3d", len(g.tracks)), fmt.Sprintf("%5s", clock(lengthOf(g.tracks)))))
	}
	if len(singles) > 0 {
		list = append(list, listRow(w, styles.dim.Render("Singles"), fmt.Sprintf("%3d", len(singles)), fmt.Sprintf("%5s", clock(lengthOf(singles)))))
	}
	return info, list
}

// albumPeek describes an album and lists its tracks with their numbers.
func (m Model) albumPeek(r Row, w int) (info, list []string) {
	ts := m.tracksFor(r.IDs)
	artist := ""
	if len(ts) > 0 {
		artist, _, _ = app.Display(ts[0])
	}
	info = []string{" " + strings.TrimPrefix(artist+" · ", " · ") + plural(r.Count, "track") + " · " + clock(lengthOf(ts))}
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
		list = append(list, listRow(w, num+title, fmt.Sprintf("%5s", clock(t.PlayingLength()))))
	}
	return info, list
}

// playlistPeek describes a playlist and lists its tracks in the order they
// play, with their artists.
func (m Model) playlistPeek(r Row, w int) (info, list []string) {
	ts := m.tracksFor(r.IDs)
	info = []string{" " + plural(r.Count, "track") + " · " + clock(lengthOf(ts)) + " plays", ""}
	for i, l := range wrap(layout.PlaylistPath(r.Label, device.Generic.Layout()), w-4) {
		prefix := "   "
		if i == 0 {
			prefix = " → "
		}
		info = append(info, prefix+styles.accent.Render(l))
	}
	artistWidth := 0
	for _, t := range ts {
		artist, _, _ := app.Display(t)
		artistWidth = max(artistWidth, ansi.StringWidth(artist))
	}
	artistWidth = min(artistWidth, max((w-12)/3, 0))
	numberWidth := len(fmt.Sprint(len(ts)))
	for i, t := range ts {
		artist, title, _ := app.Display(t)
		left := fmt.Sprintf("%*d  %s", numberWidth, i+1, title)
		list = append(list, listRow(w, left, styles.dim.Render(fit(artist, artistWidth)), fmt.Sprintf("%5s", clock(t.PlayingLength()))))
	}
	return info, list
}

// playlistsList lists the playlists with how many tracks and how long.
func (m Model) playlistsList(w int) []string {
	var list []string
	for _, p := range m.playlists {
		ts := m.tracksFor(p.Tracks)
		list = append(list, listRow(w, p.Name, fmt.Sprintf("%3d", len(p.Tracks)), fmt.Sprintf("%5s", clock(lengthOf(ts)))))
	}
	return list
}

// needsList lists the tracks that are not identified, with what each lacks.
func (m Model) needsList(r Row, w int) []string {
	var list []string
	for _, t := range m.tracksFor(r.IDs) {
		var lacking []string
		if t.OriginalTitle == "" {
			lacking = append(lacking, "no title")
		}
		if t.OriginalArtist == "" {
			lacking = append(lacking, "no artist")
		}
		list = append(list, listRow(w, needsLabel(t), styles.dim.Render(strings.Join(lacking, ", ")), fmt.Sprintf("%5s", clock(t.PlayingLength()))))
	}
	return list
}
