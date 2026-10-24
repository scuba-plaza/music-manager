package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

// copyList is the dialog that c opens on a track: what the track has, to tick
// what is to be copied. Everything it has is ticked to start with.
type copyList struct {
	t      manifest.Track
	attrs  []clipAttr // what the track has, in the order of the list
	fields []fieldKind
	has    []bool
	ticked []bool
	cursor int
	err    string
}

// copyLabelWidth is the width of the names of the attributes in the list.
const copyLabelWidth = 16

// copyOrder is the order of the attributes in the list.
var copyOrder = []fieldKind{fCover, fArtist, fAlbum, fAlbumOf, fReleased, fGenre}

func newCopyList(t manifest.Track) *copyList {
	c := &copyList{t: t, fields: copyOrder}
	first := -1
	for i, f := range copyOrder {
		a, ok := attrOf(t, f)
		c.attrs, c.has, c.ticked = append(c.attrs, a), append(c.has, ok), append(c.ticked, ok)
		if ok && first < 0 {
			first = i
		}
	}
	c.cursor = max(first, 0)
	return c
}

// copyLabel is how an attribute is named in the list.
func copyLabel(f fieldKind) string {
	if f == fAlbumOf {
		return "Tracks on album"
	}
	return f.label()
}

func (c *copyList) title() string {
	_, title, _ := app.Display(c.t)
	return "Copy from " + title
}

func (c *copyList) width() int { return 56 }

// shown is what a row says about the attribute.
func (c *copyList) shown(i int) string {
	a := c.attrs[i]
	switch {
	case a.field == fCover:
		return "yes"
	case a.latin != "":
		return a.text + " (" + a.latin + ")"
	}
	return a.text
}

func (c *copyList) lines(w int) []string {
	lines := []string{""}
	for i, f := range c.fields {
		box, value, label := "[ ] ", c.shown(i), copyLabel(f)
		var line string
		switch {
		case !c.has[i]:
			line = "    " + styles.dim.Render(fit(label, copyLabelWidth)+" —")
		default:
			if c.ticked[i] {
				box = "[x] "
			}
			line = box + fit(label, copyLabelWidth) + " " + fit(value, max(w-copyLabelWidth-8, 4))
		}
		if i == c.cursor {
			line = styles.accent.Render("▌") + line
		} else {
			line = " " + line
		}
		lines = append(lines, line)
	}
	if c.err != "" {
		lines = append(lines, "", " "+styles.bad.Render(c.err))
	}
	return append(lines, "")
}

func (c *copyList) hints(k keyMap) []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "ticks")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "copies")),
		k.Close2,
	}
}

func (c *copyList) key(m *Model, msg tea.KeyMsg) tea.Cmd {
	k := m.keys
	switch {
	case key.Matches(msg, k.Close2), msg.String() == "q":
		m.overlay = nil
	case key.Matches(msg, k.Up):
		c.cursor = max(c.cursor-1, 0)
	case key.Matches(msg, k.Down):
		c.cursor = min(c.cursor+1, len(c.fields)-1)
	case key.Matches(msg, k.Mark):
		if c.has[c.cursor] {
			c.ticked[c.cursor] = !c.ticked[c.cursor]
			c.err = ""
		}
	case key.Matches(msg, k.Apply):
		var attrs []clipAttr
		for i, a := range c.attrs {
			if c.ticked[i] {
				attrs = append(attrs, a)
			}
		}
		if len(attrs) == 0 {
			c.err = "Tick what to copy"
			return nil
		}
		m.overlay = nil
		*m = m.copied(c.t, attrs)
	}
	return nil
}

// openCopy is c in the tree: a checklist of what the track under the cursor
// has.
func (m Model) openCopy() (tea.Model, tea.Cmd) {
	r, ok := m.selectedRow()
	if !ok || r.Kind != kindTrack {
		m.say("Select a track to copy from", true)
		return m, nil
	}
	m.overlay = newCopyList(r.Track)
	return m, nil
}
