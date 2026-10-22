package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/app"
)

// doctorDialog shows what a check of the library found.
type doctorDialog struct {
	checking bool
	problems []app.Problem
	err      string
	cursor   int
	spin     spinner.Model
	titles   map[string]string // track id -> what to call it
}

// doctorMsg is the result of a check.
type doctorMsg struct {
	problems []app.Problem
	err      error
}

func (d *doctorDialog) title() string { return "Library check" }
func (d *doctorDialog) width() int    { return 90 }

func (d *doctorDialog) hints(k keyMap) []key.Binding {
	if d.checking || len(d.problems) == 0 {
		return []key.Binding{k.Close2}
	}
	return []key.Binding{k.Field, key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "go to the track")), k.Close2}
}

func (d *doctorDialog) tick(msg spinner.TickMsg) tea.Cmd {
	if !d.checking {
		return nil
	}
	return advance(&d.spin, msg)
}

func (d *doctorDialog) lines(w int) []string {
	lines := []string{""}
	switch {
	case d.checking:
		lines = append(lines, " "+d.spin.View()+" Checking that every stored file is there and unharmed…", "", " "+styles.dim.Render("esc cancels"))
	case d.err != "":
		for _, l := range wrap(d.err, w-2) {
			lines = append(lines, " "+styles.bad.Render(l))
		}
	case len(d.problems) == 0:
		lines = append(lines, " "+styles.good.Render("No problems found."))
	default:
		errors := 0
		for _, p := range d.problems {
			if p.Severity == app.Error {
				errors++
			}
		}
		lines = append(lines, " "+styles.bold.Render(fmt.Sprintf("%s, %s", plural(errors, "error"), plural(len(d.problems)-errors, "warning"))), "")
		first := clamp(d.cursor-7, 0, max(len(d.problems)-8, 0))
		for i := first; i < min(first+8, len(d.problems)); i++ {
			lines = append(lines, d.row(i, w))
		}
	}
	return append(lines, "")
}

func (d *doctorDialog) row(i, w int) string {
	p := d.problems[i]
	glyph := styles.warn.Render("!")
	if p.Severity == app.Error {
		glyph = styles.bad.Render("✗")
	}
	name := d.titles[p.TrackID]
	if name == "" {
		name = p.TrackID
	}
	text := glyph + " " + styles.bold.Render(name) + styles.dim.Render(" · "+p.Message)
	if i == d.cursor {
		text = styles.selected.Render(fit(" "+ansiPlain(glyph)+" "+name+" · "+p.Message, w-3))
		return " " + text
	}
	return "  " + fit(text, w-3)
}

func (d *doctorDialog) key(m *Model, msg tea.KeyMsg) tea.Cmd {
	k := m.keys
	switch {
	case key.Matches(msg, k.Close2):
		m.overlay = nil
	case key.Matches(msg, k.Up):
		d.cursor = max(d.cursor-1, 0)
	case key.Matches(msg, k.Down):
		d.cursor = min(d.cursor+1, max(len(d.problems)-1, 0))
	case key.Matches(msg, k.Apply):
		if d.checking || len(d.problems) == 0 {
			return nil
		}
		id := d.problems[d.cursor].TrackID
		m.overlay = nil
		for _, t := range m.tracks {
			if t.ID == id {
				m.revealTrack(t)
			}
		}
	}
	return nil
}

// openDoctor is the D key.
func (m Model) openDoctor() (tea.Model, tea.Cmd) {
	d := &doctorDialog{checking: true, spin: spinner.New(spinner.WithSpinner(spinner.MiniDot)), titles: map[string]string{}}
	d.spin.Style = styles.accent
	for _, t := range m.tracks {
		_, title, _ := app.Display(t)
		d.titles[t.ID] = title
	}
	m.overlay = d
	lib := m.lib
	return m, tea.Batch(d.spin.Tick, func() tea.Msg {
		problems, err := lib.Doctor()
		return doctorMsg{problems, err}
	})
}

func (m Model) doctored(msg doctorMsg) (tea.Model, tea.Cmd) {
	if d, ok := m.overlay.(*doctorDialog); ok && d.checking {
		d.checking = false
		d.problems = msg.problems
		if msg.err != nil {
			d.err = msg.err.Error()
		}
	}
	return m, nil
}
