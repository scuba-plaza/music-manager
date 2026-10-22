package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

// confirm is a dialog that asks before something that cannot be taken back
// with a single keypress. Only y and enter go ahead.
type confirm struct {
	head    string
	body    []string
	yesText string // what y does, for the footer
	yes     func(m *Model) tea.Cmd
}

func (c *confirm) title() string { return "Are you sure?" }
func (c *confirm) width() int    { return 64 }

func (c *confirm) lines(w int) []string {
	lines := []string{"", " " + styles.bold.Render(fit(c.head, w-2)), ""}
	for _, b := range c.body {
		for _, l := range wrap(b, w-2) {
			lines = append(lines, " "+l)
		}
	}
	return append(lines, "")
}

func (c *confirm) hints(k keyMap) []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("y", "enter"), key.WithHelp("y/enter", c.yesText)),
		k.Close2,
	}
}

func (c *confirm) key(m *Model, msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "y", "enter":
		m.overlay = nil
		return c.yes(m)
	case "esc", "n", "q":
		m.overlay = nil
	}
	return nil
}

// openRemove asks whether to remove the marked tracks, or what the cursor is on.
func (m Model) openRemove() (tea.Model, tea.Cmd) {
	r, ok := m.selectedRow()
	if !ok {
		return m, nil
	}
	var ts []manifest.Track
	var head string
	switch {
	case len(m.marks) > 0:
		ts = m.markedTracks()
		head = fmt.Sprintf("Remove %s?", plural(len(ts), "marked track"))
	case r.Kind == kindTrack:
		ts = []manifest.Track{r.Track}
		_, title, _ := app.Display(r.Track)
		head = fmt.Sprintf("Remove %q?", title)
	case r.Kind == kindArtist:
		ts = m.tracksFor(r.IDs)
		head = fmt.Sprintf("Remove %q and its %s?", r.Label, plural(len(ts), "track"))
	case r.Kind == kindAlbum:
		ts = m.tracksFor(r.IDs)
		head = fmt.Sprintf("Remove the %s of %q?", plural(len(ts), "track"), r.Label)
	default:
		ts = m.tracksFor(r.IDs)
		head = fmt.Sprintf("Remove everything in %q (%s)?", r.Label, plural(len(ts), "track"))
	}
	if len(ts) == 0 {
		return m, nil
	}

	var body []string
	if len(ts) > 1 {
		for i, t := range ts {
			if i == 5 {
				body = append(body, styles.dim.Render(fmt.Sprintf("…and %d more", len(ts)-5)))
				break
			}
			_, title, _ := app.Display(t)
			body = append(body, "• "+title)
		}
		body = append(body, "")
	}
	pronoun := "it"
	if len(ts) > 1 {
		pronoun = "them"
	}
	body = append(body, styles.dim.Render("Nothing is deleted from disk until you sync with prune, and u brings "+pronoun+" back until then."))

	ids := make([]string, len(ts))
	for i, t := range ts {
		ids[i] = t.ID
	}
	m.overlay = &confirm{head: head, body: body, yesText: "remove", yes: func(m *Model) tea.Cmd {
		m.busy = true
		return m.removing(ts)
	}}
	return m, nil
}

// removedMsg is the result of removing tracks.
type removedMsg struct {
	tracks []manifest.Track
	err    error
}

func (m Model) removing(ts []manifest.Track) tea.Cmd {
	lib := m.lib
	ids := make([]string, len(ts))
	for i, t := range ts {
		ids[i] = t.ID
	}
	return func() tea.Msg {
		got, err := lib.RemoveMany(ids)
		return removedMsg{got, err}
	}
}

func (m Model) removed(msg removedMsg) (tea.Model, tea.Cmd) {
	m.busy = false
	if msg.err != nil {
		m.say(msg.err.Error(), true)
		return m, nil
	}
	m.dropTracks(msg.tracks)
	m.record(change{label: "removal of " + plural(len(msg.tracks), "track"), before: msg.tracks})
	m.say("Removed "+plural(len(msg.tracks), "track")+" · u undoes it", false)
	m.rebuild()
	return m, nil
}

// dropTracks forgets the given tracks (and their marks).
func (m *Model) dropTracks(ts []manifest.Track) {
	gone := make(map[string]bool, len(ts))
	for _, t := range ts {
		gone[t.ID] = true
		delete(m.marks, t.ID)
	}
	m.rev++
	kept := m.tracks[:0:0]
	for _, t := range m.tracks {
		if !gone[t.ID] {
			kept = append(kept, t)
		}
	}
	m.tracks = kept
}
