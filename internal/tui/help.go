package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// helpScreen lists every key, one page for each kind of thing the keys do, so
// that no page is crowded. The tabs of the pages run along its top.
type helpScreen struct{ page int }

type helpEntry struct{ keys, what string }

type helpSection struct {
	name    string
	entries []helpEntry
}

type helpPage struct {
	name     string
	sections []helpSection
}

var helpPages = []helpPage{
	{"Moving", []helpSection{
		{"Moving", []helpEntry{
			{"↑↓  j k", "move"},
			{"PgUp PgDn", "a page at a time"},
			{"g G", "top / bottom"},
			{"z", "centre the cursor"},
			{"+  -", "open a level / close all"},
		}},
		{"Opening", []helpEntry{
			{"→  l  e", "into the fields of a track, artist, album or playlist"},
			{"enter", "open / close; on a track, into its fields"},
			{"←  h", "close, or up a level"},
		}},
		{"Finding", []helpEntry{
			{"/", "filter  (esc clears it)"},
			{"S", "sort the list"},
		}},
	}},
	{"Editing", []helpSection{
		{"In the fields", []helpEntry{
			{"↑↓", "choose a field"},
			{"enter", "edit it; enter again saves"},
			{"esc", "cancel, or back to the tree"},
			{"← →  1-3", "a track's tabs: details, audio, Latin forms"},
			{"tab", "a track's other number"},
			{"-", "clear a (mixed) field of an artist or album"},
		}},
		{"Cuts, volume, fades", []helpEntry{
			{"[ ]", "nudge 0.1 s or 1.5 dB"},
			{"{ }", "nudge 1 s or 6 dB"},
			{"0", "no cut, fade or change"},
			{"p", "hear 8 s beside it"},
		}},
	}},
	{"Marking", []helpSection{
		{"Several tracks", []helpEntry{
			{"space", "mark a track (an album marks all its tracks)"},
			{"ctrl+a", "mark everything shown"},
			{"e", "edit the marked tracks together"},
		}},
		{"Copy and paste", []helpEntry{
			{"c", "copy the cover, artist, album… of a track"},
			{"v", "paste them, or a picture copied outside"},
		}},
		{"Order and playlists", []helpEntry{
			{"J K", "move a track down / up"},
			{"n", "number an album 1…N"},
			{"p", "add to a playlist"},
			{"d", "remove (asks first)"},
			{"d", "in a playlist: take a track out of it"},
		}},
	}},
	{"Library", []helpSection{
		{"Looking up", []helpEntry{
			{"a", "add from a YouTube link"},
			{"i", "find it on MusicBrainz"},
			{"I", "identify everything"},
			{"o", "use the file's old tags"},
		}},
		{"Library", []helpEntry{
			{"s", "sync: write the folder"},
			{"u  ctrl+r", "undo / redo"},
			{"D", "check the library"},
			{"?  q", "this help / quit"},
		}},
	}},
}

func (*helpScreen) title() string { return "Keys" }
func (*helpScreen) width() int    { return 70 }

func (h *helpScreen) lines(w int) []string {
	tabs := " "
	for i, p := range helpPages {
		tabs += tabLabel(i, p.name, i == h.page) + " "
	}
	lines := []string{"", tabs, ""}
	for i, s := range helpPages[h.page].sections {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, " "+styles.accent.Render(styles.bold.Render(s.name)))
		for _, e := range s.entries {
			lines = append(lines, " "+fit(styles.key.Render(fit(e.keys, 11))+" "+e.what, w-1))
		}
	}
	for len(lines) < helpHeight()-1 { // every page as tall as the tallest, so the tabs stay put
		lines = append(lines, "")
	}
	return append(lines, "")
}

// helpHeight is how many lines the tallest page takes.
func helpHeight() int {
	tallest := 0
	for _, p := range helpPages {
		n := 4 // a blank line, the tabs, a blank line, and a blank line at the end
		for i, s := range p.sections {
			if i > 0 {
				n++
			}
			n += 1 + len(s.entries)
		}
		tallest = max(tallest, n)
	}
	return tallest
}

func (*helpScreen) hints(k keyMap) []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("left", "right"), key.WithHelp("← →", "pages")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "closes")),
	}
}

func (h *helpScreen) key(m *Model, msg tea.KeyMsg) tea.Cmd {
	last := len(helpPages) - 1
	switch s := strings.ToLower(msg.String()); s {
	case "esc", "?", "q", "enter":
		m.overlay = nil
	case "right", "l":
		h.page = min(h.page+1, last)
	case "left", "h":
		h.page = max(h.page-1, 0)
	case "tab":
		h.page = (h.page + 1) % len(helpPages)
	case "shift+tab":
		h.page = (h.page + last) % len(helpPages)
	default: // the number of a page
		if len(s) == 1 && s[0] > '0' && int(s[0]-'0') <= len(helpPages) {
			h.page = int(s[0]-'0') - 1
		}
	}
	return nil
}
