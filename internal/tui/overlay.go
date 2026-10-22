package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// An overlay is a dialog drawn over the screen. While one is open it gets
// every key; there is never more than one.
type overlay interface {
	// title is shown in the border.
	title() string
	// lines is the content for a box w columns wide.
	lines(w int) []string
	// width is how wide the dialog would like to be.
	width() int
	// key handles a key press. To close itself it sets m.overlay to nil.
	key(m *Model, msg tea.KeyMsg) tea.Cmd
	// hints are the keys to show in the footer.
	hints(k keyMap) []key.Binding
}

// animate says whether spinners keep turning. Tests switch it off, since
// their harness runs commands one after another and would sit through every
// frame of the animation.
var animate = true

// advance turns a spinner one frame and returns the command for the next, or
// nothing when animation is off.
func advance(sp *spinner.Model, msg spinner.TickMsg) tea.Cmd {
	if !animate {
		return nil
	}
	var cmd tea.Cmd
	*sp, cmd = sp.Update(msg)
	return cmd
}

// overlayView draws the model's overlay over the lines of base, centred.
func (m Model) overlayView(base string) string {
	if m.overlay == nil {
		return base
	}
	boxW := clamp(m.overlay.width(), 30, max(m.w-4, 30))
	content := m.overlay.lines(boxW - 2)
	boxH := min(len(content)+2, m.h-2)
	dialog := strings.Split(box(m.overlay.title(), content, boxW, boxH, true), "\n")

	rows := strings.Split(base, "\n")
	x, y := (m.w-boxW)/2, (len(rows)-boxH)/2
	for i, line := range dialog {
		if r := y + i; r >= 0 && r < len(rows) {
			rows[r] = splice(rows[r], line, x, m.w)
		}
	}
	return strings.Join(rows, "\n")
}

// splice puts over on top of base, starting at column x.
func splice(base, over string, x, width int) string {
	left := ansi.Truncate(base, x, "")
	left += strings.Repeat(" ", max(x-ansi.StringWidth(left), 0))
	right := ansi.TruncateLeft(base, x+ansi.StringWidth(over), "")
	return fit(left+over+right, width)
}
