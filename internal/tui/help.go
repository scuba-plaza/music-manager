package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// helpScreen lists every key, grouped by what it is for.
type helpScreen struct{}

type helpEntry struct{ keys, what string }

type helpSection struct {
	name    string
	entries []helpEntry
}

var helpLeft = []helpSection{
	{"Moving", []helpEntry{
		{"↑↓  j k", "move"},
		{"→ ←  l h", "open / close / into fields"},
		{"PgUp PgDn", "a page at a time"},
		{"g G", "top / bottom"},
		{"z", "centre the cursor"},
		{"+  -", "open a level / close all"},
		{"/", "filter  (esc clears it)"},
	}},
	{"Editing a track", []helpEntry{
		{"→  enter", "from tree into fields"},
		{"↑↓", "choose a field"},
		{"← →  1-3", "details, audio, Latin forms"},
		{"enter", "edit it; enter saves"},
		{"esc", "cancel, or back to tree"},
		{"tab", "other number"},
	}},
	{"Cuts, volume, fades", []helpEntry{
		{"[ ]", "nudge 0.1 s or 1.5 dB"},
		{"{ }", "nudge 1 s or 6 dB"},
		{"0", "no cut, fade or change"},
		{"p", "hear 8 s beside it"},
	}},
}

var helpRight = []helpSection{
	{"Several tracks", []helpEntry{
		{"space", "mark (albums mark all)"},
		{"ctrl+a", "mark everything shown"},
		{"e", "edit marked, or fields"},
		{"J K", "move a track down / up"},
		{"n", "number an album 1…N"},
		{"p", "add to a playlist"},
		{"d", "remove (asks first)"},
		{"d", "take out of a playlist"},
	}},
	{"Looking up", []helpEntry{
		{"a", "add from a YouTube link"},
		{"i", "find it on MusicBrainz"},
		{"I", "identify everything"},
		{"o", "use the file's old tags"},
	}},
	{"Library", []helpEntry{
		{"s", "sync: write the folder"},
		{"S", "sort the list"},
		{"u  ctrl+r", "undo / redo"},
		{"D", "check the library"},
		{"?  q", "this help / quit"},
	}},
}

func (helpScreen) title() string { return "Keys" }
func (helpScreen) width() int    { return 84 }

func column(sections []helpSection, w int) []string {
	var lines []string
	for i, s := range sections {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, styles.accent.Render(styles.bold.Render(s.name)))
		for _, e := range s.entries {
			lines = append(lines, fit(styles.key.Render(fit(e.keys, 11))+" "+e.what, w))
		}
	}
	return lines
}

func (helpScreen) lines(w int) []string {
	half := (w - 3) / 2
	left, right := column(helpLeft, half), column(helpRight, half)
	n := max(len(left), len(right))
	lines := []string{""}
	for i := 0; i < n; i++ {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		lines = append(lines, " "+fit(l, half)+" "+r)
	}
	return append(lines, "")
}

func (helpScreen) hints(k keyMap) []key.Binding {
	return []key.Binding{key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "closes"))}
}

func (helpScreen) key(m *Model, msg tea.KeyMsg) tea.Cmd {
	switch strings.ToLower(msg.String()) {
	case "esc", "?", "q", "enter":
		m.overlay = nil
	}
	return nil
}
