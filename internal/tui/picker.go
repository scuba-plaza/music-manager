package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

// diffLine is one row of the preview of what a candidate would change.
type diffLine struct {
	label, old, new string
	same            bool
}

// candidateDiff compares a track as it is with a candidate it could become.
func candidateDiff(t manifest.Track, c app.Candidate) []diffLine {
	orDash := func(s string) string {
		if s == "" {
			return "—"
		}
		return s
	}
	line := func(label, old, new string) diffLine {
		return diffLine{label, orDash(old), orDash(new), orDash(old) == orDash(new)}
	}
	title := name(t, fTitle)
	artist := name(t, fArtist)
	album := name(t, fAlbum)
	newTrack := ""
	switch {
	case c.TrackNumber > 0 && c.TrackTotal > 0:
		newTrack = fmt.Sprintf("%d of %d", c.TrackNumber, c.TrackTotal)
	case c.TrackNumber > 0:
		newTrack = fmt.Sprint(c.TrackNumber)
	}
	return []diffLine{
		line("Title", title, c.Title),
		line("Artist", artist, c.Artist),
		line("Album", album, c.Album),
		line("Track", displayValue(t, fTrack), newTrack),
	}
}

// diffText draws a diffLine: what it is now and what it would become, or
// just what it is if it would stay.
func diffText(d diffLine) string {
	label := styles.dim.Render(fit(d.label, 8))
	if d.same {
		return label + styles.dim.Render(d.new+"  (same)")
	}
	return label + d.old + styles.dim.Render(" → ") + styles.accent.Render(d.new)
}

type pickState int

const (
	pickSearching pickState = iota
	pickListing
	pickFailed
	pickEditing
	pickApplying
)

// picker is the dialog for choosing what a track is on MusicBrainz.
type picker struct {
	track  manifest.Track // the track as it is now, for the preview
	state  pickState
	cands  []app.Candidate
	cursor int
	err    string

	query [2]textinput.Model // the words to search for: title, artist
	focus int
	spin  spinner.Model

	gen    int // which search the answer must belong to
	cancel context.CancelFunc
}

func newPicker(t manifest.Track) *picker {
	p := &picker{track: t, spin: spinner.New(spinner.WithSpinner(spinner.MiniDot))}
	p.spin.Style = styles.accent
	title := name(t, fTitle)
	artist := name(t, fArtist)
	for i, v := range []string{title, artist} {
		in := textinput.New()
		in.Prompt = ""
		in.SetValue(v)
		in.Cursor.SetMode(cursor.CursorStatic)
		p.query[i] = in
	}
	return p
}

func (p *picker) title() string { return "Identify" }
func (p *picker) width() int    { return 84 }

func (p *picker) words() string {
	return fmt.Sprintf("“%s – %s”", strings.TrimSpace(p.query[0].Value()), strings.TrimSpace(p.query[1].Value()))
}

// candidatesMsg is the answer to a search.
type candidatesMsg struct {
	gen   int
	cands []app.Candidate
	err   error
}

// searchTimeout bounds one search, in case the service never answers.
const searchTimeout = 40 * time.Second

// search looks for the words in the form, or for the track itself the first
// time, and answers with a candidatesMsg.
func (p *picker) search(lib Library, custom bool) tea.Cmd {
	if p.cancel != nil {
		p.cancel()
	}
	p.gen++
	p.state, p.err = pickSearching, ""
	ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
	p.cancel = cancel
	var q *app.Query
	if custom {
		q = &app.Query{Title: strings.TrimSpace(p.query[0].Value()), Artist: strings.TrimSpace(p.query[1].Value())}
	}
	gen, id := p.gen, p.track.ID
	return tea.Batch(p.spin.Tick, func() tea.Msg {
		cands, err := lib.Candidates(ctx, id, q)
		return candidatesMsg{gen, cands, err}
	})
}

func (p *picker) received(msg candidatesMsg) {
	p.cursor = 0
	switch {
	case msg.err != nil:
		p.state, p.err = pickFailed, msg.err.Error()
	case len(msg.cands) == 0:
		p.cands = nil
		p.state, p.err = pickFailed, "No match found for "+p.words()+"."
	default:
		p.cands, p.state = msg.cands, pickListing
	}
}

// tick keeps the spinner turning while something is under way.
func (p *picker) tick(msg spinner.TickMsg) tea.Cmd {
	if p.state != pickSearching && p.state != pickApplying {
		return nil
	}
	return advance(&p.spin, msg)
}

func (p *picker) close(m *Model) {
	if p.cancel != nil {
		p.cancel()
	}
	m.overlay = nil
}

func (p *picker) hints(k keyMap) []key.Binding {
	switch p.state {
	case pickListing:
		return []key.Binding{k.Field, k.Apply, key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "other words")), k.Close2}
	case pickFailed:
		return []key.Binding{key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "other words")), k.Close2}
	case pickEditing:
		return []key.Binding{key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "search")), k.Close2, k.NextInput}
	}
	if p.state == pickApplying {
		return []key.Binding{key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "skip the cover"))}
	}
	return []key.Binding{k.Close2}
}

// candidateText is one row of the list. What you choose by is the release,
// so the album leads; the title is only there if this recording is not the
// one the first row has (a live or shortened version).
func candidateText(c app.Candidate, firstTitle string) string {
	album := c.Album
	if album == "" {
		album = "(no album)"
	}
	s := fmt.Sprintf("%3d%%  %s", c.Score, album)
	if c.Date != "" {
		s += " · " + c.Date
	}
	if c.TrackNumber > 0 {
		s += fmt.Sprintf(" · %d/%d", c.TrackNumber, c.TrackTotal)
	}
	s += " · " + c.Artist
	if c.Title != firstTitle {
		s += " · " + c.Title
	}
	return s
}

