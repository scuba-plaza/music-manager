package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/x/ansi"

	"music-manager/internal/app"

	"music-manager/internal/manifest"
)

func (m Model) View() string {
	if m.w == 0 || m.h == 0 {
		return ""
	}
	l := computeLayout(m.w, m.h)
	if l.mode == modeTooSmall {
		return m.tooSmallView()
	}
	return m.overlayView(strings.Join([]string{m.headerView(), m.bodyView(l), m.footerView()}, "\n"))
}

func (m Model) tooSmallView() string {
	msg := []string{
		styles.bold.Render("Terminal too small"),
		styles.dim.Render(fmt.Sprintf("mm needs at least %d×%d, this is %d×%d", minWidth, minHeight, m.w, m.h)),
		styles.dim.Render("Make the window bigger, or q to quit"),
	}
	out := make([]string, m.h)
	top := (m.h - len(msg)) / 2
	for i := range out {
		line := ""
		if j := i - top; j >= 0 && j < len(msg) {
			pad := max((m.w-ansi.StringWidth(msg[j]))/2, 0)
			line = strings.Repeat(" ", pad) + msg[j]
		}
		out[i] = fit(line, m.w)
	}
	return strings.Join(out, "\n")
}

func (m Model) headerView() string {
	left := " " + styles.header.Render("mm") + "  " + styles.dim.Render(tilde(m.lib.Root()))
	right := styles.headerRight.Render(m.summary())
	if chip := m.syncChip(); chip != "" {
		right += styles.headerRight.Render(" · ") + styles.accent.Render(chip)
	}
	right += " "
	return fit(spread(left, right, m.w), m.w)
}

// summary is the count shown at the right of the header.
// syncChip is the header's note that the library folder is behind, with the
// key that fixes it. It is empty when nothing waits.
func (m Model) syncChip() string {
	if m.syncN <= 0 {
		return ""
	}
	return fmt.Sprintf("%d to sync · press s", m.syncN)
}

func (m Model) summary() string {
	s := plural(len(m.tracks), "track")
	needs := 0
	for _, t := range m.tracks {
		if t.Status != manifest.Identified {
			needs++
		}
	}
	switch needs {
	case 0:
	case 1:
		s += " · 1 needs work"
	default:
		s += fmt.Sprintf(" · %d need work", needs)
	}
	if n := len(m.marks); n > 0 {
		s += fmt.Sprintf(" · %d marked", n)
	}
	return s
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	suffix := "s"
	for _, end := range []string{"ch", "sh", "s", "x"} {
		if strings.HasSuffix(word, end) {
			suffix = "es"
		}
	}
	return fmt.Sprintf("%d %s%s", n, word, suffix)
}

func (m Model) bodyView(l layoutInfo) string {
	inspector := func() string {
		r, ok := m.selectedRow()
		return box(peekTitle(r, ok), m.inspectorLines(l.right-2, l.body-2), l.right, l.body, m.focus == paneInspector)
	}
	if l.mode == modeOnePane && m.focus == paneInspector {
		return inspector() // one pane at a time: the fields take the place of the tree
	}
	lines, bar := m.browserContent(l.left - 2)
	left := boxWithBar(m.libraryTitle(), lines, bar, l.left, l.body, m.focus == paneBrowser)
	if l.mode == modeOnePane {
		return left
	}
	return joinPanes(left, inspector())
}

// joinPanes puts two boxes of the same height side by side.
func joinPanes(left, right string) string {
	ls, rs := strings.Split(left, "\n"), strings.Split(right, "\n")
	out := make([]string, len(ls))
	for i := range ls {
		out[i] = ls[i] + rs[i]
	}
	return strings.Join(out, "\n")
}

func (m Model) footerView() string {
	line := " "
	if m.playing != nil {
		edge := "start"
		if m.playing.edge == app.EdgeEnd {
			edge = "end"
		}
		line += styles.accent.Render("▶ Playing the "+edge+" cut") + styles.keyDesc.Render("  ·  esc stops  ·  ")
	}
	if m.status != "" {
		st := styles.good
		if m.statusBad {
			st = styles.bad
		}
		line += st.Render(m.status) + styles.keyDesc.Render("  ·  ")
	}
	return fit(line+renderKeysWithin(m.footerKeys(), m.w-ansi.StringWidth(line)), m.w)
}

