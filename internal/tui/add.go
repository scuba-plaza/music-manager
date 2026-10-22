package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/app"
	"music-manager/internal/layout"
	"music-manager/internal/manifest"
	"music-manager/internal/translit"
)

// addValues are the texts the add form starts with.
type addValues struct{ title, artist, album, track, of string }

// addDefaults is what to offer for a downloaded track: what the MusicBrainz
// match says, if there is one and it is wanted, and otherwise what the
// download itself reported (taking the uploader of a "- Topic" channel for
// the artist).
func addDefaults(d *app.Draft, useMatch bool) addValues {
	info := d.Downloaded
	if useMatch && d.Match != nil {
		info = d.Info
	}
	artist := info.Artist
	if artist == "" {
		artist = strings.TrimSuffix(info.Uploader, " - Topic")
	}
	return addValues{info.Title, artist, info.Album, numberText(info.TrackNumber), numberText(info.TrackTotal)}
}

type addState int

const (
	addAsking    addState = iota // waiting for a link
	addFetching                  // downloading
	addForm                      // the details of the track
	addSaving                    // putting it in the library
	addDuplicate                 // it is there already
	addFailed                    // the download did not work
)

// The inputs of the form, in order.
const (
	addTitle = iota
	addArtist
	addAlbum
	addTrack
	addOf
	addCutStart
	addCutEnd
	addFields
)

// addDialog is the dialog that adds a track from a link.
type addDialog struct {
	state addState
	url   textinput.Model
	err   string

	in      [addFields]textinput.Model
	touched [addFields]bool // typed in by the user, so a switch of defaults leaves it
	focus   int

	draft    *app.Draft
	useMatch bool
	existing manifest.Track // for a duplicate

	spin    spinner.Model
	started time.Time
	gen     int
	cancel  context.CancelFunc
}

func newAddDialog() *addDialog {
	d := &addDialog{spin: spinner.New(spinner.WithSpinner(spinner.MiniDot)), useMatch: true}
	d.spin.Style = styles.accent
	d.url = textinput.New()
	d.url.Prompt = ""
	d.url.Placeholder = "https://www.youtube.com/watch?v=…"
	d.url.PlaceholderStyle = styles.dim
	d.url.Cursor.SetMode(cursor.CursorStatic)
	d.url.Focus()
	for i := range d.in {
		in := textinput.New()
		in.Prompt = ""
		in.Cursor.SetMode(cursor.CursorStatic)
		d.in[i] = in
	}
	return d
}

func (d *addDialog) title() string { return "Add from YouTube" }
func (d *addDialog) width() int    { return 78 }

// fill puts v into the inputs the user has not typed in.
func (d *addDialog) fill(v addValues) {
	for i, text := range []string{v.title, v.artist, v.album, v.track, v.of} {
		if !d.touched[i] {
			d.in[i].SetValue(text)
		}
	}
}

func (d *addDialog) setFocus(i int) tea.Cmd {
	d.in[d.focus].Blur()
	d.focus = (i + addFields) % addFields
	return d.in[d.focus].Focus()
}

func (d *addDialog) hints(k keyMap) []key.Binding {
	switch d.state {
	case addAsking:
		return []key.Binding{key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "fetch")), k.Close2}
	case addForm:
		bindings := []key.Binding{key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "add")), k.Close2, k.NextField}
		if d.draft != nil && d.draft.Match != nil {
			text := "ignore the match"
			if !d.useMatch {
				text = "use the match"
			}
			bindings = append(bindings, key.NewBinding(key.WithKeys("ctrl+t"), key.WithHelp("ctrl+t", text)))
		}
		return bindings
	case addDuplicate:
		return []key.Binding{key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "show it")), k.Close2}
	case addFailed:
		return []key.Binding{key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "tries again")), k.Close2}
	}
	return []key.Binding{k.Close2}
}

