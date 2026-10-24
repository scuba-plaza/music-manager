package tui

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/app"
	"music-manager/internal/clipboard"
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

// clipContents is what c copied.
type clipContents struct {
	from  string // the title of the track it came from
	attrs []clipAttr
	// seen is the fingerprint of the picture that the clipboard of the desktop
	// held when this was copied, if it held one.
	seen string
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
func (c clipContents) edit() app.Edit {
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
func (c clipContents) names() string {
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
	m, cmd := m.copied(r.Track, []clipAttr{attr})
	return m, cmd
}

// copied keeps the attributes of a track as what v pastes. The command finds out
// what the clipboard of the desktop holds just now.
func (m Model) copied(t manifest.Track, attrs []clipAttr) (Model, tea.Cmd) {
	_, title, _ := app.Display(t)
	m.clip = &clipContents{from: title, attrs: attrs}
	if len(attrs) == 1 {
		m.say("Copied the "+m.clip.names()+" of "+title, false)
	} else {
		m.say("Copied "+m.clip.names()+" from "+title, false)
	}
	return m, m.noteDesktop(m.clip)
}

// paste is v: put what was copied, or a picture copied outside, on the marked
// tracks or on what the cursor is on.
func (m Model) paste() (tea.Model, tea.Cmd) {
	if _, ok := m.pasteTargets(); !ok {
		m.say("Select a track, an album or an artist to paste on", true)
		return m, nil
	}
	if m.sys == nil {
		return m.pasteCopied()
	}
	sys := m.sys
	return m, func() tea.Msg {
		data, name, err := sys.Image()
		return desktopMsg{data, name, err}
	}
}

// pasteCopied puts what c copied on the marked tracks, or on what the cursor is on.
func (m Model) pasteCopied() (tea.Model, tea.Cmd) {
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
	if _, ok := m.pasteTargets(); !ok || m.clip == nil && m.sys == nil {
		return key.Binding{}, false
	}
	if m.clip == nil {
		return key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "paste")), true
	}
	return key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "paste "+m.clip.names())), true
}

// ---- a picture copied outside the interface

// SystemClipboard is the clipboard of the desktop, where a picture copied in a
// file manager or a browser can be found.
type SystemClipboard interface {
	// Image returns the picture on the clipboard, a JPEG or a PNG, and the
	// name of its file if it was copied as one. It fails with an error of
	// the clipboard package if there is no picture or it cannot be used.
	Image() (data []byte, name string, err error)
}

// WithClipboard lets v paste a picture from the clipboard of the desktop.
func WithClipboard(c SystemClipboard) Option { return func(m *Model) { m.sys = c } }

// desktopMsg is what the clipboard of the desktop held when v was pressed.
type desktopMsg struct {
	data []byte
	name string
	err  error
}

// seenMsg is what the clipboard of the desktop held when something was
// copied with c: seen is the fingerprint of its picture, or empty if it had none.
type seenMsg struct {
	clip *clipContents
	seen string
}

// fingerprint tells one picture from another: the SHA-256 that a cover with
// these bytes is kept under.
func fingerprint(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

// noteDesktop finds out what the clipboard of the desktop holds as c copies,
// so that v can tell a picture copied since from one that was there already.
func (m Model) noteDesktop(c *clipContents) tea.Cmd {
	sys := m.sys
	if sys == nil {
		return nil
	}
	return func() tea.Msg {
		data, _, err := sys.Image()
		seen := ""
		if err == nil {
			seen = fingerprint(data)
		}
		return seenMsg{c, seen}
	}
}

// pasteDesktop decides what v pastes once the clipboard of the desktop has been
// read: a picture on it that was not there when c was last used, else what c
// copied, else it says what there is to copy.
func (m Model) pasteDesktop(msg desktopMsg) (tea.Model, tea.Cmd) {
	switch newer := msg.err == nil && (m.clip == nil || fingerprint(msg.data) != m.clip.seen); {
	case newer:
		return m.pastePicture(msg.data, msg.name)
	case m.clip != nil:
		return m.pasteCopied()
	case errors.Is(msg.err, clipboard.ErrNoPicture):
		m.say("Nothing is copied yet: c copies from a track, or copy a picture file", true)
	case errors.Is(msg.err, clipboard.ErrNoTool):
		m.say("Nothing is copied yet: c copies from a track (a picture from the desktop needs wl-paste or xclip)", true)
	default:
		m.say(msg.err.Error(), true)
	}
	return m, nil
}

// pastePicture makes the picture the cover of the marked tracks, or of what the
// cursor is on.
func (m Model) pastePicture(data []byte, name string) (tea.Model, tea.Cmd) {
	targets, ok := m.pasteTargets()
	if !ok {
		m.say("Select a track, an album or an artist to paste on", true)
		return m, nil
	}
	sha := fingerprint(data)
	var ids []string
	for _, t := range targets {
		if t.CoverSHA256 != sha {
			ids = append(ids, t.ID)
		}
	}
	if len(ids) == 0 {
		m.say("Nothing to paste: "+plural(len(targets), "track")+" already "+haveOrHas(len(targets))+" this cover", false)
		return m, nil
	}
	from := "the clipboard"
	if name != "" {
		from = name
	}
	m.busy = true
	return m, m.saveMany("paste of a cover", "Pasted the cover from "+from+" on "+plural(len(ids), "track")+" · u undoes it", ids,
		func(l Library) (map[string]manifest.Track, error) { return l.SetCoverMany(ids, data) })
}
