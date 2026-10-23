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

func init() { animate, polling = false, false }

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
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
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

// libTracks reads the records straight from the library.
func libTracks(t *testing.T, lib Library) []manifest.Track {
	t.Helper()
	ts, err := lib.Tracks()
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// trackNamed finds a record by its title (or its old title if not identified).
func trackNamed(t *testing.T, lib Library, title string) manifest.Track {
	t.Helper()
	for _, tr := range libTracks(t, lib) {
		if tr.Title == title || tr.Hint.Title == title {
			return tr
		}
	}
	t.Fatalf("no track titled %q", title)
	return manifest.Track{}
}

// selectTrack opens the tree down to the track with the title and selects it.
func selectTrack(t *testing.T, m Model, title string) Model {
	t.Helper()
	for _, tr := range m.tracks {
		if tr.Title == title || tr.Hint.Title == title {
			m.revealTrack(tr)
			return send(t, m, tea.WindowSizeMsg{Width: m.w, Height: m.h}) // a no-op that lets covers load
		}
	}
	t.Fatalf("no track titled %q on screen", title)
	return m
}

// appOf returns the app behind a Library, however it is wrapped.
func appOf(t *testing.T, lib Library) *app.App {
	t.Helper()
	switch l := lib.(type) {
	case *editHook:
		return appOf(t, l.Library)
	case appLibrary:
		return l.App
	}
	t.Fatalf("no app behind %T", lib)
	return nil
}

// clearField empties the input being edited, as ctrl+u does.
func clearField(t *testing.T, m Model) Model { return keys(t, m, "ctrl+u") }

// editWith wraps a Library so tests can watch or break a call.
type editHook struct {
	Library
	calls     int
	manyCalls int
	err       error
}

func (h *editHook) Edit(id string, e app.Edit) (manifest.Track, error) {
	h.calls++
	if h.err != nil {
		return manifest.Track{}, h.err
	}
	return h.Library.Edit(id, e)
}

// seedTagged ingests a file whose old tags are given, so it has hints.
func seedTagged(t *testing.T, a *app.App, name, title, artist, album, track string) manifest.Track {
	t.Helper()
	f := filepath.Join(t.TempDir(), name+".mp3")
	if err := os.WriteFile(f, testmp3.WithTagsTrack(testmp3.Silent(60+len(name)), title, artist, album, track), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := a.Ingest([]string{f})
	if err != nil || len(rep.Added) != 1 {
		t.Fatalf("ingest %s: %+v, %v", name, rep, err)
	}
	return rep.Added[0]
}

// encodePNGBytes is a 40×40 picture, red on the left and blue on the right.
func encodePNGBytes(t *testing.T) []byte {
	t.Helper()
	return encodePNG(t, twoTone(40, 40))
}

func (h *editHook) EditMany(edits map[string]app.Edit) (map[string]manifest.Track, error) {
	h.manyCalls++
	return h.Library.EditMany(edits)
}

func (h *editHook) Renumber(ids []string) (map[string]manifest.Track, error) {
	h.manyCalls++
	return h.Library.Renumber(ids)
}

func (h *editHook) RemoveMany(ids []string) ([]manifest.Track, error) {
	h.manyCalls++
	return h.Library.RemoveMany(ids)
}

func ansiStrip(s string) string { return ansi.Strip(s) }

// loadedMsgFor is what loading the library gives, for a test that changed it
// behind the model's back.
func loadedMsgFor(t *testing.T, lib Library) loadedMsg {
	t.Helper()
	ts, err := lib.Tracks()
	if err != nil {
		t.Fatal(err)
	}
	pls, err := lib.Playlists()
	if err != nil {
		t.Fatal(err)
	}
	stamp, _ := lib.Stamp()
	return loadedMsg{tracks: ts, playlists: pls, stamp: stamp}
}
