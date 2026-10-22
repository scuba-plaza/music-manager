package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/manifest"
)

// bulkForm is the dialog that edits several tracks at once.
type bulkForm struct {
	head   string
	tracks []manifest.Track
	in     [6]textinput.Model
	focus  int
	err    string
}

var bulkLabels = [6]string{"Artist", "Album", "Tracks on album", "Released", "Cut start", "Cut end"}

const (
	bulkArtist = iota
	bulkAlbum
	bulkOf
	bulkReleased
	bulkCutStart
	bulkCutEnd
)

// newBulkForm opens the form on the given tracks with the cursor in field
// focus, which may start out with prefill.
func newBulkForm(head string, tracks []manifest.Track, focus int, prefill string) *bulkForm {
	b := &bulkForm{head: head, tracks: tracks, focus: focus}
	for i := range b.in {
		in := textinput.New()
		in.Prompt = ""
		in.Placeholder = "(keep)"
		in.PlaceholderStyle = styles.dim
		in.Cursor.SetMode(cursor.CursorStatic)
		b.in[i] = in
	}
	b.in[focus].SetValue(prefill)
	b.in[focus].CursorEnd()
	b.in[focus].Focus()
	return b
}

func (b *bulkForm) values() bulkValues {
	v := func(i int) string { return b.in[i].Value() }
	return bulkValues{v(bulkArtist), v(bulkAlbum), v(bulkOf), v(bulkReleased), v(bulkCutStart), v(bulkCutEnd)}
}

func (b *bulkForm) title() string { return b.head }
func (b *bulkForm) width() int    { return 66 }

func (b *bulkForm) lines(w int) []string {
	lines := []string{
		"",
		" " + styles.dim.Render(fit(fmt.Sprintf("Applies to %s. Blank keeps what they have; - clears.", plural(len(b.tracks), "track")), w-2)),
		"",
	}
	for i, label := range bulkLabels {
		in := b.in[i]
		in.Width = max(w-22, 5)
		name := styles.dim.Render(fit(label, 17))
		if i == b.focus {
			name = styles.accent.Render(fit(label, 17))
		}
		lines = append(lines, " "+name+" "+in.View())
	}
	lines = append(lines, "")
	switch {
	case b.err != "":
		for _, l := range wrap(b.err, w-2) {
			lines = append(lines, " "+styles.bad.Render(l))
		}
	case bulkSummary(b.values()) != "":
		for _, l := range wrap(bulkSummary(b.values()), w-2) {
			lines = append(lines, " "+styles.accent.Render(l))
		}
	default:
		lines = append(lines, " "+styles.dim.Render("Type what the tracks should have."))
	}
	return append(lines, "")
}

func (b *bulkForm) hints(k keyMap) []key.Binding {
	return []key.Binding{k.Apply, k.Close2, k.NextField}
}

func (b *bulkForm) key(m *Model, msg tea.KeyMsg) tea.Cmd {
	k := m.keys
	switch {
	case key.Matches(msg, k.Close2):
		m.overlay = nil
		return nil
	case key.Matches(msg, k.Apply):
		return b.apply(m)
	case key.Matches(msg, k.NextField):
		return b.moveFocus(1)
	case key.Matches(msg, k.PrevField):
		return b.moveFocus(-1)
	}
	b.err = ""
	var cmd tea.Cmd
	b.in[b.focus], cmd = b.in[b.focus].Update(msg)
	return cmd
}

func (b *bulkForm) moveFocus(d int) tea.Cmd {
	b.in[b.focus].Blur()
	b.focus = (b.focus + d + len(b.in)) % len(b.in)
	return b.in[b.focus].Focus()
}

func (b *bulkForm) apply(m *Model) tea.Cmd {
	edits, changed, err := bulkPlan(b.tracks, b.values())
	switch {
	case err != nil:
		b.err = err.Error()
		return nil
	case changed == 0:
		b.err = "Nothing to change"
		return nil
	}
	ids := make([]string, 0, len(edits))
	for _, t := range b.tracks { // in the order they are listed
		if _, ok := edits[t.ID]; ok {
			ids = append(ids, t.ID)
		}
	}
	m.busy = true
	return m.saveMany("bulk edit of "+plural(len(ids), "track"), "Changed "+plural(len(ids), "track"), ids,
		func(l Library) (map[string]manifest.Track, error) { return l.EditMany(edits) })
}

// marked returns the tracks that are marked, in the order of the tree.
func (m Model) markedTracks() []manifest.Track {
	var out []manifest.Track
	for _, t := range m.tracks {
		if m.marks[t.ID] {
			out = append(out, t)
		}
	}
	return out
}

func (m Model) tracksFor(ids []string) []manifest.Track {
	byID := make(map[string]manifest.Track, len(m.tracks))
	for _, t := range m.tracks {
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

// openBulk is the e key: edit the marked tracks, or rename the artist or
// album the cursor is on, or just edit a track's fields.
func (m Model) openBulk() (tea.Model, tea.Cmd) {
	r, ok := m.selectedRow()
	if !ok {
		return m, nil
	}
	switch {
	case r.Kind == kindPlaylist:
		m.overlay = newPlaylistNamer(r.Label)
	case r.Key == playlistsKey:
		m.say("Select a playlist to rename it", true)
	case len(m.marks) > 0:
		ts := m.markedTracks()
		m.overlay = newBulkForm("Edit "+plural(len(ts), "marked track"), ts, bulkArtist, "")
	case r.Kind == kindTrack:
		m.focusInspector()
	case r.Kind == kindArtist:
		ts := m.tracksFor(r.IDs)
		m.overlay = newBulkForm("Rename the artist", ts, bulkArtist, firstOriginal(ts, fArtist))
	case r.Kind == kindAlbum && !strings.HasPrefix(r.Key, "singles"):
		ts := m.tracksFor(r.IDs)
		m.overlay = newBulkForm("Rename the album", ts, bulkAlbum, firstOriginal(ts, fAlbum))
	default:
		ts := m.tracksFor(r.IDs)
		m.overlay = newBulkForm("Edit "+plural(len(ts), "track"), ts, bulkArtist, "")
	}
	if m.overlay != nil {
		return m, textinput.Blink
	}
	return m, nil
}

func firstOriginal(ts []manifest.Track, f fieldKind) string {
	if len(ts) == 0 {
		return ""
	}
	original, _ := names(ts[0], f)
	return original
}
