package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

// c copies attributes of a track and v pastes them on another track, on the
// marked tracks, or on all the tracks of an album or an artist. What is copied
// is kept in the interface until it is quit or something else is copied.

// clipAttr is one attribute copied from a track: the artist, the album, how
// many tracks the album has, when it came out, the genre or the cover.
type clipAttr struct {
	field fieldKind
	// text is the value, and for the cover its SHA. A name takes the form in
	// Latin letters that was set by hand along with it, since that form belongs
	// to the name.
	text, latin string
}

// clipboard is what c copied.
type clipboard struct {
	from  string // the title of the track it came from
	attrs []clipAttr
}

// copyKind is the attribute that c copies when the cursor is on the field f of
// a track. A title, a track number, the cuts and the volume belong to the
// track and are not copied.
func copyKind(f fieldKind) (fieldKind, bool) {
	switch f {
	case fArtist, fAlbum, fReleased, fGenre, fCover:
		return f, true
	case fTrack:
		return fAlbumOf, true
	case fLatinArtist:
		return fArtist, true
	case fLatinAlbum:
		return fAlbum, true
	case fLatinGenre:
		return fGenre, true
	}
	return 0, false
}

// clipName names an attribute in what is said about a copy.
func clipName(f fieldKind) string {
	switch f {
	case fAlbumOf:
		return "tracks on album"
	case fCover:
		return "cover"
	}
	return strings.ToLower(f.label())
}

// attrOf is the attribute f of t, if t has one to copy.
func attrOf(t manifest.Track, f fieldKind) (clipAttr, bool) {
	a := clipAttr{field: f}
	switch f {
	case fArtist:
		a.text, a.latin = t.Artist, t.ArtistLatin
	case fAlbum:
		a.text, a.latin = t.Album, t.AlbumLatin
	case fGenre:
		a.text, a.latin = t.Genre, t.GenreLatin
	case fReleased:
		a.text = t.Released
	case fAlbumOf:
		if t.TrackTotal > 0 {
			a.text = strconv.Itoa(t.TrackTotal)
		}
	case fCover:
		a.text = t.CoverSHA256
	}
	return a, a.text != ""
}

// edit is the change that pasting the attributes makes to a track.
func (c clipboard) edit() app.Edit {
	var e app.Edit
	for _, a := range c.attrs {
		text, latin := a.text, a.latin
		switch a.field {
		case fArtist:
			e.Artist, e.ArtistLatin = &text, &latin
		case fAlbum:
			e.Album, e.AlbumLatin = &text, &latin
		case fGenre:
			e.Genre, e.GenreLatin = &text, &latin
		case fReleased:
			e.Released = &text
		case fAlbumOf:
			n, _ := strconv.Atoi(text)
			e.TrackTotal = &n
		case fCover:
			e.Cover = &text
		}
	}
	return e
}

// names lists the attributes in words: "cover", "artist and cover".
func (c clipboard) names() string {
	var names []string
	for _, a := range c.attrs {
		names = append(names, clipName(a.field))
	}
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// copyField is c in the fields of a track: copy the field the cursor is on.
func (m Model) copyField() (tea.Model, tea.Cmd) {
	r, ok := m.selectedRow()
	if !ok || r.Kind != kindTrack {
		return m, nil
	}
	f := m.curField()
	kind, ok := copyKind(f)
	if !ok {
		m.say("The "+strings.ToLower(f.label())+" belongs to its track and is not copied", true)
		return m, nil
	}
	attr, ok := attrOf(r.Track, kind)
	if !ok {
		m.say("This track has no "+clipName(kind)+" to copy", true)
		return m, nil
	}
	return m.copied(r.Track, []clipAttr{attr}), nil
}

// copied keeps the attributes of a track as what v pastes.
func (m Model) copied(t manifest.Track, attrs []clipAttr) Model {
	_, title, _ := app.Display(t)
	m.clip = &clipboard{from: title, attrs: attrs}
	if len(attrs) == 1 {
		m.say("Copied the "+m.clip.names()+" of "+title, false)
	} else {
		m.say("Copied "+m.clip.names()+" from "+title, false)
	}
	return m
}

// paste is v: put what was copied on the marked tracks, or on what the cursor is
// on.
func (m Model) paste() (tea.Model, tea.Cmd) {
	if m.clip == nil {
		m.say("Nothing is copied yet: c copies from a track", true)
		return m, nil
	}
	targets, ok := m.pasteTargets()
	if !ok {
		m.say("Select a track, an album or an artist to paste on", true)
		return m, nil
	}
	e := m.clip.edit()
	edits := map[string]app.Edit{}
	var ids []string
	for _, t := range targets {
		if changes(t, e) {
			edits[t.ID] = e
			ids = append(ids, t.ID)
		}
	}
	if len(edits) == 0 {
		m.say("Nothing to paste: "+plural(len(targets), "track")+" already "+haveOrHas(len(targets))+" the "+m.clip.names(), false)
		return m, nil
	}
	m.busy = true
	return m, m.saveMany("paste of the "+m.clip.names(), "Pasted the "+m.clip.names()+" on "+plural(len(ids), "track")+" · u undoes it", ids,
		func(l Library) (map[string]manifest.Track, error) { return l.EditMany(edits) })
}

func haveOrHas(n int) string {
	if n == 1 {
		return "has"
	}
	return "have"
}

// pasteTargets are the tracks that v pastes on: the marked ones, or the track
// under the cursor, or all the tracks of the album or artist it is on.
func (m Model) pasteTargets() ([]manifest.Track, bool) {
	if len(m.marks) > 0 {
		return m.markedTracks(), true
	}
	r, ok := m.selectedRow()
	switch {
	case !ok:
		return nil, false
	case r.Kind == kindTrack:
		return []manifest.Track{r.Track}, true
	case editableRow(r) && r.Kind != kindPlaylist:
		if g, ok := m.groupOf(r); ok {
			return g.tracks, true
		}
	}
	return nil, false
}

// pasteKey is the hint for v, if there is something to paste and something to
// paste it on.
func (m Model) pasteKey() (key.Binding, bool) {
	if _, ok := m.pasteTargets(); m.clip == nil || !ok {
		return key.Binding{}, false
	}
	return key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "paste "+m.clip.names())), true
}
