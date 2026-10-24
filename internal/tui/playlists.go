package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

// This file is everything the tree does with playlists: choosing one to add
// tracks to, and recording the changes made to them.

// playlistsMsg is the result of a change to the playlists.
type playlistsMsg struct {
	label, status string
	before, after []manifest.Playlist
	err           error
	done          func(m *Model) // runs once the change is in, to tidy up the screen
}

// editPlaylists runs a change to the playlists in the background and reports
// it as a playlistsMsg, with the playlists as they were and became.
func (m Model) editPlaylists(label string, call func(Library) (status string, err error), done ...func(m *Model)) tea.Cmd {
	lib, before := m.lib, m.playlists
	return func() tea.Msg {
		status, err := call(lib)
		if err != nil {
			return playlistsMsg{err: err}
		}
		after, err := lib.Playlists()
		msg := playlistsMsg{label: label, status: status, before: before, after: after, err: err}
		if len(done) > 0 {
			msg.done = done[0]
		}
		return msg
	}
}

func (m Model) playlistsEdited(msg playlistsMsg) (tea.Model, tea.Cmd) {
	m.busy = false
	if msg.err != nil {
		if o, ok := m.overlay.(interface{ setErr(string) }); ok {
			o.setErr(msg.err.Error())
		} else if m.editing {
			m.editErr = msg.err.Error()
		} else {
			m.say(msg.err.Error(), true)
		}
		return m, nil
	}
	m.overlay = nil
	m.playlists = msg.after
	m.rev++
	m.record(change{label: msg.label, playlists: true, playlistsBefore: msg.before, playlistsAfter: msg.after})
	m.say(msg.status+" · u undoes it", false)
	if msg.done != nil {
		msg.done(&m)
	}
	m.rebuild()
	return m, nil
}

// playlistTargets are the tracks that p adds to a playlist: the marked ones,
// or what the cursor is on.
func (m Model) playlistTargets() []string {
	if len(m.marks) > 0 {
		return idsOf(m.markedTracks())
	}
	r, ok := m.selectedRow()
	switch {
	case !ok, r.Key == playlistsKey:
		return nil
	case r.Kind == kindTrack:
		return []string{r.Track.ID}
	}
	return r.IDs
}

func (m Model) openPlaylistPicker() (tea.Model, tea.Cmd) {
	ids := m.playlistTargets()
	if len(ids) == 0 {
		m.say("Select tracks to add to a playlist: a track, an album, or mark several with space", true)
		return m, nil
	}
	p := &playlistPicker{ids: ids}
	for _, pl := range m.playlists {
		p.names = append(p.names, pl.Name)
		p.counts = append(p.counts, len(pl.Tracks))
	}
	p.name = textinput.New()
	p.name.Prompt = ""
	p.name.CharLimit = 80
	p.name.Placeholder = "name of the playlist"
	p.name.PlaceholderStyle = styles.dim
	if len(p.names) == 0 {
		p.naming = true
		p.name.Focus()
	}
	m.overlay = p
	return m, textinput.Blink
}

// playlistPicker is the dialog that chooses the playlist to add tracks to, or
// asks for the name of a new one.
type playlistPicker struct {
	ids    []string
	names  []string
	counts []int
	cursor int // len(names) is the entry that makes a new playlist
	naming bool
	name   textinput.Model
	err    string
}

func (p *playlistPicker) setErr(s string) { p.err = s }

func (p *playlistPicker) title() string { return "Add to a playlist" }
func (p *playlistPicker) width() int    { return 50 }

func (p *playlistPicker) lines(w int) []string {
	lines := []string{"", " " + styles.dim.Render("Adding "+plural(len(p.ids), "track")), ""}
	if p.naming {
		p.name.Width = max(w-8, 1)
		lines = append(lines, " "+styles.accent.Render("▌")+" "+p.name.View())
	} else {
		for i := 0; i <= len(p.names); i++ {
			label, right := "+ New playlist…", ""
			if i < len(p.names) {
				label, right = p.names[i], styles.dim.Render(itoa(p.counts[i])+" ")
			}
			line := "   " + label
			if i == p.cursor {
				line = " " + styles.accent.Render("▌ "+label)
			}
			lines = append(lines, spread(line, right, w))
		}
	}
	if p.err != "" {
		lines = append(lines, "")
		for _, l := range wrap(p.err, w-2) {
			lines = append(lines, " "+styles.bad.Render(l))
		}
	}
	return append(lines, "")
}