func (d *addDialog) lines(w int) []string {
	lines := []string{""}
	switch d.state {
	case addAsking:
		in := d.url
		in.Width = max(w-4, 10)
		lines = append(lines, " "+styles.dim.Render("Paste a link to a video or a track:"), "", " "+in.View())
		if d.err != "" {
			lines = append(lines, "", " "+styles.bad.Render(d.err))
		}
	case addFetching:
		secs := int(time.Since(d.started).Seconds())
		lines = append(lines, " "+d.spin.View()+fmt.Sprintf(" Downloading… %ds", secs), "", " "+styles.dim.Render("This also looks the track up on MusicBrainz. esc cancels."))
	case addSaving:
		lines = append(lines, " "+d.spin.View()+" Adding to the library…")
	case addDuplicate:
		_, title, _ := app.Display(d.existing)
		artist, _, _ := app.Display(d.existing)
		lines = append(lines, " "+styles.warn.Render("This audio is already in your library:"), "", "   "+styles.bold.Render(title)+styles.dim.Render(" · "+artist))
	case addFailed:
		for _, l := range wrap(d.err, w-2) {
			lines = append(lines, " "+styles.bad.Render(l))
		}
	case addForm:
		lines = append(lines, d.formLines(w)...)
	}
	return append(lines, "")
}

func (d *addDialog) formLines(w int) []string {
	var lines []string
	switch {
	case d.draft.Match == nil:
		lines = append(lines, " "+styles.dim.Render("No match on MusicBrainz: these are YouTube's words."))
	case d.useMatch:
		m := d.draft.Match
		lines = append(lines, " "+styles.accent.Render(fit("Matched on MusicBrainz: "+m.Artist+" - "+m.Title+" · "+m.Album, w-2)))
	default:
		lines = append(lines, " "+styles.dim.Render("Ignoring the match: these are YouTube's words."))
	}
	lines = append(lines, "")

	label := func(i int, name string) string {
		if i == d.focus {
			return styles.accent.Render(fit(name, 11))
		}
		return styles.dim.Render(fit(name, 11))
	}
	vw := max(w-14, 10)
	row := func(i int, name string) {
		in := d.in[i]
		in.Width = vw - 1
		lines = append(lines, " "+label(i, name)+" "+in.View())
		if i <= addAlbum {
			text := strings.TrimSpace(d.in[i].Value())
			if ascii := d.ascii(i, text); text != "" && ascii != text {
				lines = append(lines, "  "+strings.Repeat(" ", 11)+styles.dim.Render(fit("ascii  "+ascii, vw)))
			}
		}
	}
	row(addTitle, "Title")
	row(addArtist, "Artist")
	row(addAlbum, "Album")
	track, of := d.in[addTrack], d.in[addOf]
	track.Width, of.Width = 5, 5
	trackLabel := label(addTrack, "Track")
	if d.focus == addOf {
		trackLabel = label(addTrack, "Track")
	}
	lines = append(lines, " "+trackLabel+" "+track.View()+styles.dim.Render(" of ")+of.View())
	row(addCutStart, "Cut start")
	row(addCutEnd, "Cut end")

	lines = append(lines, "", " → "+styles.accent.Render(fit(d.path(), w-5)))
	if d.err != "" {
		for _, l := range wrap(d.err, w-2) {
			lines = append(lines, " "+styles.bad.Render(l))
		}
	}
	return lines
}

// ascii is the ASCII form a name field would be saved with.
func (d *addDialog) ascii(field int, text string) string {
	if field == addArtist && d.useMatch && d.draft.Match != nil && text == d.draft.Info.Artist && d.draft.Info.ArtistASCII != "" {
		return d.draft.Info.ArtistASCII
	}
	return translit.ToASCII(text)
}

// path is where the track would be written.
func (d *addDialog) path() string {
	title, artist := strings.TrimSpace(d.in[addTitle].Value()), strings.TrimSpace(d.in[addArtist].Value())
	if title == "" || artist == "" {
		return "fill in a title and an artist"
	}
	album := strings.TrimSpace(d.in[addAlbum].Value())
	n, _ := count(d.in[addTrack].Value())
	of, _ := count(d.in[addOf].Value())
	return layout.PathFor(manifest.Track{
		ID: "new", Status: manifest.Identified,
		Title: d.ascii(addTitle, title), Artist: d.ascii(addArtist, artist), Album: d.ascii(addAlbum, album),
		TrackNumber: n, TrackTotal: of,
	})
}

