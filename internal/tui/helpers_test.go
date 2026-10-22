package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/testmp3"
)

// testApp returns an empty library in a temporary directory.
func testApp(t *testing.T) *app.App {
	t.Helper()
	a, err := app.Open(filepath.Join(t.TempDir(), "lib"))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

type seedTrack struct {
	title, artist, album string
	n, of                int
	frames               int // 0 means 100 (about 2.6s)
}

// seed adds identified tracks to a, one real audio file each, and returns
// them in order. Unidentified tracks have an empty title or artist.
func seed(t *testing.T, a *app.App, specs ...seedTrack) []manifest.Track {
	t.Helper()
	src := t.TempDir()
	for i, s := range specs {
		frames := s.frames
		if frames == 0 {
			frames = 100
		}
		name := filepath.Join(src, string(rune('a'+i/26))+string(rune('a'+i%26))+".mp3")
		if err := os.WriteFile(name, testmp3.Silent(frames+i), 0o644); err != nil { // +i makes every audio distinct
			t.Fatal(err)
		}
	}
	if _, err := a.Ingest([]string{src}); err != nil {
		t.Fatal(err)
	}
	m, err := a.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range specs {
		if s.title == "" || s.artist == "" {
			continue
		}
		e := app.Edit{Title: &s.title, Artist: &s.artist, Album: &s.album, TrackNumber: &s.n, TrackTotal: &s.of}
		if _, err := a.Edit(m.Tracks[i].ID, e); err != nil {
			t.Fatal(err)
		}
	}
	m, _ = a.Manifest()
	return m.Tracks
}

// newTestModel returns a model on lib, sized w×h, with its tracks loaded.
func newTestModel(t *testing.T, lib Library, w, h int) Model {
	t.Helper()
	m := New(lib)
	return send(t, m, m.Init(), tea.WindowSizeMsg{Width: w, Height: h})
}

// send feeds messages to the model, running every command it returns and
// feeding those results back too. Commands that take long (timers, spinner
// ticks) are dropped, so tests never wait for them.
func send(t *testing.T, m Model, msgs ...any) Model {
	t.Helper()
	queue := append([]any(nil), msgs...)
	for steps := 0; len(queue) > 0; steps++ {
		if steps > 1000 {
			t.Fatal("the model keeps producing messages")
		}
		item := queue[0]
		queue = queue[1:]
		switch v := item.(type) {
		case nil:
		case tea.Cmd:
			if msg := runCmd(v); msg != nil {
				queue = append(queue, msg)
			}
		case tea.BatchMsg:
			for _, c := range v {
				queue = append(queue, c)
			}
		case tea.Msg:
			next, cmd := m.Update(v)
			m = next.(Model)
			if cmd != nil {
				queue = append(queue, cmd)
			}
		}
	}
	return m
}

// runCmd runs a command, giving up on those that wait (timers, ticks).
func runCmd(c tea.Cmd) tea.Msg {
	if c == nil {
		return nil
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- c() }()
	select {
	case msg := <-done:
		return msg
	case <-time.After(300 * time.Millisecond):
		return nil
	}
}

func press(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "ctrl+r":
		return tea.KeyMsg{Type: tea.KeyCtrlR}
	case "ctrl+a":
		return tea.KeyMsg{Type: tea.KeyCtrlA}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// keys sends a sequence like "down", "down", "enter", or text to type.
func keys(t *testing.T, m Model, ks ...string) Model {
	t.Helper()
	for _, k := range ks {
		m = send(t, m, press(k))
	}
	return m
}

// typed sends text one character at a time, as typing would.
func typed(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		m = send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

// screen is the plain text on screen.
func screen(m Model) string { return ansi.Strip(m.View()) }

// lines is the screen split into lines.
func lines(m Model) []string { return strings.Split(screen(m), "\n") }

func contains(t *testing.T, m Model, want ...string) {
	t.Helper()
	s := screen(m)
	for _, w := range want {
		if !strings.Contains(s, w) {
			t.Errorf("the screen lacks %q:\n%s", w, s)
		}
	}
}

func lacks(t *testing.T, m Model, unwanted ...string) {
	t.Helper()
	s := screen(m)
	for _, w := range unwanted {
		if strings.Contains(s, w) {
			t.Errorf("the screen has %q:\n%s", w, s)
		}
	}
}

// quits reports whether running cmd quits the program.
func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := runCmd(cmd).(tea.QuitMsg)
	return ok
}

var _ = context.Background
