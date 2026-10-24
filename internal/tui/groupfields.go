package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/manifest"
)

// An artist, an album and a playlist have fields in the right pane, as a track
// has. What is edited there is edited on the whole of them: all the tracks of
// the artist or the album, whatever the filter shows.

// group is what an artist, album or playlist row stands for.
type group struct {
	kind     rowKind
	name     string            // the artist, album or playlist
	artist   string            // the artist of an album
	tracks   []manifest.Track  // the tracks of an artist or album
	playlist manifest.Playlist // the playlist, for a playlist
}

// editableRow reports whether the row has fields to edit: a track, an artist,
// an album (the singles of an artist are not one) or a playlist.
func editableRow(r Row) bool {
	switch r.Kind {
	case kindTrack, kindArtist, kindPlaylist:
		return true
	case kindAlbum:
		return r.Album != ""
	}
	return false
}

// groupOf is the whole group the row stands for, found among all the tracks
// and not only those a filter keeps.
func (m Model) groupOf(r Row) (group, bool) {
	g := group{kind: r.Kind, name: r.Label, artist: r.Artist}
	switch r.Kind {
	case kindArtist:
		for _, t := range m.tracks {
			if t.Status == manifest.Identified && t.Artist == r.Artist {
				g.tracks = append(g.tracks, t)
			}
		}
	case kindAlbum:
		if r.Album == "" {
			return g, false
		}
		for _, t := range m.tracks {
			if t.Status == manifest.Identified && t.Artist == r.Artist && t.Album == r.Album {
				g.tracks = append(g.tracks, t)
			}
		}
	case kindPlaylist:
		p, ok := m.playlistNamed(r.Label)
		g.playlist = p
		return g, ok
	default:
		return g, false
	}
	return g, len(g.tracks) > 0
}

// groupFields are the fields of a kind of group, in the order they are listed.
func groupFields(kind rowKind) []fieldKind {
	switch kind {
	case kindArtist:
		return []fieldKind{fArtist, fLatinArtist, fGenre, fLatinGenre}
	case kindAlbum:
		return []fieldKind{fAlbum, fLatinAlbum, fReleased, fGenre, fLatinGenre, fAlbumOf}
	case kindPlaylist:
		return []fieldKind{fPlaylistName, fPlaylistLatin}
	}
	return nil
}

// curField is the field the cursor is on in the pane: a field of the track
// selected, or of the artist, album or playlist.
func (m Model) curField() fieldKind {
	if r, ok := m.selectedRow(); ok && r.Kind != kindTrack {
		if fs := groupFields(r.Kind); len(fs) > 0 {
			return fs[min(m.gfield, len(fs)-1)]
		}
	}
	return trackFields[m.field]
}

// groupLabelWidth is the width of the label column in the pane of a group,
// including the mark of the field the cursor is on.
const groupLabelWidth = 18

// groupLabel is how a field is named in the pane of a group.
func groupLabel(f fieldKind) string {
	switch f {
	case fArtist, fAlbum, fPlaylistName:
		return "Name"
	case fLatinArtist, fLatinAlbum, fPlaylistLatin:
		return "Latin name"
	case fLatinGenre:
		return "Latin genre"
	case fAlbumOf:
		return "Tracks on album"
	}
	return f.label()
}

// groupValue is what the field shows for the group: the value all its tracks
// have, or mixed if they differ. For a Latin form, byHand says whether it was
// set by hand on any of the tracks.
func groupValue(g group, f fieldKind) (text string, mixed, byHand bool) {
	if g.kind == kindPlaylist {
		if f == fPlaylistLatin {
			return g.playlist.LatinName(), false, g.playlist.NameLatin != ""
		}
		return g.playlist.Name, false, false
	}
	for i, t := range g.tracks {
		v := trackValueOf(t, f)
		if i > 0 && v != text {
			mixed = true
		}
		text = v
		if f.isLatin() && handSet(t, f) {
			byHand = true
		}
	}
	if mixed {
		return "", true, byHand
	}
	return text, false, byHand
}

// trackValueOf is what one track has for a field of a group.
func trackValueOf(t manifest.Track, f fieldKind) string {
	switch f {
	case fArtist:
		return t.Artist
	case fAlbum:
		return t.Album
	case fGenre:
		return t.Genre
	case fReleased:
		return t.Released
	case fAlbumOf:
		return numberText(t.TrackTotal)
	case fCover:
		return t.CoverSHA256
	}
	form, _ := latinOf(t, f)
	return form
}

// groupNote is the line under the fields that says what an edit reaches.
func groupNote(g group) string {
	if g.kind == kindPlaylist {
		return ""
	}
	if len(g.tracks) == 1 {
		return "its only track · one undo step"
	}
	return "all " + plural(len(g.tracks), "track") + " · one undo step"
}

// groupBlock is the fields of a group, with the note under them, drawn for a
// pane w wide.
func (m Model) groupBlock(g group, w int) []string {
	var lines []string
	for i, f := range groupFields(g.kind) {
		lines = append(lines, m.groupFieldLines(g, f, m.focus == paneInspector && i == m.gfield, w)...)
	}
	if note := groupNote(g); note != "" {
		lines = append(lines, "  "+styles.dim.Render(fit(note, w-3)))
	}
	return lines
}

// groupFieldLines is one field of a group: its label and its value.
func (m Model) groupFieldLines(g group, f fieldKind, current bool, w int) []string {
	prefix, _, vw := fieldPrefix(groupLabel(f), current, groupLabelWidth, w)
	text, mixed, byHand := groupValue(g, f)
	switch {
	case mixed:
		return []string{prefix + styles.dim.Render("(mixed)")}
	case text == "":
		return []string{prefix + styles.dim.Render("—")}
	case (f.isLatin() || f == fPlaylistLatin) && !byHand:
		return []string{prefix + styles.dim.Render(fit(text, vw))}
	}
	return []string{prefix + fit(text, vw)}
}

// updateGroupFields is a key pressed while the cursor is in the fields of an
// artist, an album or a playlist.
func (m Model) updateGroupFields(msg tea.KeyMsg, r Row) (tea.Model, tea.Cmd) {
	k := m.keys
	switch {
	case key.Matches(msg, k.Back), key.Matches(msg, k.Switch):
		m.focus = paneBrowser
	case key.Matches(msg, k.Up):
		m.gfield = max(m.gfield-1, 0)
	case key.Matches(msg, k.Down):
		m.gfield = min(m.gfield+1, len(groupFields(r.Kind))-1)
	case key.Matches(msg, k.Help):
		m.overlay = helpScreen{}
	case key.Matches(msg, k.Doctor):
		return m.openDoctor()
	case key.Matches(msg, k.Add):
		return m.openAdd()
	case key.Matches(msg, k.Sync):
		return m.openSync()
	case key.Matches(msg, k.Undo):
		return m.undo()
	case key.Matches(msg, k.Redo):
		return m.redo()
	}
	return m, nil
}
