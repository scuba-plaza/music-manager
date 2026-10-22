package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"music-manager/internal/app"

	"music-manager/internal/manifest"
	"strings"
)

// This file is the right pane's behaviour: choosing a field, editing it,
// saving at once, and undoing.

// focusInspector gives the keyboard to the inspector if a track is selected.
func (m *Model) focusInspector() bool {
	r, ok := m.selectedRow()
	if !ok || r.Kind != kindTrack {
		return false
	}
	m.focus = paneInspector
	return true
}

func (m Model) updateInspector(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	switch {
	case key.Matches(msg, k.Back), key.Matches(msg, k.Switch):
		m.focus = paneBrowser
	case key.Matches(msg, k.Up):
		m.field = max(m.field-1, 0)
	case key.Matches(msg, k.Down):
		m.field = min(m.field+1, len(trackFields)-1)
	case key.Matches(msg, k.Edit):
		return m, m.startEdit()
	case key.Matches(msg, k.Play):
		return m.togglePreview()
	case key.Matches(msg, k.Less):
		return m.nudge(-100 * time.Millisecond)
	case key.Matches(msg, k.More):
		return m.nudge(100 * time.Millisecond)
	case key.Matches(msg, k.LessBig):
		return m.nudge(-time.Second)
	case key.Matches(msg, k.MoreBig):
		return m.nudge(time.Second)
	case key.Matches(msg, k.ClearCut):
		return m.setCut(0, "")
	case key.Matches(msg, k.Undo):
		return m.undo()
	case key.Matches(msg, k.Redo):
		return m.redo()
	}
	return m, nil
}

func newInput(value string) textinput.Model {
	in := textinput.New()
	in.Prompt = ""
	in.Cursor.SetMode(cursor.CursorStatic)
	in.SetValue(value)
	in.CursorEnd()
	return in
}

// hasSecond reports whether the field being edited has a second input.
func (m Model) hasSecond() bool { return m.dr.field.isName() || m.dr.field == fTrack }

// prefilledFrom reports whether the draft of f starts from the file's old tags
// because the record has no value of its own yet.
func prefilledFrom(t manifest.Track, f fieldKind) bool {
	if t.Status == manifest.Identified {
		return false
	}
	switch f {
	case fTitle:
		return t.OriginalTitle == ""
	case fArtist:
		return t.OriginalArtist == ""
	case fAlbum:
		return t.OriginalAlbum == "" && t.Hint.Album != ""
	case fTrack:
		return t.TrackNumber == 0 && t.Hint.Track > 0
	}
	return false
}

func (m *Model) startEdit() tea.Cmd {
	r, ok := m.selectedRow()
	if !ok || r.Kind != kindTrack {
		return nil
	}
	f := trackFields[m.field]
	m.dr = newDraft(r.Track, f)
	m.prefilled = prefilledFrom(r.Track, f)
	m.in[0], m.in[1] = newInput(m.dr.text), newInput(m.dr.second)
	m.inFocus, m.editing, m.editErr = 0, true, ""
	return m.in[0].Focus()
}

func (m Model) updateEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.editing, m.editErr = false, ""
		return m, nil
	case key.Matches(msg, m.keys.Save):
		return m.commit()
	case key.Matches(msg, m.keys.NextInput) && m.hasSecond():
		m.in[m.inFocus].Blur()
		m.inFocus = 1 - m.inFocus
		return m, m.in[m.inFocus].Focus()
	}

	m.editErr = ""
	var cmd tea.Cmd
	m.in[m.inFocus], cmd = m.in[m.inFocus].Update(msg)
	if m.inFocus == 0 {
		m.dr.setText(m.in[0].Value())
		if m.dr.field.isName() && m.dr.follow && !m.dr.touched {
			m.in[1].SetValue(m.dr.second)
		}
	} else {
		m.dr.setSecond(m.in[1].Value())
	}
	return m, cmd
}

// editedMsg is the result of saving a change. A change with a nudge key is
// one of a run of nudges of the same cut.
type editedMsg struct {
	label         string
	nudge         string
	before, after manifest.Track
	err           error
}

// save sends a change to the library, and says how it turned out in an
// editedMsg.
func (m Model) save(t manifest.Track, e app.Edit, label, nudge string) tea.Cmd {
	lib := m.lib
	return func() tea.Msg {
		after, err := lib.Edit(t.ID, e)
		return editedMsg{label: label, nudge: nudge, before: t, after: after, err: err}
	}
}

// minKept is the least audio a cut may leave of a track.
const minKept = 100 * time.Millisecond