// renderKeysWithin is renderKeys, but when the keys do not fit in w columns
// it leaves some out, working back from the end, rather than cut one in
// half. The last two (help and quit) always stay.
func renderKeysWithin(bindings []key.Binding, w int) string {
	enabled := make([]key.Binding, 0, len(bindings))
	for _, b := range bindings {
		if b.Enabled() {
			enabled = append(enabled, b)
		}
	}
	for n := len(enabled) - 2; n > 0; n-- {
		kept := append(append([]key.Binding(nil), enabled[:n]...), enabled[len(enabled)-2:]...)
		if s := renderKeys(kept); ansi.StringWidth(s) <= w {
			return s
		}
	}
	return renderKeys(enabled[max(len(enabled)-2, 0):])
}

// renderKeys draws bindings as "key description" pairs.
func renderKeys(bindings []key.Binding) string {
	var parts []string
	for _, b := range bindings {
		if !b.Enabled() {
			continue
		}
		h := b.Help()
		parts = append(parts, styles.key.Render(h.Key)+" "+styles.keyDesc.Render(h.Desc))
	}
	return strings.Join(parts, styles.keyDesc.Render("  ·  "))
}

// wrap breaks s into lines of at most w columns.
func wrap(s string, w int) []string {
	return strings.Split(ansi.Wrap(s, max(w-1, 1), ""), "\n")
}

// footerKeys are the keys worth showing now.
func (m Model) footerKeys() []key.Binding {
	k := m.keys
	switch {
	case m.overlay != nil:
		return m.overlay.hints(k)
	case m.editing:
		bindings := []key.Binding{k.Save, k.Cancel}
		if m.hasSecond() {
			bindings = append(bindings, k.NextInput)
		}
		return bindings
	case m.filtering:
		return []key.Binding{k.FilterKeep, k.FilterClear}
	}

	var bindings []key.Binding
	if r, ok := m.selectedRow(); ok && m.focus == paneInspector && r.Kind != kindTrack {
		bindings = []key.Binding{k.Field, k.Edit, k.Back} // a group has no tabs
		if m.curField() == fCover {
			bindings = append(bindings, k.RemoveCover)
		}
	} else if m.focus == paneInspector {
		bindings = []key.Binding{k.Field, k.Tabs, k.Edit, k.Back}
		switch m.curField() {
		case fCutStart, fCutEnd, fVolume, fFadeIn, fFadeOut:
			bindings = append(bindings, k.Less, k.Play)
		case fCover:
			bindings = append(bindings, k.RemoveCover)
		}
	} else {
		bindings = []key.Binding{k.Up, k.Open, k.Sync, k.Mark, k.Filter, k.Sort, k.Playlist}
		if len(m.marks) > 0 {
			bindings = []key.Binding{k.Up, k.EditMany, k.Mark, k.Playlist, k.ClearMarks}
		}
		if r, ok := m.selectedRow(); ok {
			switch _, in := playlistOf(r); {
			case in && r.Kind == kindPlaylist:
				bindings = []key.Binding{k.Up, k.EditGroup, k.OpenGroup, k.DeletePlaylist, k.Mark}
			case in:
				bindings = []key.Binding{k.Up, k.MoveDown, k.TakeOut, k.Mark, k.Filter}
			case len(m.marks) == 0 && (r.Kind == kindArtist || editableRow(r) && r.Kind == kindAlbum):
				bindings = []key.Binding{k.Up, k.EditGroup, k.OpenGroup, k.Sync, k.Mark, k.Filter, k.Sort, k.Playlist}
			}
		}
		if m.filter != "" {
			bindings = append(bindings, k.ClearFilter)
		}
	}
	if len(m.undoStack) > 0 {
		bindings = append(bindings, k.Undo)
	}
	return append(bindings, k.Help, k.Quit)
}