// edit turns the form into the details of the track.
func (d *addDialog) edit() (app.Edit, error) {
	var e app.Edit
	parts := []draft{
		{field: fTitle, text: d.in[addTitle].Value()},
		{field: fArtist, text: d.in[addArtist].Value()},
		{field: fAlbum, text: d.in[addAlbum].Value()},
		{field: fTrack, text: d.in[addTrack].Value(), second: d.in[addOf].Value()},
		{field: fCutStart, text: d.in[addCutStart].Value()},
		{field: fCutEnd, text: d.in[addCutEnd].Value()},
	}
	for _, p := range parts {
		part, err := p.edit()
		if err != nil {
			return e, err
		}
		e = mergeEdits(e, part)
	}
	if text := strings.TrimSpace(d.in[addArtist].Value()); d.useMatch && d.draft.Match != nil && text == d.draft.Info.Artist && d.draft.Info.ArtistASCII != "" {
		ascii := d.draft.Info.ArtistASCII // the form the database knows the artist by
		e.ArtistASCII = &ascii
	}
	return e, nil
}

// mergeEdits is a with every field b sets.
func mergeEdits(a, b app.Edit) app.Edit {
	if b.Title != nil {
		a.Title = b.Title
	}
	if b.Artist != nil {
		a.Artist = b.Artist
	}
	if b.Album != nil {
		a.Album = b.Album
	}
	if b.TitleASCII != nil {
		a.TitleASCII = b.TitleASCII
	}
	if b.ArtistASCII != nil {
		a.ArtistASCII = b.ArtistASCII
	}
	if b.AlbumASCII != nil {
		a.AlbumASCII = b.AlbumASCII
	}
	if b.TrackNumber != nil {
		a.TrackNumber = b.TrackNumber
	}
	if b.TrackTotal != nil {
		a.TrackTotal = b.TrackTotal
	}
	if b.TrimStart != nil {
		a.TrimStart = b.TrimStart
	}
	if b.TrimEnd != nil {
		a.TrimEnd = b.TrimEnd
	}
	return a
}

// fetchedMsg is the result of downloading a link.
type fetchedMsg struct {
	gen   int
	draft *app.Draft
	err   error
}

// addedMsg is the result of adding the track.
type addedMsg struct {
	gen   int
	track manifest.Track
	err   error
}

// downloadTimeout bounds a download, in case it never ends.
const downloadTimeout = 10 * time.Minute

func (d *addDialog) fetch(lib Library) tea.Cmd {
	if d.cancel != nil {
		d.cancel()
	}
	d.gen++
	d.state, d.err, d.started = addFetching, "", time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	d.cancel = cancel
	gen, url := d.gen, cleanLink(d.url.Value())
	return tea.Batch(d.spin.Tick, func() tea.Msg {
		draft, err := lib.Fetch(ctx, url)
		return fetchedMsg{gen, draft, err}
	})
}

// cleanLink strips the spaces and line breaks that come with a paste.
func cleanLink(s string) string { return strings.Join(strings.Fields(s), "") }

func (d *addDialog) fetched(m *Model, msg fetchedMsg) {
	var dup *app.DuplicateError
	switch {
	case errors.As(msg.err, &dup):
		d.state = addDuplicate
		for _, t := range m.tracks {
			if t.ID == dup.ID {
				d.existing = t
			}
		}
	case msg.err != nil:
		d.state, d.err = addFailed, msg.err.Error()
	default:
		d.state, d.draft, d.useMatch = addForm, msg.draft, msg.draft.Match != nil
		d.touched = [addFields]bool{}
		d.fill(addDefaults(msg.draft, d.useMatch))
		d.focus = addTitle
		d.in[addTitle].Focus()
	}
}

