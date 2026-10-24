package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"music-manager/internal/app"
	"music-manager/internal/device"
	"music-manager/internal/render"
)

// opCounts says how many operations of each kind a plan holds.
type opCounts struct{ create, update, stray, remove int }

func countOps(ops []render.Op) opCounts {
	var c opCounts
	for _, op := range ops {
		switch op.Kind {
		case render.Create:
			c.create++
		case render.Update:
			c.update++
		case render.Stray:
			c.stray++
		case render.Remove:
			c.remove++
		}
	}
	return c
}

// writes is how many files would be written.
func (c opCounts) writes() int { return c.create + c.update }

func (c opCounts) text() string {
	var parts []string
	if c.create > 0 {
		parts = append(parts, fmt.Sprintf("%d to create", c.create))
	}
	if c.update > 0 {
		parts = append(parts, fmt.Sprintf("%d to update", c.update))
	}
	if c.stray > 0 {
		parts = append(parts, plural(c.stray, "stray file"))
	}
	if c.remove > 0 {
		parts = append(parts, fmt.Sprintf("%d to delete", c.remove))
	}
	if len(parts) == 0 {
		return "Nothing to do"
	}
	return strings.Join(parts, " · ")
}

// mountCandidates lists the folders directly inside the given roots, which
// is where removable drives are mounted. skip names a folder to leave out,
// such as the user's own folder inside /media.
func mountCandidates(roots []string, skip string) []string {
	var found []string
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		var names []string
		for _, e := range entries {
			if e.IsDir() && e.Name() != skip {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, n := range names {
			found = append(found, filepath.Join(root, n))
		}
	}
	return found
}

// detectMounts finds the drives mounted for the current user.
func detectMounts() []string {
	user := os.Getenv("USER")
	return mountCandidates([]string{"/run/media/" + user, "/media/" + user, "/media"}, user)
}

type syncState int

const (
	syncChoosing syncState = iota
	syncTyping
	syncPlanning
	syncReviewing
	syncConfirming
	syncApplying
	syncFinished
	syncFailed
)

type syncTarget struct {
	label, path string
	custom      bool
}

// syncDialog is the dialog that writes the library folder.
type syncDialog struct {
	state   syncState
	targets []syncTarget
	cursor  int
	folder  textinput.Model
	confirm textinput.Model

	wolfgang bool // write for the Wolfgang WMP01; never for the library folder, never remembered

	target string // "" for the library folder
	where  string // what to call the target
	plan   Plan
	prune  bool
	offset int

	done, total int
	collected   int
	err         string

	spin   spinner.Model
	gen    int
	cancel context.CancelFunc
	events <-chan syncEventMsg
}

func newSyncDialog(root string, mounts []string) *syncDialog {
	d := &syncDialog{spin: spinner.New(spinner.WithSpinner(spinner.MiniDot))}
	d.spin.Style = styles.accent
	d.targets = append(d.targets, syncTarget{"Library folder", root, false})
	for _, m := range mounts {
		d.targets = append(d.targets, syncTarget{filepath.Base(m), m, false})
	}
	d.targets = append(d.targets, syncTarget{label: "Another folder…", custom: true})
	d.folder = textinput.New()
	d.folder.Prompt = ""
	d.folder.Placeholder = "~/Music/player"
	d.folder.PlaceholderStyle = styles.dim
	d.folder.Cursor.SetMode(cursor.CursorStatic)
	d.confirm = textinput.New()
	d.confirm.Prompt = ""
	d.confirm.Cursor.SetMode(cursor.CursorStatic)
	return d
}

func (d *syncDialog) title() string { return "Sync" }
func (d *syncDialog) width() int    { return 84 }

// listRows is how many operations the review shows at once.
const listRows = 10

// warningRows is how many warnings the review shows; the rest are counted.
const warningRows = 4

func (d *syncDialog) hints(k keyMap) []key.Binding {
	apply := key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply"))
	prune := key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "strays"))
	switch d.state {
	case syncChoosing:
		return []key.Binding{k.Field, key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "choose")), k.Close2}
	case syncTyping:
		return []key.Binding{key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "plan")), k.Close2}
	case syncReviewing:
		if len(d.plan.Ops()) == 0 {
			return []key.Binding{key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "close"))}
		}
		return []key.Binding{k.Field, apply, prune, k.Close2}
	case syncConfirming:
		return []key.Binding{key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")), k.Close2}
	case syncFinished, syncFailed:
		return []key.Binding{key.NewBinding(key.WithKeys("enter", "esc"), key.WithHelp("enter", "close"))}
	case syncApplying:
		return nil
	}
	return []key.Binding{k.Close2}
}

