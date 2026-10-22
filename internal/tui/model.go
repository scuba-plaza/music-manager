package tui

import (
	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/manifest"
)

// Model is the whole interface. It follows the usual Bubble Tea shape: every
// event arrives in Update, which returns the new model and any work to do in
// the background; View draws the current state and nothing else.
type Model struct {
	lib  Library
	keys keyMap

	w, h int

	loaded  bool
	loadErr error
	tracks  []manifest.Track

	// The tree. rows is derived from tracks, filter and expanded by rebuild;
	// selKey, not cursor, is what the user selected, so the selection
	// survives a rebuild.
	expanded    map[string]bool
	filter      string
	filterInput textinput.Model
	filtering   bool
	rows        []Row
	cursor      int
	offset      int
	selKey      string
}

// New returns the interface on lib. Init loads the tracks.
func New(lib Library) Model {
	in := textinput.New()
	in.Prompt = "/ "
	in.PromptStyle = styles.accent
	in.Placeholder = "filter  (e.g. shawn, status:unidentified)"
	in.PlaceholderStyle = styles.dim
	in.Cursor.SetMode(cursor.CursorStatic)
	return Model{lib: lib, keys: newKeyMap(), expanded: map[string]bool{}, filterInput: in}
}

type loadedMsg struct {
	tracks []manifest.Track
	err    error
}

func (m Model) load() tea.Cmd {
	return func() tea.Msg {
		ts, err := m.lib.Tracks()
		return loadedMsg{ts, err}
	}
}

func (m Model) Init() tea.Cmd { return m.load() }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.filterInput.Width = max(m.leftInner()-6, 1)
		m.scrollToCursor()
		return m, nil
	case loadedMsg:
		m.loaded, m.loadErr, m.tracks = true, msg.err, msg.tracks
		m.rebuild()
		return m, nil
	case tea.KeyMsg:
		if m.filtering {
			return m.updateFilter(msg)
		}
		return m.updateBrowser(msg)
	}
	return m, nil
}

func (m Model) updateBrowser(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	switch {
	case key.Matches(msg, k.Quit):
		return m, tea.Quit
	case key.Matches(msg, k.Filter):
		m.filtering = true
		return m, m.filterInput.Focus()
	case key.Matches(msg, k.ClearFilter) && m.filter != "":
		m.clearFilter()
	case key.Matches(msg, k.Up):
		m.moveBy(-1)
	case key.Matches(msg, k.Down):
		m.moveBy(1)
	case key.Matches(msg, k.PageUp):
		m.moveBy(-max(m.viewRows()-1, 1))
	case key.Matches(msg, k.PageDown):
		m.moveBy(max(m.viewRows()-1, 1))
	case key.Matches(msg, k.Top):
		m.moveTo(0)
	case key.Matches(msg, k.Bottom):
		m.moveTo(len(m.rows) - 1)
	case key.Matches(msg, k.Open):
		m.open()
	case key.Matches(msg, k.Close):
		m.close()
	case key.Matches(msg, k.Toggle):
		m.toggle()
	}
	return m, nil
}

func (m Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.FilterClear):
		m.clearFilter()
		return m, nil
	case key.Matches(msg, m.keys.FilterKeep):
		m.filtering = false
		m.filterInput.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	if v := m.filterInput.Value(); v != m.filter {
		m.filter = v
		m.selKey, m.cursor, m.offset = "", 0, 0 // a new search starts at its first match
		m.rebuild()
	}
	return m, cmd
}

func (m *Model) clearFilter() {
	m.filter = ""
	m.filtering = false
	m.filterInput.SetValue("")
	m.filterInput.Blur()
	m.rebuild()
}
