package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

// By default the library is a tree of artists and albums. Sorting replaces it
// with one flat list of every track, in the order of one of their fields.

// sortedNotTree is what the tree keys say while the list is sorted.
const sortedNotTree = "The list is sorted, not a tree · S changes that"

// buildSortedRows lays the tracks out as one flat list in the order of key
// (turned around if reverse), after the filter, if there is one.
func buildSortedRows(tracks []manifest.Track, filter string, key app.SortKey, reverse bool) []Row {
	if strings.TrimSpace(filter) != "" {
		tracks = (&manifest.Manifest{Tracks: tracks}).Search(filter)
	}
	var rows []Row
	for _, t := range app.SortTracks(tracks, key, reverse) {
		artist, _, album := app.Display(t)
		detail := artist
		if key == app.SortAlbum {
			detail = album
		}
		rows = append(rows, Row{
			Kind: kindTrack, Key: trackKey(t.ID), Label: needsLabel(t), Detail: detail,
			Right: sortedValue(t, key), Track: t,
		})
	}
	return rows
}

// sortedValue is what to show at the right of a track in a list sorted by
// key: its date or number, or a dash if it has none. Other keys show the
// length, as the tree does, and so give "".
func sortedValue(t manifest.Track, key app.SortKey) string {
	switch key {
	case app.SortReleased, app.SortAdded, app.SortEdited, app.SortTrack:
		if v := app.SortValue(t, key); v != "" {
			return v
		}
		return "—"
	}
	return ""
}

// sortDescending says whether the list runs from high to low: dates start
// with the newest, everything else with the lowest, until reversed.
func (m Model) sortDescending() bool { return m.sortKey.IsDate() != m.sortReverse }

// libraryTitle is the title of the left pane: it names the sort, and which
// way it runs.
func (m Model) libraryTitle() string {
	if m.sortKey == "" {
		return "Library"
	}
	arrow := "▲"
	if m.sortDescending() {
		arrow = "▼"
	}
	return "Library · " + m.sortKey.Label() + " " + arrow
}

// setSort puts the list in the order of key, or back into the tree if key is
// empty, keeping the selected track selected.
func (m *Model) setSort(key app.SortKey, reverse bool) {
	if key == "" {
		reverse = false
	}
	sel, ok := m.selectedRow()
	if ok && key != "" && sel.Kind != kindTrack && len(sel.IDs) > 0 {
		m.selKey = trackKey(sel.IDs[0]) // a list has only tracks: take the first one under the row
	}
	m.sortKey, m.sortReverse = key, reverse
	if ok && key == "" && sel.Kind == kindTrack {
		m.revealTrack(sel.Track) // the tree opens up to show it
		return
	}
	m.rebuild()
}

// sortChoices are the entries of the sort menu: the tree, then every key.
var sortChoices = append([]app.SortKey{""}, app.SortKeys...)

// sortMenu is the dialog that chooses how the list is ordered.
type sortMenu struct {
	cursor  int
	reverse bool
}

func newSortMenu(current app.SortKey, reverse bool) *sortMenu {
	s := &sortMenu{reverse: reverse}
	for i, c := range sortChoices {
		if c == current {
			s.cursor = i
		}
	}
	return s
}

func (s *sortMenu) title() string { return "Sort" }
func (s *sortMenu) width() int    { return 46 }

// direction says in words how a key's order runs.
func direction(k app.SortKey, reverse bool) string {
	pick := func(forward, backward string) string {
		if reverse {
			return backward
		}
		return forward
	}
	switch k {
	case app.SortArtist, app.SortAlbum, app.SortTitle:
		return pick("A → Z", "Z → A")
	case app.SortTrack:
		return pick("1 → 9", "9 → 1")
	case app.SortLength:
		return pick("shortest first", "longest first")
	case app.SortReleased, app.SortAdded, app.SortEdited:
		return pick("newest first", "oldest first")
	}
	return "artist → album → track"
}

func (s *sortMenu) lines(w int) []string {
	lines := []string{""}
	for i, c := range sortChoices {
		label := "Tree"
		if c != "" {
			label = c.Label()
		}
		name := " " + styles.dim.Render(string(rune('1'+i))) + "  " + label
		hint := styles.dim.Render(direction(c, s.reverse && c != "") + " ")
		if i == s.cursor {
			name = " " + styles.accent.Render("▌"+string(rune('1'+i))) + " " + styles.accent.Render(label)
		}
		lines = append(lines, spread(name, hint, w))
	}
	state := "off"
	if s.reverse {
		state = "on"
	}
	return append(lines, "", " "+styles.key.Render("r")+" "+styles.keyDesc.Render("reverse: "+state), "")
}

func (s *sortMenu) hints(k keyMap) []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "choose")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reverse")),
		k.Close2,
	}
}

func (s *sortMenu) key(m *Model, msg tea.KeyMsg) tea.Cmd {
	k := m.keys
	switch text := msg.String(); {
	case key.Matches(msg, k.Close2), text == "q", text == "S":
		m.overlay = nil
	case key.Matches(msg, k.Up):
		s.cursor = max(s.cursor-1, 0)
	case key.Matches(msg, k.Down):
		s.cursor = min(s.cursor+1, len(sortChoices)-1)
	case text == "r":
		s.reverse = !s.reverse
	case key.Matches(msg, k.Apply):
		s.choose(m, s.cursor)
	case len(text) == 1 && text[0] >= '1' && int(text[0]-'1') < len(sortChoices):
		s.choose(m, int(text[0]-'1'))
	}
	return nil
}

func (s *sortMenu) choose(m *Model, i int) {
	m.overlay = nil
	m.setSort(sortChoices[i], s.reverse)
}