// nudge moves the cut being looked at by step, within what the track allows.
func (m Model) nudge(step time.Duration) (tea.Model, tea.Cmd) {
	f := trackFields[m.field]
	r, ok := m.selectedRow()
	if !ok || (f != fCutStart && f != fCutEnd) {
		return m, nil
	}
	t := r.Track
	cur, other := t.TrimStart, t.TrimEnd
	if f == fCutEnd {
		cur, other = other, cur
	}
	next := clamp(int(cur+step), 0, int(max(t.SourceLength-other-minKept, 0)))
	d := time.Duration(next).Round(time.Millisecond)
	if d == cur {
		return m, nil
	}
	return m.setCut(d, t.ID+"\x00"+f.label())
}

// setCut saves the cut being looked at as d.
func (m Model) setCut(d time.Duration, nudge string) (tea.Model, tea.Cmd) {
	f := trackFields[m.field]
	r, ok := m.selectedRow()
	if !ok || (f != fCutStart && f != fCutEnd) {
		return m, nil
	}
	var e app.Edit
	if f == fCutStart {
		e.TrimStart = &d
	} else {
		e.TrimEnd = &d
	}
	m.busy = true
	return m, m.save(r.Track, e, "change of "+strings.ToLower(f.label()), nudge)
}

func (m Model) commit() (tea.Model, tea.Cmd) {
	r, ok := m.selectedRow()
	if !ok {
		return m, nil
	}
	t := r.Track
	e, err := m.dr.edit()
	if err != nil {
		m.editErr = err.Error()
		return m, nil
	}
	if orig := newDraft(t, m.dr.field); !m.prefilled && m.dr.text == orig.text && m.dr.second == orig.second {
		m.editing = false // nothing changed, nothing to save
		return m, nil
	}
	m.busy = true
	return m, m.save(t, e, "change of "+strings.ToLower(m.dr.field.label()), "")
}

func (m Model) edited(msg editedMsg) (tea.Model, tea.Cmd) {
	m.busy = false
	if msg.err != nil {
		m.editErr = msg.err.Error()
		return m, nil
	}
	m.editing, m.editErr = false, ""
	m.upsert(msg.after)
	if top := len(m.undoStack) - 1; msg.nudge != "" && msg.nudge == m.nudgeKey && top >= 0 {
		m.undoStack[top].after = []manifest.Track{msg.after} // one more step of the same run
		m.redoStack = nil
	} else {
		m.record(change{label: msg.label, before: []manifest.Track{msg.before}, after: []manifest.Track{msg.after}})
	}
	m.nudgeKey = msg.nudge
	m.say("Saved", false)
	m.revealTrack(msg.after)
	return m, nil
}

// upsert replaces the record with t's ID, or adds it.
func (m *Model) upsert(t manifest.Track) {
	for i := range m.tracks {
		if m.tracks[i].ID == t.ID {
			m.tracks[i] = t
			return
		}
	}
	m.tracks = append(m.tracks, t)
}

// record remembers a change so it can be undone, and forgets what could
// have been redone.
func (m *Model) record(c change) {
	m.undoStack = append(m.undoStack, c)
	if len(m.undoStack) > 200 {
		m.undoStack = m.undoStack[1:]
	}
	m.redoStack = nil
}

// restoredMsg is the result of undoing or redoing a change.
type restoredMsg struct {
	c      change
	undone bool // true for undo, false for redo
	err    error
}

func (m Model) undo() (tea.Model, tea.Cmd) {
	if len(m.undoStack) == 0 {
		m.say("Nothing to undo", false)
		return m, nil
	}
	return m.restoring(m.undoStack[len(m.undoStack)-1], true)
}

func (m Model) redo() (tea.Model, tea.Cmd) {
	if len(m.redoStack) == 0 {
		m.say("Nothing to redo", false)
		return m, nil
	}
	return m.restoring(m.redoStack[len(m.redoStack)-1], false)
}

func (m Model) restoring(c change, undone bool) (tea.Model, tea.Cmd) {
	m.busy = true
	lib := m.lib
	return m, func() tea.Msg {
		tracks := c.after
		if undone {
			tracks = c.before
		}
		for _, t := range tracks {
			if err := lib.Restore(t); err != nil {
				return restoredMsg{c: c, undone: undone, err: err}
			}
		}
		return restoredMsg{c: c, undone: undone}
	}
}

func (m Model) restored(msg restoredMsg) (tea.Model, tea.Cmd) {
	m.busy = false
	if msg.err != nil {
		m.say(msg.err.Error(), true)
		return m, nil
	}
	tracks, verb := msg.c.after, "Redid"
	if msg.undone {
		tracks, verb = msg.c.before, "Undid"
		m.undoStack = m.undoStack[:len(m.undoStack)-1]
		m.redoStack = append(m.redoStack, msg.c)
	} else {
		m.redoStack = m.redoStack[:len(m.redoStack)-1]
		m.undoStack = append(m.undoStack, msg.c)
	}
	for _, t := range tracks {
		m.upsert(t)
	}
	m.say(verb+": "+msg.c.label, false)
	if len(tracks) > 0 {
		m.revealTrack(tracks[0])
	}
	return m, nil
}
