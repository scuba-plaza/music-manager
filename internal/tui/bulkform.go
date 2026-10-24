package tui

import (
	"fmt"

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
	in     [10]textinput.Model
	focus  int
	err    string
}

var bulkLabels = [10]string{"Artist", "Artist (Latin)", "Album", "Album (Latin)", "Tracks on album", "Released", "Genre", "Genre (Latin)", "Cut start", "Cut end"}

const (
	bulkArtist = iota
	bulkArtistLatin
	bulkAlbum
	bulkAlbumLatin
	bulkOf
	bulkReleased
	bulkGenre
	bulkGenreLatin
	bulkCutStart
	bulkCutEnd
)

// newBulkForm opens the form on the given tracks with the cursor in the first
// field.
func newBulkForm(head string, tracks []manifest.Track) *bulkForm {
	b := &bulkForm{head: head, tracks: tracks}
	for i := range b.in {
		in := textinput.New()
		in.Prompt = ""
		in.Placeholder = "(keep)"
		in.PlaceholderStyle = styles.dim
		in.Cursor.SetMode(cursor.CursorStatic)
		b.in[i] = in
	}
	b.in[b.focus].Focus()
	return b
}

func (b *bulkForm) values() bulkValues {
	v := func(i int) string { return b.in[i].Value() }
	return bulkValues{
		artist: v(bulkArtist), album: v(bulkAlbum), of: v(bulkOf), released: v(bulkReleased), genre: v(bulkGenre), cutStart: v(bulkCutStart), cutEnd: v(bulkCutEnd),
		artistLatin: v(bulkArtistLatin), albumLatin: v(bulkAlbumLatin), genreLatin: v(bulkGenreLatin),
	}
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

// openBulk is the e key: edit the marked tracks, or go into the fields of what
// the cursor is on (a track, an artist, an album, a playlist), as → does.
func (m Model) openBulk() (tea.Model, tea.Cmd) {
	r, ok := m.selectedRow()
	if !ok {
		return m, nil
	}
	switch {
	case r.Kind == kindPlaylist:
		m.focusInspector()
	case r.Key == playlistsKey:
		m.say("Select a playlist to edit it", true)
	case len(m.marks) > 0:
		ts := m.markedTracks()
		m.overlay = newBulkForm("Edit "+plural(len(ts), "marked track"), ts)
	case editableRow(r):
		m.focusInspector()
	default:
		ts := m.tracksFor(r.IDs)
		m.overlay = newBulkForm("Edit "+plural(len(ts), "track"), ts)
	}
	if m.overlay != nil {
		return m, textinput.Blink
	}
	return m, nil
}
