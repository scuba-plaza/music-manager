package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"music-manager/internal/manifest"
)

// filterHint is what the empty filter box says when there is room.
const filterHint = "filter  (e.g. shawn, status:unidentified)"

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

	// How far the library folder lags behind the records, and what is needed to
	// keep that up to date: rev changes whenever the tracks do.
	syncN        int
	syncKnown    bool
	rev, syncRev int

	// Looking for changes made elsewhere, and telling double clicks apart.
	stamp        string
	now          func() time.Time
	lastClickKey string
	lastClick    time.Time

	baselineTried bool
	mounts        func() []string // the drives to offer; detectMounts if nil

	// runs counts the batch identifications started, to tell one from the next.
	runs int

	// Marked tracks (by id), and the dialog that is open, if any.
	marks   map[string]bool
	overlay overlay

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
	in.Placeholder = filterHint
	in.PlaceholderStyle = styles.dim
	in.Cursor.SetMode(cursor.CursorStatic)
	m := Model{
		lib: lib, keys: newKeyMap(), expanded: map[string]bool{}, filterInput: in,
		thumbs: map[string]thumb{}, thumbPending: map[string]bool{},
		marks: map[string]bool{},
	}
	for _, o := range opts {
		o(&m)
	}
	return m
}

type loadedMsg struct {
	tracks []manifest.Track
	err    error
	stamp  string
}

func (m Model) load() tea.Cmd {
	return func() tea.Msg {
		stamp, _ := m.lib.Stamp() // taken first, so a change during the load is seen next time
		ts, err := m.lib.Tracks()
		return loadedMsg{ts, err, stamp}
	}
}

// polling says whether the interface keeps looking for changes made by other
// programs. Tests switch it off, as they do animation.
var polling = true

// pollEvery is how often the library is looked at for outside changes.
const pollEvery = 2 * time.Second

// pollMsg says it is time to look; stampMsg is what the look found.
type pollMsg struct{}
type stampMsg struct{ stamp string }

func (m Model) poll() tea.Cmd {
	if !polling {
		return nil
	}
	return tea.Tick(pollEvery, func(time.Time) tea.Msg { return pollMsg{} })
}

func (m Model) Init() tea.Cmd { return tea.Batch(m.load(), m.poll()) }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	nm := next.(Model)
	cmds := []tea.Cmd{cmd, nm.wantThumb()} // the cover now selected may need loading
	if nm.loaded && nm.rev != nm.syncRev {
		// The tracks changed: see how far behind the folder is now. Scanning the
		// folder is only ever needed once, if it was never synced.
		nm.syncRev = nm.rev
		cmds = append(cmds, nm.refreshSync(!nm.baselineTried))
		nm.baselineTried = true
	}
	return nm, tea.Batch(cmds...)
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
		m.filterInput.Placeholder = filterHint
		if m.filterInput.Width < ansi.StringWidth(filterHint) {
			m.filterInput.Placeholder = "filter"
		}
		m.scrollToCursor()
		return m, nil
	case loadedMsg:
		m.loaded, m.loadErr, m.tracks, m.stamp = true, msg.err, msg.tracks, msg.stamp
		m.rev++
		m.rebuild()
		return m, nil
	case pollMsg:
		lib := m.lib
		return m, tea.Batch(m.poll(), func() tea.Msg {
			s, _ := lib.Stamp()
			return stampMsg{s}
		})
	case stampMsg:
		if msg.stamp != "" && msg.stamp != m.stamp && !m.busy {
			m.stamp = msg.stamp
			return m, m.load()
		}
		return m, nil
	case doctorMsg:
		return m.doctored(msg)
	case tea.MouseMsg:
		return m.mouse(msg)
	case syncStatusMsg:
		return m.syncStatus(msg)
	case syncEventMsg:
		return m.syncEvent(msg)
	case editedMsg:
		return m.edited(msg)
	case restoredMsg:
		return m.restored(msg)
	case changedMsg:
		return m.changed(msg)
	case removedMsg:
		return m.removed(msg)
	case candidatesMsg:
		if p, ok := m.overlay.(*picker); ok && p.gen == msg.gen {
			p.received(msg)
		}
		return m, nil
	case identifiedMsg:
		return m.identified(msg)
	case identifyEventMsg:
		return m.identifyEvent(msg)
	case fetchedMsg:
		if d, ok := m.overlay.(*addDialog); ok && d.gen == msg.gen {
			d.fetched(&m, msg)
		}
		return m, nil
	case addedMsg:
		return m.added(msg)
	case adoptedMsg:
		return m.adopted(msg)
	case spinner.TickMsg:
		if t, ok := m.overlay.(interface{ tick(spinner.TickMsg) tea.Cmd }); ok {
			return m, t.tick(msg)
		}
		return m, nil
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
		case key.Matches(msg, m.keys.Quit) && !m.editing && !m.filtering && m.overlay == nil:
			m.stopPlayback()
			return m, tea.Quit
		case msg.Type == tea.KeyCtrlC:
			m.stopPlayback()
			return m, tea.Quit
		case m.busy:
			return m, nil
		case m.overlay != nil:
			cmd := m.overlay.key(&m, msg)
			return m, cmd
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
	case key.Matches(msg, k.ClearMarks) && len(m.marks) > 0:
		m.marks = map[string]bool{}
	case key.Matches(msg, k.Mark):
		m.toggleMark()
	case key.Matches(msg, k.MarkAll):
		m.markAll()
	case key.Matches(msg, k.EditMany):
		return m.openBulk()
	case key.Matches(msg, k.Remove):
		return m.openRemove()
	case key.Matches(msg, k.Help):
		m.overlay = helpScreen{}
	case key.Matches(msg, k.Doctor):
		return m.openDoctor()
	case key.Matches(msg, k.Add):
		return m.openAdd()
	case key.Matches(msg, k.Sync):
		return m.openSync()
	case key.Matches(msg, k.Identify):
		return m.openPicker()
	case key.Matches(msg, k.IdentifyAll):
		return m.openIdentifyAll()
	case key.Matches(msg, k.Adopt):
		return m.adoptOld()
	case key.Matches(msg, k.MoveDown):
		return m.moveTrack(1)
	case key.Matches(msg, k.MoveUp):
		return m.moveTrack(-1)
	case key.Matches(msg, k.Renumber):
		return m.renumber()
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
