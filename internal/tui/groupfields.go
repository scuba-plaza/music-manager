package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/app"
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
		return []fieldKind{fAlbum, fLatinAlbum, fReleased, fGenre, fLatinGenre, fAlbumOf, fCover}
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
	prefix, indent, vw := fieldPrefix(groupLabel(f), current, groupLabelWidth, w)
	if m.editing && current {
		return m.groupEditorLines(g, f, prefix, indent, vw)
	}
	if f == fCover {
		text := coverSummary(g.tracks)
		if text == "none" {
			return []string{prefix + styles.dim.Render(text)}
		}
		return []string{prefix + fit(text, vw)}
	}
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
	case key.Matches(msg, k.Paste):
		return m.paste()
	case key.Matches(msg, k.Edit):
		return m, m.startEdit()
	case key.Matches(msg, k.RemoveCover) && m.curField() == fCover:
		if g, ok := m.groupOf(r); ok {
			return m.setGroupCover(g, "")
		}
	case key.Matches(msg, k.Help):
		m.overlay = &helpScreen{}
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

// anyHas reports whether any of the tracks has the name that the Latin field f
// is the form of.
func anyHas(ts []manifest.Track, f fieldKind) bool {
	for _, t := range ts {
		if trueName(t, f) != "" {
			return true
		}
	}
	return false
}

// isNameField reports whether the field is the name of the group, which may not
// be empty.
func isNameField(f fieldKind) bool { return f == fArtist || f == fAlbum || f == fPlaylistName }

// newGroupDraft is the field of a group as it starts being edited: the text is
// what all the tracks have, or empty if they differ.
func newGroupDraft(g group, f fieldKind) draft {
	if f == fCover {
		return draft{field: f} // the text is the path of a new image
	}
	text, mixed, _ := groupValue(g, f)
	return draft{field: f, text: text, mixed: mixed}
}

// startGroupEdit opens the field the cursor is on, in the pane of a group.
func (m *Model) startGroupEdit(r Row) tea.Cmd {
	g, ok := m.groupOf(r)
	if !ok {
		return nil
	}
	f := m.curField()
	if f.isLatin() && !anyHas(g.tracks, f) {
		m.say("There is no "+strings.ToLower(f.label())+" to write in Latin letters", false)
		return nil
	}
	m.dr = newGroupDraft(g, f)
	m.prefilled = false
	m.in[0], m.in[1] = newInput(m.dr.text), newInput("")
	if m.dr.mixed {
		m.in[0].Placeholder, m.in[0].PlaceholderStyle = "(mixed)", styles.dim
	}
	m.inFocus, m.editing, m.editErr = 0, true, ""
	return m.in[0].Focus()
}

// groupNoun names the kind of group, for messages.
func groupNoun(kind rowKind) string {
	switch kind {
	case kindArtist:
		return "artist"
	case kindAlbum:
		return "album"
	}
	return "playlist"
}

// commitGroup saves the field being edited on all the tracks of the group.
// Text left as it started changes nothing; for fields whose tracks differ
// (they start empty) empty keeps them, and - clears them.
func (m Model) commitGroup(r Row) (tea.Model, tea.Cmd) {
	g, ok := m.groupOf(r)
	if !ok {
		return m, nil
	}
	if m.dr.field == fCover {
		return m.setGroupCover(g, m.dr.text)
	}
	if g.kind == kindPlaylist {
		return m.commitPlaylist(g)
	}
	f := m.dr.field
	text := strings.TrimSpace(m.dr.text)
	start := newGroupDraft(g, f)
	if text == start.text {
		m.editing = false // nothing changed, nothing to save
		return m, nil
	}
	if start.mixed && text == clearMark {
		text = ""
	}
	if text == "" && isNameField(f) {
		m.editErr = "The " + groupNoun(g.kind) + "'s name cannot be empty"
		return m, nil
	}
	d := m.dr
	d.text = text
	e, err := d.edit()
	if err != nil {
		m.editErr = err.Error()
		return m, nil
	}
	edits := map[string]app.Edit{}
	var ids []string
	for _, t := range g.tracks {
		if f.isLatin() && trueName(t, f) == "" {
			continue // a form of nothing
		}
		if changes(t, e) {
			edits[t.ID] = e
			ids = append(ids, t.ID)
		}
	}
	if len(edits) == 0 {
		m.editing = false
		return m, nil
	}
	moves := map[string]string{}
	switch f {
	case fArtist:
		moves = artistMoves(g.name, text, albumsOf(g.tracks))
	case fAlbum:
		moves = map[string]string{albumRowKey(g.artist, g.name): albumRowKey(g.artist, text)}
	}
	m.busy = true
	return m, m.saveMany("change of "+groupNoun(g.kind)+" "+whatOf(f), "Changed "+plural(len(ids), "track"), ids,
		func(l Library) (map[string]manifest.Track, error) { return l.EditMany(edits) },
		func(m *Model) { // the rows keep their place under the new name
			m.editing, m.editErr = false, ""
			moveKeys(m.expanded, moves)
			if to, ok := moves[m.selKey]; ok {
				m.selKey = to
			}
			m.rebuild()
		})
}

