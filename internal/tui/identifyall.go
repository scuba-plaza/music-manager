package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

// progressBar draws done/total of width cells.
func progressBar(done, total, width int) string {
	filled := 0
	if total > 0 {
		filled = clamp(done*width/total, 0, width)
	}
	return styles.accent.Render(strings.Repeat("█", filled)) + styles.dim.Render(strings.Repeat("░", width-filled))
}

type runState int

const (
	runRunning runState = iota
	runFinished
	runStopped
)

// identifyRun is the dialog that shows "identify everything that needs work"
// while it runs in the background, and what came of it afterwards.
type identifyRun struct {
	gen                      int
	total, done              int
	identified, left, failed int
	failures                 []string
	state                    runState
	cancel                   context.CancelFunc
	events                   <-chan identifyEventMsg
	before, after            []manifest.Track // the tracks it identified, as they were and became
}

// identifyEventMsg is one thing that happened in a run.
type identifyEventMsg struct {
	gen   int
	res   app.IdentifyResult
	track manifest.Track // the track as saved, if it was identified
	done  bool
	err   error
}

func (r *identifyRun) title() string { return "Identify all" }
func (r *identifyRun) width() int    { return 64 }

func (r *identifyRun) lines(w int) []string {
	head := "Identifying tracks on MusicBrainz…"
	switch r.state {
	case runFinished:
		head = "Done"
	case runStopped:
		head = "Stopped"
	}
	lines := []string{"", " " + styles.bold.Render(head), ""}
	lines = append(lines, " "+progressBar(r.done, r.total, w-14)+fmt.Sprintf("  %d/%d", r.done, r.total), "")
	lines = append(lines, fmt.Sprintf(" Identified %d · Left for you %d · Failed %d", r.identified, r.left, r.failed))
	for i, f := range r.failures {
		if i == 3 {
			lines = append(lines, " "+styles.dim.Render("…and more"))
			break
		}
		for _, l := range wrap(f, w-3) {
			lines = append(lines, " "+styles.warn.Render(l))
		}
	}
	if r.state != runRunning && r.left > 0 {
		lines = append(lines, "")
		for _, l := range wrap("The rest need a closer look: i searches for one track, o uses its old tags.", w-3) {
			lines = append(lines, " "+styles.dim.Render(l))
		}
	}
	return append(lines, "")
}

func (r *identifyRun) hints(k keyMap) []key.Binding {
	if r.state == runRunning {
		return []key.Binding{key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "stop"))}
	}
	return []key.Binding{key.NewBinding(key.WithKeys("enter", "esc"), key.WithHelp("enter", "close"))}
}

func (r *identifyRun) key(m *Model, msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "enter":
		if r.state == runRunning {
			r.cancel()
			r.state = runStopped
			m.recordRun(r)
			return nil
		}
		m.overlay = nil
	}
	return nil
}

// openIdentifyAll is the I key.
func (m Model) openIdentifyAll() (tea.Model, tea.Cmd) {
	if !m.lib.Lookup() {
		m.say("Lookup is not available: no MusicBrainz client is set up", true)
		return m, nil
	}
	need := 0
	for _, t := range m.tracks {
		if t.Status != manifest.Identified {
			need++
		}
	}
	if need == 0 {
		m.say("Nothing needs work", false)
		return m, nil
	}
	m.overlay = &confirm{
		head: fmt.Sprintf("Identify %s on MusicBrainz?", plural(need, "track")),
		body: []string{
			"Each one is searched for, and the best match is taken only if it is a confident one (a score of 95 or more). The rest are left alone for you to look at.",
			"",
			styles.dim.Render("This takes about a second per track, and u undoes it all in one step."),
		},
		yesText: "identify",
		yes: func(m *Model) tea.Cmd {
			return m.startIdentifyAll(need)
		},
	}
	return m, nil
}

func (m *Model) startIdentifyAll(total int) tea.Cmd {
	m.runs++
	events := make(chan identifyEventMsg, 32)
	ctx, cancel := context.WithCancel(context.Background())
	run := &identifyRun{gen: m.runs, total: total, cancel: cancel, events: events}
	m.overlay = run

	lib, gen := m.lib, run.gen
	go func() {
		defer close(events)
		send := func(ev identifyEventMsg) {
			select {
			case events <- ev:
			case <-ctx.Done():
			}
		}
		_, err := lib.IdentifyAll(ctx, func(r app.IdentifyResult, t manifest.Track) {
			send(identifyEventMsg{gen: gen, res: r, track: t})
		})
		send(identifyEventMsg{gen: gen, done: true, err: err})
	}()
	return waitForIdentify(gen, events)
}