func (d *addDialog) tick(msg spinner.TickMsg) tea.Cmd {
	if d.state != addFetching && d.state != addSaving {
		return nil
	}
	var cmd tea.Cmd
	d.spin, cmd = d.spin.Update(msg)
	return cmd
}

func (d *addDialog) close(m *Model) {
	if d.cancel != nil {
		d.cancel()
	}
	m.overlay = nil
}

func (d *addDialog) key(m *Model, msg tea.KeyMsg) tea.Cmd {
	k := m.keys
	switch d.state {
	case addAsking:
		switch {
		case key.Matches(msg, k.Close2):
			d.close(m)
		case key.Matches(msg, k.Apply):
			if cleanLink(d.url.Value()) == "" {
				d.err = "Paste a link first"
				return nil
			}
			return d.fetch(m.lib)
		default:
			d.err = ""
			var cmd tea.Cmd
			d.url, cmd = d.url.Update(msg)
			return cmd
		}
	case addFetching, addSaving:
		if key.Matches(msg, k.Close2) {
			d.close(m)
		}
	case addDuplicate:
		switch {
		case key.Matches(msg, k.Close2):
			d.close(m)
		case key.Matches(msg, k.Apply):
			d.close(m)
			m.revealTrack(d.existing)
		}
	case addFailed:
		switch {
		case key.Matches(msg, k.Close2):
			d.close(m)
		case key.Matches(msg, k.Apply):
			return d.fetch(m.lib)
		}
	case addForm:
		return d.formKey(m, msg)
	}
	return nil
}

func (d *addDialog) formKey(m *Model, msg tea.KeyMsg) tea.Cmd {
	k := m.keys
	switch {
	case key.Matches(msg, k.Close2):
		d.close(m)
		return nil
	case key.Matches(msg, k.Apply):
		return d.save(m)
	case msg.String() == "ctrl+t" && d.draft.Match != nil:
		d.useMatch = !d.useMatch
		d.fill(addDefaults(d.draft, d.useMatch))
		return nil
	case key.Matches(msg, k.NextField):
		return d.setFocus(d.focus + 1)
	case key.Matches(msg, k.PrevField):
		return d.setFocus(d.focus - 1)
	}
	d.err = ""
	d.touched[d.focus] = true
	var cmd tea.Cmd
	d.in[d.focus], cmd = d.in[d.focus].Update(msg)
	return cmd
}

func (d *addDialog) save(m *Model) tea.Cmd {
	e, err := d.edit()
	if err != nil {
		d.err = err.Error()
		return nil
	}
	d.state, d.err = addSaving, ""
	d.gen++
	gen, lib, draft := d.gen, m.lib, d.draft
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	d.cancel = cancel
	return tea.Batch(d.spin.Tick, func() tea.Msg {
		defer cancel()
		t, err := lib.Finish(ctx, draft, e)
		return addedMsg{gen, t, err}
	})
}

func (m Model) added(msg addedMsg) (tea.Model, tea.Cmd) {
	d, open := m.overlay.(*addDialog)
	if open && d.gen != msg.gen {
		return m, nil // an answer to something the dialog has moved on from
	}
	if msg.err != nil {
		if open {
			d.state, d.err = addForm, msg.err.Error()
		} else {
			m.say(msg.err.Error(), true)
		}
		return m, nil
	}
	m.overlay = nil
	m.upsert(msg.track)
	m.record(change{label: "addition of a track", after: []manifest.Track{msg.track}})
	m.say(fmt.Sprintf("Added %q", msg.track.Title), false)
	m.revealTrack(msg.track)
	return m, nil
}

// openAdd is the a key.
func (m Model) openAdd() (tea.Model, tea.Cmd) {
	if !m.lib.CanAdd() {
		m.say("Adding from YouTube is not available: yt-dlp is not set up", true)
		return m, nil
	}
	m.overlay = newAddDialog()
	return m, textinput.Blink
}