// commitPlaylist renames the playlist or sets its Latin name, whichever field
// is being edited. A refusal (a name that is another playlist's, or that would
// be the file of one) leaves everything as it was and is said under the input.
func (m Model) commitPlaylist(g group) (tea.Model, tea.Cmd) {
	f, old := m.dr.field, g.playlist.Name
	text := strings.TrimSpace(m.dr.text)
	shown, _, byHand := groupValue(g, f)
	switch {
	case text == shown, f == fPlaylistLatin && text == "" && !byHand:
		m.editing = false // nothing changed, nothing to save
		return m, nil
	case f == fPlaylistName && text == "":
		m.editErr = "The playlist's name cannot be empty"
		return m, nil
	}
	m.busy = true
	if f == fPlaylistLatin {
		return m, m.editPlaylists("change of the Latin name of "+old, func(l Library) (string, error) {
			return fmt.Sprintf("Latin name of %s is %s", old, latinNameText(text)), l.SetPlaylistLatin(old, text)
		}, func(m *Model) { m.editing, m.editErr = false, "" })
	}
	return m, m.editPlaylists("renaming "+old, func(l Library) (string, error) {
		return fmt.Sprintf("Renamed %s to %s", old, text), l.RenamePlaylist(old, text)
	}, func(m *Model) { // the renamed playlist stays open, and selected
		m.editing, m.editErr = false, ""
		moves := map[string]string{playlistRowKey(old): playlistRowKey(text)}
		moveKeys(m.expanded, moves)
		if to, ok := moves[m.selKey]; ok {
			m.selKey = to
		}
	})
}

// latinNameText is a Latin name as the status line says it.
func latinNameText(latin string) string {
	if latin == "" {
		return "worked out"
	}
	return latin
}

// whatOf names a field of a group in a change: "name", "Latin name", "genre".
func whatOf(f fieldKind) string {
	label := groupLabel(f)
	if strings.HasPrefix(label, "Latin") {
		return label
	}
	return strings.ToLower(label)
}

// albumsOf lists the albums of the tracks, each once, in the order they appear.
func albumsOf(ts []manifest.Track) []string {
	var albums []string
	seen := map[string]bool{}
	for _, t := range ts {
		if t.Album != "" && !seen[t.Album] {
			seen[t.Album] = true
			albums = append(albums, t.Album)
		}
	}
	return albums
}

// artistMoves are the keys of the rows that change with the name of an artist:
// the artist, its singles and its albums.
func artistMoves(from, to string, albums []string) map[string]string {
	moves := map[string]string{artistKey(from): artistKey(to), singlesKey(from): singlesKey(to)}
	for _, a := range albums {
		moves[albumRowKey(from, a)] = albumRowKey(to, a)
	}
	return moves
}

// moveKeys carries the open rows over to the keys they have under their new
// names.
func moveKeys(open map[string]bool, moves map[string]string) {
	var carried []string
	for from, to := range moves {
		if open[from] {
			delete(open, from)
			carried = append(carried, to)
		}
	}
	for _, to := range carried {
		open[to] = true
	}
}

