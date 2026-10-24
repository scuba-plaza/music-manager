package browse

import (
	"errors"
	"fmt"
	"strings"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

// An artist, an album and a playlist have fields, as a track has. What is
// edited there is edited on the whole of them: all the tracks of the artist or
// the album, whatever the filter shows.

// group is what an artist, album or playlist row stands for.
type group struct {
	kind     rowKind
	name     string            // the artist, album or playlist
	artist   string            // the artist of an album
	tracks   []manifest.Track  // the tracks of an artist or album
	playlist manifest.Playlist // the playlist, for a playlist
}

// groupOf finds the group that the row with the given key stands for, among
// all the tracks, not only those a filter keeps.
func groupOf(s Snapshot, key string) (group, bool) {
	var r Row
	found := false
	for _, row := range buildRows(s.Tracks, s.Playlists, "", func(string) bool { return true }) {
		if row.Key == key {
			r, found = row, true
			break
		}
	}
	if !found {
		return group{}, false
	}
	g := group{kind: r.Kind, name: r.Label, artist: r.Artist}
	switch r.Kind {
	case kindArtist:
		for _, t := range s.Tracks {
			if t.Status == manifest.Identified && t.Artist == r.Artist {
				g.tracks = append(g.tracks, t)
			}
		}
	case kindAlbum:
		if r.Album == "" {
			return g, false
		}
		for _, t := range s.Tracks {
			if t.Status == manifest.Identified && t.Artist == r.Artist && t.Album == r.Album {
				g.tracks = append(g.tracks, t)
			}
		}
	case kindPlaylist:
		if i, ok := manifest.PlaylistNamed(s.Playlists, r.Label); ok {
			g.playlist = s.Playlists[i]
			return g, true
		}
		return g, false
	default:
		return g, false
	}
	return g, len(g.tracks) > 0
}

// GroupIDs lists the tracks of the artist or album with the given row key,
// whatever a filter shows; it is how an edit or a paste on a group finds them.
func GroupIDs(s Snapshot, key string) ([]string, bool) {
	g, ok := groupOf(s, key)
	if !ok || g.kind == kindPlaylist {
		return nil, false
	}
	return idsOf(g.tracks), true
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

// groupFieldViews describes the fields of a group for the pane.
func groupFieldViews(g group) []FieldView {
	var out []FieldView
	for _, f := range groupFields(g.kind) {
		fv := FieldView{Key: f.key(), Label: groupLabel(f), Editable: true, Empty: "—"}
		switch {
		case f == fCover:
			fv.Text, fv.Empty = coverSummary(g.tracks), "none"
			if fv.Text == "none" {
				fv.Text = ""
			}
			fv.Hint = "a JPEG or PNG image · choose or drop one, or remove the cover"
		default:
			text, mixed, byHand := groupValue(g, f)
			fv.Text, fv.Mixed, fv.Edit = text, mixed, text
			if mixed {
				fv.Empty = "(mixed)"
			}
			if f.isLatin() && !byHand || f == fPlaylistLatin && !byHand {
				fv.Dim = true
			}
			fv.Hint = groupHint(g, f, mixed)
			if f.isLatin() && !anyHas(g.tracks, f) {
				fv.Editable = false
			}
		}
		out = append(out, fv)
	}
	return out
}

// groupHint explains what may be typed in a field of a group.
func groupHint(g group, f fieldKind, mixed bool) string {
	var hint string
	switch {
	case f.isLatin():
		hint = "for " + g.name + " · empty takes the form worked out back"
	case f == fPlaylistLatin:
		hint = "empty takes the form worked out back"
	case f == fReleased:
		hint = "e.g. 1999, 1999-05 or 1999-05-17 · empty if unknown"
	case f == fGenre:
		hint = "e.g. Folk or Neoclassical · empty for none"
	case f == fAlbumOf:
		hint = "how many tracks the album has · empty if unknown"
	}
	if mixed {
		if hint != "" {
			hint += " · "
		}
		hint += "the tracks differ · empty keeps them · - clears"
	}
	return hint
}

// GroupHint says that the name typed for an artist or an album is one that
// exists already, so that saving it joins the group with it.
func GroupHint(s Snapshot, key, field, text string) string {
	g, ok := groupOf(s, key)
	f, fok := fieldByKey(field)
	if !ok || !fok {
		return ""
	}
	typed := strings.TrimSpace(text)
	if typed == "" || typed == g.name || (f != fArtist && f != fAlbum) {
		return ""
	}
	n := 0
	for _, t := range s.Tracks {
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

// SetGroupField saves a field of an artist, an album or a playlist on all it
// holds. Text left as it started changes nothing; for a field whose tracks
// differ (they start empty) empty keeps them, and - clears them.
func (e *Editor) SetGroupField(key, field, text string) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	f, ok := fieldByKey(field)
	if !ok {
		return Result{}, fmt.Errorf("there is no field %q", field)
	}
	s, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	g, ok := groupOf(s, key)
	if !ok {
		return Result{}, errors.New("that is not there any more")
	}
	if !contains(groupFields(g.kind), f) {
		return Result{}, fmt.Errorf("a %s has no field %q", groupNoun(g.kind), field)
	}
	if g.kind == kindPlaylist {
		return e.setPlaylistField(g, f, text)
	}
	text = strings.TrimSpace(text)
	shown, mixed, _ := groupValue(g, f)
	if !mixed && text == shown {
		return Result{Same: true}, nil
	}
	if mixed && text == "" {
		return Result{Same: true}, nil
	}
	if mixed && text == clearMark {
		text = ""
	}
	if f.isLatin() && !anyHas(g.tracks, f) {
		return Result{}, errors.New("There is no " + strings.ToLower(f.label()) + " to write in Latin letters")
	}
	if text == "" && isNameField(f) {
		return Result{}, errors.New("The " + groupNoun(g.kind) + "'s name cannot be empty")
	}
	d := draft{field: f, text: text}
	edit, err := d.edit()
	if err != nil {
		return Result{}, err
	}
	edits := map[string]app.Edit{}
	var ids []string
	for _, t := range g.tracks {
		if f.isLatin() && trueName(t, f) == "" {
			continue // a form of nothing
		}
		if changes(t, edit) {
			edits[t.ID] = edit
			ids = append(ids, t.ID)
		}
	}
	if len(edits) == 0 {
		return Result{Same: true}, nil
	}
	moves := map[string]string{}
	switch f {
	case fArtist:
		moves = artistMoves(g.name, text, albumsOf(g.tracks))
	case fAlbum:
		moves = map[string]string{albumRowKey(g.artist, g.name): albumRowKey(g.artist, text)}
	}
	r, err := e.saveMany(s, "change of "+groupNoun(g.kind)+" "+whatOf(f), "Changed "+plural(len(ids), "track"), ids,
		func() (map[string]manifest.Track, error) { return e.lib.EditMany(edits) })
	if err != nil {
		return r, err
	}
	r.Moves = moves
	e.noteMoves(moves)
	r.Reveal = "" // the cursor stays on the group, whatever it is called now
	return r, nil
}

func contains(fs []fieldKind, f fieldKind) bool {
	for _, x := range fs {
		if x == f {
			return true
		}
	}
	return false
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

// setPlaylistField renames the playlist or sets its Latin name. A refusal (a
// name that is another playlist's, or that would be the file of one) leaves
// everything as it was.
func (e *Editor) setPlaylistField(g group, f fieldKind, text string) (Result, error) {
	old := g.playlist.Name
	text = strings.TrimSpace(text)
	shown, _, byHand := groupValue(g, f)
	switch {
	case text == shown, f == fPlaylistLatin && text == "" && !byHand:
		return Result{Same: true}, nil
	case f == fPlaylistName && text == "":
		return Result{}, errors.New("The playlist's name cannot be empty")
	}
	if f == fPlaylistLatin {
		return e.editPlaylists("change of the Latin name of "+old, func() (string, error) {
			return fmt.Sprintf("Latin name of %s is %s", old, latinNameText(text)), e.lib.SetPlaylistLatin(old, text)
		})
	}
	r, err := e.editPlaylists("renaming "+old, func() (string, error) {
		return fmt.Sprintf("Renamed %s to %s", old, text), e.lib.RenamePlaylist(old, text)
	})
	if err == nil && !r.Same {
		r.Moves = map[string]string{playlistRowKey(old): playlistRowKey(text)}
		e.noteMoves(r.Moves)
	}
	return r, err
}

// latinNameText is a Latin name as the status line says it.
func latinNameText(latin string) string {
	if latin == "" {
		return "worked out"
	}
	return latin
}

// SetGroupCover gives all the tracks of the artist or album the image, or
// takes their covers away if there is none.
func (e *Editor) SetGroupCover(key string, image []byte) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	g, ok := groupOf(s, key)
	if !ok || g.kind == kindPlaylist {
		return Result{}, errors.New("that is not there any more")
	}
	r, err := e.setCover(s, idsOf(g.tracks), image, groupNoun(g.kind)+" cover")
	r.Reveal = "" // the cursor stays on the group
	return r, err
}
