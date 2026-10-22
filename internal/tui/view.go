package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) View() string {
	if m.w == 0 || m.h == 0 {
		return ""
	}
	l := computeLayout(m.w, m.h)
	if l.mode == modeTooSmall {
		return m.tooSmallView()
	}
	return strings.Join([]string{m.headerView(), m.bodyView(l), m.footerView()}, "\n")
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
	right := styles.headerRight.Render(plural(len(m.tracks), "track")) + " "
	return fit(spread(left, right, m.w), m.w)
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func (m Model) bodyView(l layoutInfo) string {
	left := box("Library", m.browserLines(l.left-2), l.left, l.body, true)
	if l.mode == modeOnePane {
		return left
	}
	right := box("Details", m.inspectorLines(l.right-2), l.right, l.body, false)
	return joinPanes(left, right)
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

func (m Model) browserLines(w int) []string {
	switch {
	case m.loadErr != nil:
		return wrap(styles.bad.Render("Could not read the library: "+m.loadErr.Error()), w)
	case !m.loaded:
		return []string{styles.dim.Render(" Loading…")}
	case len(m.tracks) == 0:
		return []string{
			"",
			" " + styles.bold.Render("No tracks yet"),
			"",
			styles.dim.Render(" Press a to add one from YouTube,"),
			styles.dim.Render(" or run: mm ingest <folder>"),
		}
	}
	return []string{styles.dim.Render(fmt.Sprintf(" %d tracks", len(m.tracks)))}
}

func (m Model) inspectorLines(w int) []string {
	return []string{"", styles.dim.Render(" Select a track")}
}

func (m Model) footerView() string {
	return fit(" "+renderKeys([]key.Binding{m.keys.Help, m.keys.Quit}), m.w)
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