func opLine(op render.Op, w int) string {
	var glyph string
	switch op.Kind {
	case render.Create:
		glyph = styles.good.Render("+")
	case render.Update:
		glyph = styles.accent.Render("~")
	case render.Remove:
		glyph = styles.bad.Render("−")
	default:
		glyph = styles.warn.Render("!")
	}
	return fit(" "+glyph+" "+op.Path, w)
}

func (d *syncDialog) lines(w int) []string {
	lines := []string{""}
	switch d.state {
	case syncChoosing:
		lines = append(lines, " "+styles.dim.Render("Where should the music be written?"), "")
		for i, t := range d.targets {
			text := t.label
			if t.path != "" {
				text = fit(t.label, 18) + " " + styles.dim.Render(tilde(t.path))
			}
			if i == d.cursor {
				lines = append(lines, " "+styles.selected.Render(fit(" "+ansiPlain(text), w-4)))
			} else {
				lines = append(lines, "   "+fit(text, w-4))
			}
		}
		lines = append(lines, "", d.formatLine())
	case syncTyping:
		in := d.folder
		in.Width = max(w-14, 10)
		lines = append(lines, " "+styles.dim.Render("Write the music into which folder?"), "", " "+styles.accent.Render("Folder")+" "+in.View())
		if d.err != "" {
			lines = append(lines, "", " "+styles.bad.Render(d.err))
		}
	case syncPlanning:
		lines = append(lines, " "+d.spin.View()+" Preparing "+d.where+"…")
		if d.total > 0 {
			lines = append(lines, "", " "+progressBar(d.done, d.total, w-16)+fmt.Sprintf("  %d/%d", d.done, d.total))
		}
		lines = append(lines, "", " "+styles.dim.Render("Checking every file against its record; esc cancels."))
	case syncReviewing, syncConfirming:
		lines = append(lines, d.reviewLines(w)...)
	case syncApplying:
		lines = append(lines, " "+d.spin.View()+" Writing "+d.where+"…", "", " "+progressBar(d.done, d.total, w-16)+fmt.Sprintf("  %d/%d", d.done, d.total))
	case syncFinished:
		lines = append(lines, " "+styles.good.Render("Done")+" "+d.where, "", " "+d.summary())
	case syncFailed:
		for _, l := range wrap(d.err, w-2) {
			lines = append(lines, " "+styles.bad.Render(l))
		}
	}
	return append(lines, "")
}

// ansiPlain drops colour codes, for text that is drawn in a style of its own.
func ansiPlain(s string) string { return ansi.Strip(s) }

func (d *syncDialog) summary() string {
	c := countOps(d.plan.Ops())
	var parts []string
	if c.create > 0 {
		parts = append(parts, fmt.Sprintf("%d created", c.create))
	}
	if c.update > 0 {
		parts = append(parts, fmt.Sprintf("%d updated", c.update))
	}
	if c.remove > 0 {
		parts = append(parts, fmt.Sprintf("%d deleted", c.remove))
	}
	if d.collected > 0 {
		parts = append(parts, plural(d.collected, "unused file")+" cleared away")
	}
	if len(parts) == 0 {
		return "Nothing needed doing."
	}
	return strings.Join(parts, " · ")
}

func (d *syncDialog) reviewLines(w int) []string {
	ops := d.plan.Ops()
	c := countOps(ops)
	if len(ops) == 0 {
		return []string{" " + styles.bold.Render("Nothing to do") + styles.dim.Render(" · "+d.where+" is up to date.")}
	}
	lines := []string{" " + styles.bold.Render(c.text()) + styles.dim.Render("  →  "+d.where), ""}
	end := min(d.offset+listRows, len(ops))
	for i := d.offset; i < end; i++ {
		lines = append(lines, opLine(ops[i], w-2))
	}
	if len(ops) > listRows {
		lines = append(lines, " "+styles.dim.Render(fmt.Sprintf("%d–%d of %d", d.offset+1, end, len(ops))))
	}
	if ws := d.plan.Warnings(); len(ws) > 0 {
		lines = append(lines, "", " "+styles.warn.Render(plural(len(ws), "warning")+" for the player"))
		for i, wn := range ws {
			if i == warningRows {
				lines = append(lines, " "+styles.dim.Render(fmt.Sprintf("… %d more", len(ws)-i)))
				break
			}
			lines = append(lines, " "+styles.dim.Render(wn.TrackID)+"  "+fit(wn.Message, w-2-len(wn.TrackID)-2))
		}
	}
	lines = append(lines, "")
	switch {
	case c.stray > 0:
		lines = append(lines, " "+styles.dim.Render("Stray files are left alone. p also deletes the strays."))
	case c.remove > 0:
		lines = append(lines, " "+styles.warn.Render("The files marked − will be deleted. p keeps them."))
	}
	if d.state == syncConfirming {
		in := d.confirm
		in.Width = 12
		lines = append(lines, "", " "+styles.warn.Render(fmt.Sprintf("Type delete to remove %s: ", plural(c.remove, "file")))+in.View())
	}
	return lines
}

