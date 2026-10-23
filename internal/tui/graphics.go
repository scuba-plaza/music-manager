package tui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/sys/unix"

	"music-manager/internal/manifest"
)

// graphics is the way to the terminal for covers shown in full resolution
// (see kitty.go). It is nil in a terminal that cannot do it.
type graphics struct {
	mu   sync.Mutex
	out  io.Writer
	cell func() (w, h int) // pixels in a character cell; 0, 0 if unknown
}

// write sends a graphics command to the terminal. All of it is written in one
// piece, so that it cannot be split by the drawing of a screen.
func (g *graphics) write(s string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, err := io.WriteString(g.out, s)
	return err
}

func (g *graphics) cellSize() (w, h int) {
	if g.cell == nil {
		return 0, 0
	}
	return g.cell()
}

// WithGraphics lets the interface show covers with the kitty graphics
// protocol, writing the pictures to out, which must be the terminal that the
// interface itself draws on, through the same lock. cell reports the size
// of a character cell in pixels, if it is known.
func WithGraphics(out io.Writer, cell func() (w, h int)) Option {
	return func(m *Model) { m.gfx = &graphics{out: out, cell: cell} }
}

// picture is a cover that has been sent to the terminal, and the number of
// cells it was sent to cover.
type picture struct {
	id         uint32
	cols, rows int
}

// maxCoverRows is the tallest the cover is drawn, however much room there is.
const maxCoverRows = 24

// pictureMsg says that a cover has been sent to the terminal, or why not.
type pictureMsg struct {
	sha, key string
	pic      picture
	err      error
}

// coverPlan is how many cells the full-resolution cover of t takes in an
// inspector w wide and h high. It is worked out as if no field were being
// edited, so that typing, which changes the number of lines above the cover,
// does not make the cover be sent again at another size.
func (m Model) coverPlan(t manifest.Track, th thumb, w, h int) (cols, rows int) {
	quiet := m.quiet()
	room := quiet.coverRoom(t, w, h, len(quiet.trackHead(t, w)))
	cw, ch := m.gfx.cellSize()
	return placementSize(th.kw, th.kh, w-labelWidth-4, min(room, maxCoverRows), cw, ch)
}

// wantPicture sends the cover of the selected track to the terminal, if it
// is loaded and has not been sent at this size already.
func (m Model) wantPicture() tea.Cmd {
	r, ok := m.selectedRow()
	if m.gfx == nil || !ok || r.Kind != kindTrack {
		return nil
	}
	t := r.Track
	th, loaded := m.thumbs[t.CoverSHA256]
	l := computeLayout(m.w, m.h)
	if t.CoverSHA256 == "" || !loaded || th.kitty == nil || l.mode == modeTooSmall {
		return nil
	}
	cols, rows := m.coverPlan(t, th, l.right-2, l.body-2)
	if cols < minCoverCols || rows < minCoverRows {
		return nil
	}
	if p, ok := m.pictures[t.CoverSHA256]; ok && p.cols == cols && p.rows == rows {
		return nil
	}
	key := fmt.Sprintf("%s:%dx%d", t.CoverSHA256, cols, rows)
	if m.picturePending[key] {
		return nil
	}
	m.picturePending[key] = true

	sha, data, g := t.CoverSHA256, th.kitty, m.gfx
	id := imageID(sha, cols, rows)
	old, hadOld := m.pictures[sha]
	return func() tea.Msg {
		seq := transmitSeq(id, data, cols, rows)
		if hadOld {
			seq = deleteSeq(old.id) + seq
		}
		return pictureMsg{sha: sha, key: key, pic: picture{id, cols, rows}, err: g.write(seq)}
	}
}

// pictured handles a cover having been sent. If it could not be, the
// terminal is not taking pictures after all and the blocks are used from now on.
func (m Model) pictured(msg pictureMsg) (tea.Model, tea.Cmd) {
	delete(m.picturePending, msg.key)
	if msg.err != nil {
		m.gfx = nil
		return m, nil
	}
	m.pictures[msg.sha] = msg.pic
	return m, nil
}

// freePictures takes the pictures we sent out of the terminal's memory. Only
// those: the terminal may hold others, put there by other programs.
func (m Model) freePictures() {
	if m.gfx == nil || len(m.pictures) == 0 {
		return
	}
	var seq strings.Builder
	for _, p := range m.pictures {
		seq.WriteString(deleteSeq(p.id))
	}
	_ = m.gfx.write(seq.String())
}

// lockedTTY is the terminal as the interface draws on it. Everything written
// to it goes through one lock, so a picture sent in the middle of drawing a
// screen waits for its turn instead of landing inside it.
type lockedTTY struct {
	*os.File
	mu sync.Mutex
}

func (t *lockedTTY) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.File.Write(p)
}

// cellPixels is the size in pixels of one character cell of the terminal on
// f, or 0, 0 if the terminal does not say.
func cellPixels(f *os.File) (w, h int) {
	ws, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 || ws.Row == 0 || ws.Xpixel == 0 || ws.Ypixel == 0 {
		return 0, 0
	}
	return int(ws.Xpixel) / int(ws.Col), int(ws.Ypixel) / int(ws.Row)
}
