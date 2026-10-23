package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"music-manager/internal/manifest"
)

// doubleClick is how close in time two clicks on a row must be.
const doubleClick = 400 * time.Millisecond

// Screen geometry the mouse relies on: row 0 is the header, row 1 the top
// border of the panes, row 2 the first line inside them (the filter line in
// the tree), row 3 the first tree row.
const (
	firstContentRow = 2
	firstTreeRow    = 3
)

// fieldStart says at which line of the inspector a field starts.
type fieldStart struct{ field, line int }

// fieldStarts returns, for each field of the tab shown, the line of the
// inspector at which it starts, so a click can be traced to a field.
func (m Model) fieldStarts(t manifest.Track, w, h int) []fieldStart {
	tab := m.tab()
	line := 3                          // the strip of tabs, the title and a rule
	line += len(m.tabNotes(t, tab, w)) // what is said before the fields
	var starts []fieldStart
	for i, f := range trackFields {
		if tabOf(f) != tab {
			continue
		}
		starts = append(starts, fieldStart{i, line})
		line += len(m.fieldLines(t, f, false, w))
		if f == fCutEnd {
			line++ // the bar of what the cuts keep
		}
		if f == fFadeOut {
			line += len(m.levelLines(t, w)) // the level line, if there are fades
		}
	}
	return starts
}

func (m Model) mouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.overlay != nil || m.busy || m.editing {
		return m, nil
	}
	l := computeLayout(m.w, m.h)
	if l.mode == modeTooSmall {
		return m, nil
	}
	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && msg.Y == 0 {
		return m.clickHeader(msg)
	}
	inInspector := m.focus == paneInspector && (l.mode == modeOnePane || msg.X >= l.left)
	if l.mode == modeTwoPane && msg.X >= l.left {
		inInspector = true
	}

	switch {
	case msg.Button == tea.MouseButtonWheelDown && msg.Action == tea.MouseActionPress:
		if !inInspector {
			m.moveBy(3)
		}
	case msg.Button == tea.MouseButtonWheelUp && msg.Action == tea.MouseActionPress:
		if !inInspector {
			m.moveBy(-3)
		}
	case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
		if inInspector {
			return m.clickInspector(msg, l)
		}
		return m.clickTree(msg)
	}
	return m, nil
}

// clickHeader opens the sync screen when the note about tracks waiting to
// be synced is what was clicked.
func (m Model) clickHeader(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	chip := m.syncChip()
	if chip == "" || msg.X < m.w-1-ansi.StringWidth(chip) || msg.X >= m.w-1 {
		return m, nil
	}
	return m.openSync()
}

func (m Model) clickTree(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	i := m.offset + msg.Y - firstTreeRow
	if msg.Y < firstTreeRow || i < 0 || i >= len(m.rows) || i >= m.offset+m.viewRows() {
		return m, nil
	}
	if m.focus == paneInspector {
		m.focus = paneBrowser
	}
	row := m.rows[i]
	now := m.clock()
	double := row.Key == m.lastClickKey && now.Sub(m.lastClick) < doubleClick
	m.lastClickKey, m.lastClick = row.Key, now

	wasSelected := i == m.cursor
	m.moveTo(i)
	switch {
	case double && row.Kind != kindTrack && m.filter == "":
		m.setOpen(row.Key, !row.Expanded)
		m.lastClickKey = ""
	case double && row.Kind == kindTrack:
		m.focusInspector()
		m.lastClickKey = ""
		return m, m.startEdit()
	case wasSelected && row.Kind == kindTrack:
		m.focusInspector()
	}
	return m, nil
}

func (m Model) clickInspector(msg tea.MouseMsg, l layoutInfo) (tea.Model, tea.Cmd) {
	r, ok := m.selectedRow()
	if !ok || r.Kind != kindTrack {
		return m, nil
	}
	w := l.right - 2
	if l.mode == modeOnePane {
		w = l.left - 2
	}
	line := msg.Y - firstContentRow
	if line == 0 { // the strip of tabs
		left := l.left
		if l.mode == modeOnePane {
			left = 0
		}
		_, spans := m.tabStrip()
		for i, s := range spans {
			if x := msg.X - left - 1; x >= s[0] && x < s[1] {
				m.focus, m.field = paneInspector, firstFieldOf(i)
			}
		}
		return m, nil
	}
	starts := m.fieldStarts(r.Track, w, l.body-2)
	field := -1
	for _, s := range starts {
		if line >= s.line {
			field = s.field
		}
	}
	if len(starts) == 0 || line < starts[0].line || field < 0 {
		return m, nil
	}
	m.focus = paneInspector
	if m.field == field {
		return m, m.startEdit()
	}
	m.field = field
	return m, nil
}

// clock is the current time; tests replace it.
func (m Model) clock() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}
