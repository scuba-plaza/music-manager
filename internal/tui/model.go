package tui

import (
	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/manifest"
)

// pane is which half of the screen has the keyboard.
type pane int

const (
	paneBrowser pane = iota
	paneInspector
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

	// The inspector: which field the cursor is on and, while one is being
	// edited, its draft and inputs (the text, and a second one for the ASCII
	// form of a name or the size of an album).
	focus     pane
	field     int
	editing   bool
	dr        draft
	prefilled bool // the draft started from the file's old tags, so saving it unchanged still counts
	in        [2]textinput.Model
	inFocus   int
	editErr   string

	// Everything that can be undone, and redone.
	undoStack, redoStack []change

	// One line about what just happened, cleared by the next key.
	status    string
	statusBad bool

	busy bool // a change is being saved

	// Hearing a cut: the player (nil if none is installed), what is playing
	// now, and a counter that tells one preview from the next.
	player   Player
	playing  *playback
	previews int

	// Covers: shrunken copies of the ones shown, loaded in the background.
	thumbs       map[string]thumb
	thumbPending map[string]bool

	// nudgeKey names the cut that was just nudged, so that a run of nudges
	// of one cut is one step to undo.
	nudgeKey string
}

// Option adjusts a Model.
type Option func(*Model)

// WithPlayer lets the interface play previews of the cuts.
func WithPlayer(p Player) Option { return func(m *Model) { m.player = p } }

// change is one undoable step: the tracks as they were and as they became.
type change struct {
	label         string
	before, after []manifest.Track
}

// New returns the interface on lib. Init loads the tracks.
func New(lib Library, opts ...Option) Model {
	in := textinput.New()
	in.Prompt = "/ "
	in.PromptStyle = styles.accent
	in.Placeholder = "filter  (e.g. shawn, status:unidentified)"
	in.PlaceholderStyle = styles.dim
	in.Cursor.SetMode(cursor.CursorStatic)
	m := Model{
		lib: lib, keys: newKeyMap(), expanded: map[string]bool{}, filterInput: in,
		thumbs: map[string]thumb{}, thumbPending: map[string]bool{},
	}
	for _, o := range opts {
		o(&m)
	}
	return m
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
	next, cmd := m.update(msg)
	nm := next.(Model)
	// Whatever happened, the cover now selected may still need loading.
	return nm, tea.Batch(cmd, nm.wantThumb())
}

func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case thumbMsg:
		delete(m.thumbPending, msg.sha)
		m.thumbs[msg.sha] = msg.thumb
		return m, nil
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.filterInput.Width = max(m.leftInner()-6, 1)
		m.scrollToCursor()
		return m, nil
	case loadedMsg:
		m.loaded, m.loadErr, m.tracks = true, msg.err, msg.tracks
		m.rebuild()
		return m, nil
	case editedMsg:
		return m.edited(msg)
	case restoredMsg:
		return m.restored(msg)
	case playbackStartedMsg:
		return m.playbackStarted(msg)
	case playbackDoneMsg:
		return m.playbackDone(msg)
	case tea.KeyMsg:
		m.status, m.statusBad = "", false
		if !m.keys.isNudge(msg) {
			m.nudgeKey = ""
		}
		switch {
		case key.Matches(msg, m.keys.Quit) && !m.editing && !m.filtering:
			m.stopPlayback()
			return m, tea.Quit
		case msg.Type == tea.KeyCtrlC:
			m.stopPlayback()
			return m, tea.Quit
		case m.busy:
			return m, nil
		case msg.Type == tea.KeyEsc && m.playing != nil && !m.editing && !m.filtering:
			m.stopPlayback()
			return m, nil
		case m.editing:
			return m.updateEditing(msg)
		case m.filtering:
			return m.updateFilter(msg)
		case m.focus == paneInspector:
			return m.updateInspector(msg)
		}
		return m.updateBrowser(msg)
	}
	return m, nil
}

func (m Model) updateBrowser(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := m.keys
	switch {
	case key.Matches(msg, k.Filter):
		m.filtering = true
		return m, m.filterInput.Focus()
	case key.Matches(msg, k.ClearFilter) && m.filter != "":
		m.clearFilter()
	case key.Matches(msg, k.Undo):
		return m.undo()
	case key.Matches(msg, k.Redo):
		return m.redo()
	case key.Matches(msg, k.Switch):
		m.focusInspector()
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
		if !m.focusInspector() {
			m.open()
		}
	case key.Matches(msg, k.Close):
		m.close()
	case key.Matches(msg, k.Toggle):
		if !m.focusInspector() {
			m.toggle()
		}
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

// say shows a line in the footer until the next key.
func (m *Model) say(text string, bad bool) { m.status, m.statusBad = text, bad }
