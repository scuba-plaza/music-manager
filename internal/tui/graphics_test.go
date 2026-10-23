package tui

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image/png"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// terminalLog records what the interface writes to the terminal outside its
// drawing, which is where pictures go.
type terminalLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
	err error // if set, every write fails
}

func (l *terminalLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err != nil {
		return 0, l.err
	}
	return l.buf.Write(p)
}

func (l *terminalLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// tenByTwenty is the size of a character cell in pixels.
func tenByTwenty() (int, int) { return 10, 20 }

// pictureModel is the library with a cover on "Flow", shown in a terminal
// that has the graphics protocol, with that track selected.
func pictureModel(t *testing.T, w, h int) (Model, *terminalLog, *editHook) {
	t.Helper()
	_, hook := editingModel(t, "Flow")
	tr := trackNamed(t, hook.Library, "Flow")
	if err := appOf(t, hook.Library).SetCover(tr.ID, encodePNG(t, twoTone(60, 60))); err != nil {
		t.Fatal(err)
	}
	log := &terminalLog{}
	m := New(hook, WithGraphics(log, tenByTwenty))
	m = send(t, m, m.Init(), tea.WindowSizeMsg{Width: w, Height: h})
	return selectTrack(t, m, "Flow"), log, hook
}

// transmissions are the pictures sent so far, as the keys of their commands.
func transmissions(t *testing.T, log *terminalLog) []map[string]string {
	t.Helper()
	cmds, _ := parseAPC(t, log.String())
	var out []map[string]string
	for _, c := range cmds {
		if c["a"] == "T" {
			out = append(out, c)
		}
	}
	return out
}

func placeholderCount(m Model) int { return strings.Count(screen(m), "\U0010EEEE") }

func TestAFullResolutionCoverIsSentOnceAndDrawnWithPlaceholders(t *testing.T) {
	m, log, _ := pictureModel(t, 120, 36)

	sent := transmissions(t, log)
	if len(sent) != 1 {
		t.Fatalf("%d pictures sent, want 1; terminal got %q", len(sent), log.String())
	}
	cmd := sent[0]
	if cmd["U"] != "1" || cmd["f"] != "100" || cmd["q"] != "2" {
		t.Errorf("not a quiet PNG for placeholders: %v", cmd)
	}
	_, payloads := parseAPC(t, log.String())
	raw, err := base64.StdEncoding.DecodeString(payloads[0])
	if err != nil {
		t.Fatal(err)
	}
	if img, err := png.Decode(bytes.NewReader(raw)); err != nil || img.Bounds().Dx() != 60 {
		t.Errorf("the picture sent is not the cover (%v)", err)
	}

	cols, rows := atoi(cmd["c"]), atoi(cmd["r"])
	if cols < 4 || rows < 4 || cols != rows*2 {
		t.Errorf("a square cover in cells twice as tall as wide should be drawn %d×%d cells", cols, rows)
	}
	if got := placeholderCount(m); got != cols*rows {
		t.Errorf("%d placeholder cells on screen, want %d×%d", got, cols, rows)
	}
	lacks(t, m, "▀")
	contains(t, m, "60×60 JPEG")
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}

func TestSelectingAnotherTrackAndComingBackDoesNotSendThePictureAgain(t *testing.T) {
	m, log, _ := pictureModel(t, 120, 36)
	m = selectTrack(t, m, "Along Our Way") // no cover
	if got := placeholderCount(m); got != 0 {
		t.Errorf("a track without a cover shows %d placeholders", got)
	}
	m = selectTrack(t, m, "Flow")
	if got := len(transmissions(t, log)); got != 1 {
		t.Errorf("%d pictures sent after coming back, want still 1", got)
	}
	if placeholderCount(m) == 0 {
		t.Error("the cover is not drawn after coming back")
	}
}

func TestAnotherSizeGetsItsOwnPictureAndTheOldOneIsFreed(t *testing.T) {
	m, log, _ := pictureModel(t, 120, 36)
	first := transmissions(t, log)[0]

	m = send(t, m, tea.WindowSizeMsg{Width: 100, Height: 50})
	sent := transmissions(t, log)
	if len(sent) != 2 {
		t.Fatalf("%d pictures sent after the window changed, want 2", len(sent))
	}
	if sent[1]["i"] == first["i"] {
		t.Error("the new size must be a new picture, not the old one resized")
	}
	cmds, _ := parseAPC(t, log.String())
	freed := false
	for _, c := range cmds {
		freed = freed || (c["a"] == "d" && c["d"] == "I" && c["i"] == first["i"])
	}
	if !freed {
		t.Errorf("the picture of the old size was never freed: %v", cmds)
	}
	if got, want := placeholderCount(m), atoi(sent[1]["c"])*atoi(sent[1]["r"]); got != want {
		t.Errorf("%d placeholders, want %d", got, want)
	}

	m = send(t, m, tea.WindowSizeMsg{Width: 100, Height: 50})
	if got := len(transmissions(t, log)); got != 2 {
		t.Errorf("an unchanged size sent the picture again: %d", got)
	}
}

func TestEditingAFieldDoesNotResendThePicture(t *testing.T) {
	m, log, _ := pictureModel(t, 120, 36)
	m = keys(t, m, "right", "enter") // edit the title
	m = typed(t, m, "Ё")
	if got := len(transmissions(t, log)); got != 1 {
		t.Errorf("typing resized the cover: %d pictures sent", got)
	}
}

func TestADialogOverThePictureHidesItWithoutAnyCommands(t *testing.T) {
	m, log, _ := pictureModel(t, 120, 36)
	before := len(log.String())
	m = keys(t, m, "?")
	if len(log.String()) != before {
		t.Error("opening the help wrote to the terminal")
	}
	contains(t, m, "Keys")
}

func TestQuittingFreesTheOnesPicturesWeSent(t *testing.T) {
	m, log, _ := pictureModel(t, 120, 36)
	sent := transmissions(t, log)[0]["i"]
	_ = keys(t, m, "q")
	cmds, _ := parseAPC(t, log.String())
	last := cmds[len(cmds)-1]
	if last["a"] != "d" || last["d"] != "I" || last["i"] != sent {
		t.Errorf("the last thing written should free the picture we sent (%s), got %v", sent, last)
	}
}

func TestQuittingWithNoPicturesWritesNothing(t *testing.T) {
	_, hook := editingModel(t, "Flow")
	log := &terminalLog{}
	m := New(hook, WithGraphics(log, tenByTwenty))
	m = send(t, m, m.Init(), tea.WindowSizeMsg{Width: 120, Height: 36})
	_ = keys(t, m, "q")
	if log.String() != "" {
		t.Errorf("wrote %q to the terminal for no reason", log.String())
	}
}

func TestATerminalThatCannotTakeThePictureStillGetsTheBlocks(t *testing.T) {
	_, hook := editingModel(t, "Flow")
	tr := trackNamed(t, hook.Library, "Flow")
	if err := appOf(t, hook.Library).SetCover(tr.ID, encodePNG(t, twoTone(60, 60))); err != nil {
		t.Fatal(err)
	}
	log := &terminalLog{err: errors.New("broken pipe")}
	m := New(hook, WithGraphics(log, tenByTwenty))
	m = send(t, m, m.Init(), tea.WindowSizeMsg{Width: 120, Height: 36})
	m = selectTrack(t, m, "Flow")
	contains(t, m, "▀")
	if placeholderCount(m) != 0 {
		t.Error("placeholders drawn for a picture that never arrived")
	}
}

func TestWithoutGraphicsNothingIsWrittenAndBlocksAreDrawn(t *testing.T) {
	m, _ := coverModel(t)
	contains(t, m, "▀")
	if placeholderCount(m) != 0 {
		t.Error("placeholders without the graphics protocol")
	}
}

func TestTheBlocksFillTheRoomTheInspectorHas(t *testing.T) {
	m, _ := coverModel(t)
	cells := strings.Count(screen(m), "▀")
	if cells < 100 {
		t.Errorf("the cover is only %d cells: it should use the room there is", cells)
	}
}