func waitForIdentify(gen int, events <-chan identifyEventMsg) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-events
		if !ok {
			return identifyEventMsg{gen: gen, done: true}
		}
		return ev
	}
}

func (m Model) identifyEvent(msg identifyEventMsg) (tea.Model, tea.Cmd) {
	run, ok := m.overlay.(*identifyRun)
	if !ok || run.gen != msg.gen || run.state != runRunning {
		return m, nil
	}
	if msg.done {
		run.state = runFinished
		if msg.err != nil && len(run.failures) == 0 {
			run.failures = append(run.failures, msg.err.Error())
		}
		m.recordRun(run)
		return m, nil
	}

	run.done++
	switch r := msg.res; {
	case r.Err != nil:
		run.failed++
		run.failures = append(run.failures, r.Err.Error())
	case r.Applied != nil:
		run.identified++
		for _, t := range m.tracks {
			if t.ID == msg.track.ID {
				run.before = append(run.before, t)
			}
		}
		run.after = append(run.after, msg.track)
		m.upsert(msg.track)
		m.rebuild()
	default:
		run.left++
	}
	return m, waitForIdentify(run.gen, run.events)
}

// recordRun makes everything a run identified one step to undo.
func (m *Model) recordRun(r *identifyRun) {
	if len(r.after) > 0 {
		m.record(change{label: "identification of " + plural(len(r.after), "track"), before: r.before, after: r.after})
	}
	r.before, r.after = nil, nil
}

// adoptedMsg is the result of using tracks' old tags.
type adoptedMsg struct {
	before  []manifest.Track
	tracks  []manifest.Track // every track, as saved now
	adopted int
	err     error
}

// adoptOld is the o key: identify the track (or the whole Needs work group)
// by what its file said about itself.
func (m Model) adoptOld() (tea.Model, tea.Cmd) {
	r, ok := m.selectedRow()
	if !ok {
		return m, nil
	}
	var ids []string
	switch {
	case len(m.marks) > 0:
		for _, t := range m.markedTracks() {
			if t.Status != manifest.Identified {
				ids = append(ids, t.ID)
			}
		}
	case r.Kind == kindTrack && r.Track.Status != manifest.Identified:
		ids = []string{r.Track.ID}
	case r.Kind == kindGroup:
		ids = r.IDs
	}
	if len(ids) == 0 {
		m.say("Select a track that needs work, or the Needs work group", true)
		return m, nil
	}
	before, lib := m.tracksFor(ids), m.lib
	m.busy = true
	return m, func() tea.Msg {
		results, err := lib.Adopt(ids)
		if err != nil {
			return adoptedMsg{err: err}
		}
		adopted := 0
		for _, res := range results {
			if res.Adopted {
				adopted++
			}
		}
		tracks, err := lib.Tracks()
		return adoptedMsg{before: before, tracks: tracks, adopted: adopted, err: err}
	}
}

func (m Model) adopted(msg adoptedMsg) (tea.Model, tea.Cmd) {
	m.busy = false
	if msg.err != nil {
		m.say(msg.err.Error(), true)
		return m, nil
	}
	if msg.adopted == 0 {
		m.say("These files have no old tags to use", true)
		return m, nil
	}
	now := map[string]manifest.Track{}
	for _, t := range msg.tracks {
		now[t.ID] = t
	}
	var before, after []manifest.Track
	for _, b := range msg.before {
		if a := now[b.ID]; a.Status == manifest.Identified && b.Status != manifest.Identified {
			before, after = append(before, b), append(after, a)
			m.upsert(a)
		}
	}
	m.record(change{label: "use of old tags", before: before, after: after})
	text := "Used the old tags of " + plural(len(after), "track")
	if rest := len(msg.before) - len(after); rest > 0 {
		text += fmt.Sprintf(" · %d had none", rest)
	}
	m.say(text, false)
	if len(after) > 0 {
		m.revealTrack(after[0])
	} else {
		m.rebuild()
	}
	return m, nil
}
