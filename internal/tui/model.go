package tui

import (
	"github.com/charmbracelet/bubbles/key"
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
}

// New returns the interface on lib. Init loads the tracks.
func New(lib Library) Model {
	return Model{lib: lib, keys: newKeyMap()}
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
	case loadedMsg:
		m.loaded, m.loadErr, m.tracks = true, msg.err, msg.tracks
	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
	}
	return m, nil
}
