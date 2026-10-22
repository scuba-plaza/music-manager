package tui

import (
	"strings"

	"music-manager/internal/manifest"
)

// This file is the left pane: moving through the tree, opening and closing
// it, and drawing it.

// leftInner is the width of the browser pane inside its border.
func (m Model) leftInner() int { return computeLayout(m.w, m.h).left - 2 }

// viewRows is how many tree rows fit in the pane (one line goes to the filter).
func (m Model) viewRows() int { return max(computeLayout(m.w, m.h).body-3, 1) }

// rebuild derives the rows again, keeping the selected row selected if it
// still exists, and otherwise the row at the same position.
func (m *Model) rebuild() {
	m.rows = buildRows(m.tracks, m.filter, m.expanded)
	if m.selKey != "" {
		for i, r := range m.rows {
			if r.Key == m.selKey {
				m.cursor = i
				m.scrollToCursor()
				m.checkFocus()
				return
			}
		}
	}
	m.cursor = clamp(m.cursor, 0, max(len(m.rows)-1, 0))
	m.syncSelection()
	m.scrollToCursor()
	m.checkFocus()
}

// checkFocus hands the keyboard back to the tree if what is selected has no
// fields to edit.
func (m *Model) checkFocus() {
	if r, ok := m.selectedRow(); !ok || r.Kind != kindTrack {
		m.focus, m.editing = paneBrowser, false
	}
}

// syncSelection records which row the cursor is on.
func (m *Model) syncSelection() {
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		m.selKey = m.rows[m.cursor].Key
	} else {
		m.selKey = ""
	}
}

// selectedRow is the row the cursor is on, if there is one.
func (m Model) selectedRow() (Row, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return Row{}, false
	}
	return m.rows[m.cursor], true
}

func (m *Model) moveTo(i int) {
	if len(m.rows) == 0 {
		return
	}
	m.cursor = clamp(i, 0, len(m.rows)-1)
	m.syncSelection()
	m.scrollToCursor()
}

func (m *Model) moveBy(n int) { m.moveTo(m.cursor + n) }

// scrollToCursor scrolls the list the least that keeps the cursor in view.
func (m *Model) scrollToCursor() {
	view := m.viewRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+view {
		m.offset = m.cursor - view + 1
	}
	m.offset = clamp(m.offset, 0, max(len(m.rows)-view, 0))
}

func (m *Model) setOpen(key string, open bool) {
	if open {
		m.expanded[key] = true
	} else {
		delete(m.expanded, key)
	}
	m.rebuild()
}

// open opens the selected row, or steps into it if it is open already.
func (m *Model) open() {
	r, ok := m.selectedRow()
	if !ok || r.Kind == kindTrack {
		return
	}
	if !r.Expanded {
		m.setOpen(r.Key, true)
		return
	}
	if m.cursor+1 < len(m.rows) && m.rows[m.cursor+1].Parent == r.Key {
		m.moveBy(1)
	}
}

// close closes the selected row, or moves up to its parent if it has
// nothing to close.
func (m *Model) close() {
	r, ok := m.selectedRow()
	if !ok {
		return
	}
	if r.Kind != kindTrack && r.Expanded && m.filter == "" {
		m.setOpen(r.Key, false)
		return
	}
	for i := m.cursor - 1; i >= 0; i-- {
		if m.rows[i].Key == r.Parent {
			m.moveTo(i)
			return
		}
	}
}

func (m *Model) toggle() {
	r, ok := m.selectedRow()
	if !ok || r.Kind == kindTrack || m.filter != "" {
		return
	}
	m.setOpen(r.Key, !r.Expanded)
}

// revealTrack opens whatever hides the track and selects it.
func (m *Model) revealTrack(t manifest.Track) {
	for _, k := range ancestorKeys(t) {
		m.expanded[k] = true
	}
	m.selKey = trackKey(t.ID)
	m.rebuild()
}

// browserContent is what the left pane shows, with its scrollbar if the
// list is longer than the pane (one entry per content line; "" for none).
func (m Model) browserContent(w int) (lines, bar []string) {
	lines = m.browserLines(w)
	if len(m.rows) == 0 || !m.loaded {
		return lines, nil
	}
	view := m.viewRows()
	if cells := scrollbar(len(m.rows), view, m.offset, view); cells != nil {
		bar = append([]string{""}, cells...) // the first line is the filter
	}
	return lines, bar
}

func (m Model) browserLines(w int) []string {
	lines := []string{" " + m.filterInput.View()}
	switch {
	case m.loadErr != nil:
		return append(lines, wrap(styles.bad.Render(" Could not read the library: "+m.loadErr.Error()), w)...)
	case !m.loaded:
		return append(lines, styles.dim.Render(" Loading…"))
	case len(m.tracks) == 0:
		return append(lines,
			"",
			" "+styles.bold.Render("No tracks yet"),
			"",
			styles.dim.Render(" Press a to add one from YouTube,"),
			styles.dim.Render(" or run: mm ingest <folder>"),
		)
	case len(m.rows) == 0:
		return append(lines, "", " "+styles.bold.Render("No tracks match")+" "+styles.dim.Render("“"+strings.TrimSpace(m.filter)+"”"))
	}

	view := m.viewRows()
	end := min(m.offset+view, len(m.rows))
	for i := m.offset; i < end; i++ {
		lines = append(lines, renderRow(m.rows[i], w, i == m.cursor))
	}
	return lines
}

// renderRow draws one tree row w columns wide.
func renderRow(r Row, w int, selected bool) string {
	indent := strings.Repeat("  ", r.Depth)
	glyph := " "
	switch {
	case r.Kind != kindTrack && r.Expanded:
		glyph = "▾"
	case r.Kind != kindTrack:
		glyph = "▸"
	}

	label := r.Label
	if r.Num != "" {
		label = r.Num + " " + label
	}
	right := ""
	switch r.Kind {
	case kindTrack:
		right = clock(r.Track.PlayingLength())
	default:
		right = itoa(r.Count)
	}

	if selected {
		left := " " + indent + glyph + " " + label
		if r.Detail != "" {
			left += "  " + r.Detail
		}
		return styles.selected.Render(fit(spread(left, right+" ", w), w))
	}

	st := styles.text
	if r.Kind == kindArtist || r.Kind == kindGroup {
		st = styles.bold
	}
	left := " " + indent + styles.dim.Render(glyph) + " "
	if r.Num != "" {
		left += styles.dim.Render(r.Num) + " "
	}
	left += st.Render(r.Label)
	if r.Detail != "" {
		left += styles.dim.Render("  " + r.Detail)
	}
	return fit(spread(left, styles.dim.Render(right)+" ", w), w)
}