// groupEditorLines draws the field being edited in the pane of a group: the
// input, and under it what the text does.
func (m Model) groupEditorLines(g group, f fieldKind, prefix, indent string, vw int) []string {
	first := m.in[0]
	first.Width = max(vw-1, 1)
	lines := []string{prefix + first.View()}
	hint := ""
	switch {
	case f == fArtist || f == fAlbum:
		hint = m.joinHint(g, f)
	case f.isLatin():
		hint = "for " + g.name + " · empty takes the form worked out back"
	case f == fReleased:
		hint = "e.g. 1999, 1999-05 or 1999-05-17 · empty if unknown"
	case f == fGenre:
		hint = "e.g. Folk or Neoclassical · empty for none"
	case f == fAlbumOf:
		hint = "how many tracks the album has · empty if unknown"
	case f == fCover:
		hint = "a JPEG or PNG file · empty removes the cover"
	}
	if hint != "" {
		lines = append(lines, hintLines(indent, hint, vw)...)
	}
	if m.dr.mixed && f != fCover {
		lines = append(lines, hintLines(indent, "the tracks differ · empty keeps them · - clears", vw)...)
	}
	if m.editErr != "" {
		for _, l := range wrap(m.editErr, vw) {
			lines = append(lines, indent+styles.bad.Render(l))
		}
	}
	return lines
}

// joinHint says that the name being typed is one that exists already, so that
// saving it joins the group with it.
func (m Model) joinHint(g group, f fieldKind) string {
	typed := strings.TrimSpace(m.dr.text)
	if typed == "" || typed == g.name {
		return ""
	}
	n := 0
	for _, t := range m.tracks {
		switch {
		case t.Status != manifest.Identified:
		case f == fArtist && t.Artist == typed:
			n++
		case f == fAlbum && t.Artist == g.artist && t.Album == typed:
			n++
		}
	}
	switch {
	case n == 0:
		return ""
	case f == fArtist:
		return "joins the artist " + typed + " (" + plural(n, "track") + ")"
	}
	return "joins the album " + typed + " of " + g.artist + " (" + plural(n, "track") + ")"
}

// coverSummary says what the covers of the tracks are: none, the same on all of
// them, or how they differ.
func coverSummary(ts []manifest.Track) string {
	distinct := map[string]bool{}
	without := 0
	for _, t := range ts {
		if t.CoverSHA256 == "" {
			without++
		} else {
			distinct[t.CoverSHA256] = true
		}
	}
	switch {
	case len(distinct) == 0:
		return "none"
	case len(distinct) == 1 && without == 0:
		return "the same on all " + plural(len(ts), "track")
	case without == 0:
		return plural(len(distinct), "cover") + " among the tracks"
	}
	return plural(len(distinct), "cover") + ", " + plural(without, "track") + " without one"
}

// setGroupCover gives all the tracks of the group the image in the file at path,
// or takes their covers away if path is empty.
func (m Model) setGroupCover(g group, path string) (tea.Model, tea.Cmd) {
	path = cleanPath(path)
	var ids []string
	for _, t := range g.tracks {
		if path != "" || t.CoverSHA256 != "" {
			ids = append(ids, t.ID)
		}
	}
	if len(ids) == 0 {
		m.editing = false // no cover to remove
		return m, nil
	}
	label := "change of " + groupNoun(g.kind) + " cover"
	if path == "" {
		label = "removal of the " + groupNoun(g.kind) + "'s cover"
	}
	m.busy = true
	return m, m.saveMany(label, "Changed "+plural(len(ids), "track"), ids,
		func(l Library) (map[string]manifest.Track, error) {
			var data []byte
			if path != "" {
				var err error
				if data, err = os.ReadFile(path); err != nil {
					return nil, friendlyFileError(path, err)
				}
			}
			return l.SetCoverMany(ids, data)
		},
		func(m *Model) { m.editing, m.editErr = false, "" })
}