func (p *playlistPicker) hints(k keyMap) []key.Binding {
	enter := "add"
	if p.naming {
		enter = "create"
	}
	return []key.Binding{key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", enter)), k.Close2}
}

func (p *playlistPicker) key(m *Model, msg tea.KeyMsg) tea.Cmd {
	k := m.keys
	if p.naming {
		switch {
		case key.Matches(msg, k.Close2):
			if len(p.names) == 0 {
				m.overlay = nil
				return nil
			}
			p.naming, p.err = false, ""
			p.name.Blur()
			return nil
		case key.Matches(msg, k.Apply):
			name := strings.TrimSpace(p.name.Value())
			m.busy = true
			ids := p.ids
			return m.editPlaylists("new playlist "+name, func(l Library) (string, error) {
				return fmt.Sprintf("Made %s with %s", name, plural(len(ids), "track")), l.CreatePlaylist(name, ids...)
			})
		}
		var cmd tea.Cmd
		p.name, cmd = p.name.Update(msg)
		p.err = ""
		return cmd
	}

	switch {
	case key.Matches(msg, k.Close2), msg.String() == "q", msg.String() == "p":
		m.overlay = nil
	case key.Matches(msg, k.Up):
		p.cursor = max(p.cursor-1, 0)
	case key.Matches(msg, k.Down):
		p.cursor = min(p.cursor+1, len(p.names))
	case key.Matches(msg, k.Apply):
		if p.cursor == len(p.names) {
			p.naming = true
			return p.name.Focus()
		}
		name, ids := p.names[p.cursor], p.ids
		m.busy = true
		return m.editPlaylists("adding to "+name, func(l Library) (string, error) {
			added, err := l.AddToPlaylist(name, ids)
			status := "Added " + plural(added, "track") + " to " + name
			if skipped := len(ids) - added; skipped > 0 {
				status += fmt.Sprintf(" (%d already there)", skipped)
			}
			return status, err
		})
	}
	return nil
}

// playlistOf says which playlist a row belongs to: the playlist itself, or
// one of its tracks.
func playlistOf(r Row) (name string, ok bool) {
	switch {
	case r.Kind == kindPlaylist:
		return r.Label, true
	case r.Kind == kindTrack && strings.HasPrefix(r.Parent, playlistRowPrefix):
		return strings.TrimPrefix(r.Parent, playlistRowPrefix), true
	}
	return "", false
}

// playlistNamed finds a playlist by name.
func (m Model) playlistNamed(name string) (manifest.Playlist, bool) {
	for _, p := range m.playlists {
		if p.Name == name {
			return p, true
		}
	}
	return manifest.Playlist{}, false
}

// openRemoveInPlaylist is d where the cursor is in the playlists. It never
// touches the library: on a track it takes the track (or the marked tracks
// that are in the playlist) out of the playlist, and on a playlist it asks
// whether to delete it.
func (m Model) openRemoveInPlaylist(r Row, name string) (tea.Model, tea.Cmd) {
	if r.Kind == kindPlaylist {
		m.overlay = &confirm{
			head:    fmt.Sprintf("Delete the playlist “%s”?", name),
			body:    []string{"Its " + plural(r.Count, "track") + " stay in the library.", "", styles.dim.Render("u brings the playlist back.")},
			yesText: "delete",
			yes: func(m *Model) tea.Cmd {
				m.busy = true
				return m.editPlaylists("deletion of the playlist "+name, func(l Library) (string, error) {
					return "Deleted the playlist " + name, l.DeletePlaylist(name)
				})
			},
		}
		return m, nil
	}

	p, _ := m.playlistNamed(name)
	var ids []string
	for _, id := range p.Tracks {
		if m.marks[id] {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		ids = []string{r.Track.ID}
	}
	status := "Took " + plural(len(ids), "track") + " out of " + name
	if len(ids) == 1 {
		_, title, _ := app.Display(r.Track)
		status = fmt.Sprintf("Took “%s” out of %s", title, name)
	}
	m.busy = true
	return m, m.editPlaylists("removal from "+name, func(l Library) (string, error) {
		return status, l.RemoveFromPlaylist(name, ids)
	})
}

// movePlaylistTrack moves the selected track one place up or down its
// playlist, and the selection goes with it.
func (m Model) movePlaylistTrack(r Row, name string, dir int) (tea.Model, tea.Cmd) {
	if m.filter != "" {
		m.say("Clear the filter first, so the whole playlist is in view", true)
		return m, nil
	}
	p, _ := m.playlistNamed(name)
	to := slices.Index(p.Tracks, r.Track.ID) + dir
	if to < 0 || to >= len(p.Tracks) {
		if dir > 0 {
			m.say("It is the last track already", false)
		} else {
			m.say("It is the first track already", false)
		}
		return m, nil
	}
	id := r.Track.ID
	m.busy = true
	return m, m.editPlaylists("move of a track in "+name, func(l Library) (string, error) {
		return "Moved", l.MoveInPlaylist(name, id, to)
	})
}