// syncEventMsg is progress in, or the end of, planning or applying.
type syncEventMsg struct {
	gen         int
	done, total int
	plan        Plan
	err         error
	finished    bool
}

func waitForSync(gen int, events <-chan syncEventMsg) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-events
		if !ok {
			return syncEventMsg{gen: gen, finished: true}
		}
		return ev
	}
}

func (d *syncDialog) stop() {
	if d.cancel != nil {
		d.cancel()
	}
}

// formatLine is the row that says what kind of place the music goes to.
func (d *syncDialog) formatLine() string {
	label := styles.dim.Render("Format  ")
	switch {
	case d.cursor == 0:
		return " " + label + "Generic " + styles.dim.Render("· the library folder is always written the standard way")
	case d.wolfgang:
		return " " + label + styles.accent.Render("Wolfgang WMP01") + " " + styles.dim.Render("· w for the standard way")
	}
	return " " + label + "Generic " + styles.dim.Render("· w for another format")
}

// plan starts working out what writing the target would change.
func (d *syncDialog) planFor(m *Model, target, where string) tea.Cmd {
	d.stop()
	d.wolfgang = d.wolfgang && target != ""
	var dev device.Target
	if d.wolfgang {
		dev, where = device.Wolfgang, where+" · Wolfgang WMP01"
	}
	d.target, d.where = target, where
	d.state, d.err, d.done, d.total, d.prune = syncPlanning, "", 0, 0, false
	d.gen++
	events := make(chan syncEventMsg, 32)
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel, d.events = cancel, events
	lib, gen := m.lib, d.gen
	go func() {
		defer close(events)
		send := func(ev syncEventMsg) {
			select {
			case events <- ev:
			case <-ctx.Done():
			}
		}
		plan, err := lib.PlanSync(app.SyncOptions{Target: target, Device: dev, OnProgress: func(done, total int) {
			send(syncEventMsg{gen: gen, done: done, total: total})
		}})
		send(syncEventMsg{gen: gen, plan: plan, err: err, finished: true})
	}()
	return tea.Batch(d.spin.Tick, waitForSync(gen, events))
}

func (d *syncDialog) applyPlan(m *Model) tea.Cmd {
	d.stop()
	d.state, d.done, d.total = syncApplying, 0, len(d.plan.Ops())
	d.gen++
	events := make(chan syncEventMsg, 32)
	d.events = events
	plan, gen := d.plan, d.gen
	go func() {
		defer close(events)
		err := plan.Apply(func(done, total int) { events <- syncEventMsg{gen: gen, done: done, total: total} })
		events <- syncEventMsg{gen: gen, err: err, finished: true}
	}()
	return tea.Batch(d.spin.Tick, waitForSync(gen, events))
}

func (d *syncDialog) tick(msg spinner.TickMsg) tea.Cmd {
	if d.state != syncPlanning && d.state != syncApplying {
		return nil
	}
	return advance(&d.spin, msg)
}

