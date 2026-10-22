package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

// This file is everything the tree does with several tracks: marking them,
// moving one within its album, and numbering an album.

// markState is how much of what a row holds is marked: none, some or all.
func (m Model) markState(r Row) (marked, of int) {
	if r.Kind == kindTrack {
		if m.marks[r.Track.ID] {
			return 1, 1
		}
		return 0, 1
	}
	for _, id := range r.IDs {
		if m.marks[id] {
			marked++
		}
	}
	return marked, len(r.IDs)
}

// toggleMark marks the selected row's tracks, or unmarks them if all are
// marked already, and moves on to the next row.
func (m *Model) toggleMark() {
	r, ok := m.selectedRow()
	if !ok {
		return
	}
	ids := r.IDs
	if r.Kind == kindTrack {
		ids = []string{r.Track.ID}
	}
	if marked, of := m.markState(r); of > 0 && marked == of {
		for _, id := range ids {
			delete(m.marks, id)
		}
	} else {
		for _, id := range ids {
			m.marks[id] = true
		}
	}
	m.moveBy(1)
}

// markAll marks every track matching the filter, or unmarks everything if
// they all are marked already.
func (m *Model) markAll() {
	shown := m.tracks
	if strings.TrimSpace(m.filter) != "" {
		shown = (&manifest.Manifest{Tracks: m.tracks}).Search(m.filter)
	}
	all := len(shown) > 0
	for _, t := range shown {
		all = all && m.marks[t.ID]
	}
	for _, t := range shown {
		if all {
			delete(m.marks, t.ID)
		} else {
			m.marks[t.ID] = true
		}
	}
}

// changedMsg is the result of a change to several tracks at once.
type changedMsg struct {
	label, status string
	before, after []manifest.Track // aligned: the same tracks, before and after
	err           error
}

// saveMany runs a change to the tracks with the given ids in the background
// and reports it as a changedMsg.
func (m Model) saveMany(label, status string, ids []string, call func(Library) (map[string]manifest.Track, error)) tea.Cmd {
	byID := map[string]manifest.Track{}
	for _, t := range m.tracks {
		byID[t.ID] = t
	}
	lib := m.lib
	return func() tea.Msg {
		got, err := call(lib)
		if err != nil {
			return changedMsg{err: err}
		}
		var before, after []manifest.Track
		for _, id := range ids {
			if t, ok := got[id]; ok {
				before, after = append(before, byID[id]), append(after, t)
			}
		}
		return changedMsg{label: label, status: status, before: before, after: after}
	}
}

func (m Model) changed(msg changedMsg) (tea.Model, tea.Cmd) {
	m.busy = false
	if msg.err != nil {
		if form, ok := m.overlay.(*bulkForm); ok {
			form.err = msg.err.Error()
		} else {
			m.say(msg.err.Error(), true)
		}
		return m, nil
	}
	m.overlay = nil
	for _, t := range msg.after {
		m.upsert(t)
	}
	m.record(change{label: msg.label, before: msg.before, after: msg.after})
	m.say(msg.status, false)
	if len(msg.after) > 0 {
		m.revealTrack(msg.after[0])
	}
	return m, nil
}

// moveTrack swaps the selected track's number with its neighbour's, which
// moves it up or down its album.
func (m Model) moveTrack(dir int) (tea.Model, tea.Cmd) {
	if m.sortKey != "" {
		m.say("Moving a track needs the tree view · S, then Tree", true)
		return m, nil
	}
	r, ok := m.selectedRow()
	if !ok || r.Kind != kindTrack || !strings.HasPrefix(r.Parent, "album") {
		m.say("Only tracks of an album can be moved", true)
		return m, nil
	}
	if m.filter != "" {
		m.say("Clear the filter first, so the whole album is in view", true)
		return m, nil
	}
	i := m.cursor + dir
	if i < 0 || i >= len(m.rows) || m.rows[i].Kind != kindTrack || m.rows[i].Parent != r.Parent {
		if dir > 0 {
			m.say("It is the last track already", false)
		} else {
			m.say("It is the first track already", false)
		}
		return m, nil
	}
	a, b := r.Track, m.rows[i].Track
	if a.TrackNumber == 0 || b.TrackNumber == 0 {
		m.say("Number the album's tracks first: select the album and press n", true)
		return m, nil
	}
	edits := map[string]app.Edit{a.ID: {TrackNumber: &b.TrackNumber}, b.ID: {TrackNumber: &a.TrackNumber}}
	m.busy = true
	return m, m.saveMany("move of a track", "Moved", []string{a.ID, b.ID},
		func(l Library) (map[string]manifest.Track, error) { return l.EditMany(edits) })
}

// renumber numbers the album of the selected row 1..N in the order shown.
func (m Model) renumber() (tea.Model, tea.Cmd) {
	if m.sortKey != "" {
		m.say("Numbering an album needs the tree view · S, then Tree", true)
		return m, nil
	}
	r, ok := m.selectedRow()
	if !ok {
		return m, nil
	}
	if r.Kind == kindTrack {
		for _, p := range m.rows {
			if p.Key == r.Parent {
				r = p
			}
		}
	}
	if r.Kind != kindAlbum || strings.HasPrefix(r.Key, "singles") {
		m.say("Select an album, or one of its tracks, to number it", true)
		return m, nil
	}
	if m.filter != "" {
		m.say("Clear the filter first, so the whole album is in view", true)
		return m, nil
	}
	ids := r.IDs
	m.busy = true
	return m, m.saveMany("numbering of an album", fmt.Sprintf("Numbered %s 1–%d", plural(len(ids), "track"), len(ids)), ids,
		func(l Library) (map[string]manifest.Track, error) { return l.Renumber(ids) })
}