func (p *picker) lines(w int) []string {
	lines := []string{""}
	switch p.state {
	case pickSearching:
		lines = append(lines, " "+p.spin.View()+" Searching MusicBrainz for "+p.words()+"…", "", " "+styles.dim.Render("esc cancels"))
	case pickApplying:
		lines = append(lines, " "+p.spin.View()+" Applying…", "", " "+styles.dim.Render("Fetching the cover can take a moment; esc skips it."))
	case pickFailed:
		for _, l := range wrap(p.err, w-2) {
			lines = append(lines, " "+styles.warn.Render(l))
		}
		lines = append(lines, "", " "+styles.dim.Render("Press / to search with other words."))
	case pickEditing:
		lines = append(lines, " "+styles.dim.Render("Search for:"), "")
		for i, label := range []string{"Title", "Artist"} {
			in := p.query[i]
			in.Width = max(w-14, 5)
			name := styles.dim.Render(fit(label, 8))
			if i == p.focus {
				name = styles.accent.Render(fit(label, 8))
			}
			lines = append(lines, " "+name+" "+in.View())
		}
	case pickListing:
		lines = append(lines, " "+styles.dim.Render(fmt.Sprintf("%s for %s", plural(len(p.cands), "match"), p.words())))
		visible := min(len(p.cands), 9)
		first := clamp(p.cursor-visible+1, 0, len(p.cands)-visible)
		for i := first; i < first+visible; i++ {
			text := fit(candidateText(p.cands[i], p.cands[0].Title), w-4)
			if i == p.cursor {
				lines = append(lines, " "+styles.selected.Render(" "+text+" "))
			} else {
				lines = append(lines, "  "+text)
			}
		}
		lines = append(lines, "", " "+styles.dim.Render("This would change:"))
		for _, d := range candidateDiff(p.track, p.cands[p.cursor]) {
			lines = append(lines, "  "+fit(diffText(d), w-3))
		}
	}
	return append(lines, "")
}

func (p *picker) key(m *Model, msg tea.KeyMsg) tea.Cmd {
	k := m.keys
	switch p.state {
	case pickSearching:
		if key.Matches(msg, k.Close2) {
			p.close(m)
		}
	case pickApplying:
		if key.Matches(msg, k.Close2) && p.cancel != nil {
			p.cancel() // gives up on the cover; the track is still identified
		}
	case pickListing:
		switch {
		case key.Matches(msg, k.Close2):
			p.close(m)
		case key.Matches(msg, k.Up):
			p.cursor = max(p.cursor-1, 0)
		case key.Matches(msg, k.Down):
			p.cursor = min(p.cursor+1, len(p.cands)-1)
		case key.Matches(msg, k.PageUp), key.Matches(msg, k.Top):
			p.cursor = 0
		case key.Matches(msg, k.PageDown), key.Matches(msg, k.Bottom):
			p.cursor = len(p.cands) - 1
		case key.Matches(msg, k.Apply):
			return p.apply(m)
		case msg.String() == "/":
			p.edit()
		}
	case pickFailed:
		switch {
		case key.Matches(msg, k.Close2):
			p.close(m)
		case msg.String() == "/":
			p.edit()
		}
	case pickEditing:
		switch {
		case key.Matches(msg, k.Close2):
			p.state = pickFailed
			if len(p.cands) > 0 {
				p.state = pickListing
			}
		case key.Matches(msg, k.Apply):
			return p.search(m.lib, true)
		case key.Matches(msg, k.NextInput):
			p.query[p.focus].Blur()
			p.focus = 1 - p.focus
			return p.query[p.focus].Focus()
		default:
			var cmd tea.Cmd
			p.query[p.focus], cmd = p.query[p.focus].Update(msg)
			return cmd
		}
	}
	return nil
}

func (p *picker) edit() {
	p.state, p.focus = pickEditing, 0
	p.query[0].Focus()
	p.query[1].Blur()
	p.query[0].CursorEnd()
}

// identifiedMsg is the result of applying a candidate.
type identifiedMsg struct {
	before, after manifest.Track
	err           error
}

func (p *picker) apply(m *Model) tea.Cmd {
	c, lib, before := p.cands[p.cursor], m.lib, p.track
	p.state = pickApplying
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	p.cancel = cancel
	return tea.Batch(p.spin.Tick, func() tea.Msg {
		defer cancel()
		after, err := lib.ApplyCandidate(ctx, before.ID, c)
		return identifiedMsg{before, after, err}
	})
}

func (m Model) identified(msg identifiedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		if p, ok := m.overlay.(*picker); ok {
			p.state, p.err = pickFailed, msg.err.Error()
		} else {
			m.say(msg.err.Error(), true)
		}
		return m, nil
	}
	m.overlay = nil
	m.upsert(msg.after)
	m.record(change{label: "identification of a track", before: []manifest.Track{msg.before}, after: []manifest.Track{msg.after}})
	m.say("Identified as "+msg.after.Artist+" - "+msg.after.Title, false)
	m.revealTrack(msg.after)
	return m, nil
}

// openPicker is the i key: look the selected track up.
func (m Model) openPicker() (tea.Model, tea.Cmd) {
	r, ok := m.selectedRow()
	if !ok || r.Kind != kindTrack {
		return m, nil
	}
	if !m.lib.Lookup() {
		m.say("Lookup is not available: no MusicBrainz client is set up", true)
		return m, nil
	}
	p := newPicker(r.Track)
	m.overlay = p
	return m, p.search(m.lib, false)
}