func (d *syncDialog) key(m *Model, msg tea.KeyMsg) tea.Cmd {
	k := m.keys
	switch d.state {
	case syncChoosing:
		switch {
		case key.Matches(msg, k.Close2):
			m.overlay = nil
		case key.Matches(msg, k.Up):
			d.cursor = max(d.cursor-1, 0)
		case key.Matches(msg, k.Down):
			d.cursor = min(d.cursor+1, len(d.targets)-1)
		case msg.String() == "w":
			d.wolfgang = d.cursor != 0 && !d.wolfgang // the library folder has no other format
		case key.Matches(msg, k.Apply):
			t := d.targets[d.cursor]
			if t.custom {
				d.state = syncTyping
				return d.folder.Focus()
			}
			target := t.path
			if d.cursor == 0 {
				target = "" // the library folder itself
			}
			return d.planFor(m, target, t.label)
		}
	case syncTyping:
		switch {
		case key.Matches(msg, k.Close2):
			d.state = syncChoosing
		case key.Matches(msg, k.Apply):
			path := cleanPath(d.folder.Value())
			if path == "" {
				d.err = "Type the folder to write into"
				return nil
			}
			return d.planFor(m, path, tilde(path))
		default:
			d.err = ""
			var cmd tea.Cmd
			d.folder, cmd = d.folder.Update(msg)
			return cmd
		}
	case syncPlanning:
		if key.Matches(msg, k.Close2) {
			d.stop()
			m.overlay = nil
		}
	case syncReviewing:
		return d.reviewKey(m, msg)
	case syncConfirming:
		switch {
		case key.Matches(msg, k.Close2):
			d.state = syncReviewing
		case key.Matches(msg, k.Apply):
			if strings.TrimSpace(d.confirm.Value()) == "delete" {
				return d.applyPlan(m)
			}
			d.confirm.SetValue("")
		default:
			var cmd tea.Cmd
			d.confirm, cmd = d.confirm.Update(msg)
			return cmd
		}
	case syncFinished, syncFailed:
		if msg.String() == "enter" || msg.String() == "esc" {
			m.overlay = nil
		}
	}
	return nil
}

func (d *syncDialog) reviewKey(m *Model, msg tea.KeyMsg) tea.Cmd {
	k := m.keys
	ops := d.plan.Ops()
	last := max(len(ops)-listRows, 0)
	switch {
	case key.Matches(msg, k.Close2):
		m.overlay = nil
	case key.Matches(msg, k.Up):
		d.offset = max(d.offset-1, 0)
	case key.Matches(msg, k.Down):
		d.offset = min(d.offset+1, last)
	case key.Matches(msg, k.PageUp):
		d.offset = max(d.offset-listRows, 0)
	case key.Matches(msg, k.PageDown):
		d.offset = min(d.offset+listRows, last)
	case msg.String() == "p":
		d.prune = !d.prune
		d.plan = d.plan.WithPrune(d.prune)
		d.offset = 0
	case key.Matches(msg, k.Apply):
		switch c := countOps(ops); {
		case len(ops) == 0:
			m.overlay = nil
		case c.remove > 0:
			d.state = syncConfirming
			d.confirm.SetValue("")
			return d.confirm.Focus()
		default:
			return d.applyPlan(m)
		}
	}
	return nil
}

// openSync is the s key.
func (m Model) openSync() (tea.Model, tea.Cmd) {
	mounts := m.mounts
	if mounts == nil {
		mounts = detectMounts
	}
	d := newSyncDialog(m.lib.Root(), mounts())
	d.wolfgang = m.wolfgang // a drive starts on the format that was asked for; the library folder never takes it
	m.overlay = d
	return m, nil
}

// syncEvent handles progress in, and the end of, the sync dialog's work.
func (m Model) syncEvent(msg syncEventMsg) (tea.Model, tea.Cmd) {
	d, ok := m.overlay.(*syncDialog)
	if !ok || d.gen != msg.gen {
		return m, nil
	}
	if !msg.finished {
		d.done, d.total = msg.done, msg.total
		return m, waitForSync(d.gen, d.events)
	}
	switch d.state {
	case syncPlanning:
		if msg.err != nil {
			d.state, d.err = syncFailed, msg.err.Error()
			return m, nil
		}
		d.plan, d.state, d.offset = msg.plan, syncReviewing, 0
		if len(msg.plan.Ops()) == 0 && d.target == "" { // the library folder is in sync: remember it
			return m, m.recordSynced()
		}
	case syncApplying:
		if msg.err != nil {
			d.state, d.err = syncFailed, msg.err.Error()
			return m, nil
		}
		d.collected, d.state = d.plan.Collected(), syncFinished
		if d.target == "" || d.collected > 0 { // the files of a removed track go with any sync, whatever it writes to
			m.undoStack, m.redoStack = nil, nil // what a sync deleted cannot be brought back
		}
		if d.target == "" {
			return m, m.refreshSync(false)
		}
	}
	return m, nil
}
