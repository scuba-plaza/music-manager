//go:build pty

package tui

// Real-terminal tests: they build the actual mm binary, run it in a
// pseudo-terminal of a fixed size, type at it, and read the screen back
// through a terminal emulator. Run them with:
//
//	go test -tags pty -run PTY -v ./internal/tui

import (
	"fmt"
	"io"
	"music-manager/internal/testmp3"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

var (
	buildOnce sync.Once
	mmBinary  string
	buildErr  error
)

func builtMM(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "mm-pty-")
		if err != nil {
			buildErr = err
			return
		}
		mmBinary = filepath.Join(dir, "mm")
		out, err := exec.Command("go", "build", "-o", mmBinary, "../../cmd/mm").CombinedOutput()
		if err != nil {
			buildErr = &buildFailure{string(out)}
		}
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return mmBinary
}

type buildFailure struct{ out string }

func (b *buildFailure) Error() string { return "building mm: " + b.out }

// session is one run of mm in a terminal of rows×cols.
type session struct {
	t    *testing.T
	cmd  *exec.Cmd
	ptmx *os.File
	em   *vt.SafeEmulator
	done chan error

	rawMu sync.Mutex
	raw   []byte // everything the program wrote to the terminal
}

func startMM(t *testing.T, cols, rows int, library string, env ...string) *session {
	t.Helper()
	bin := builtMM(t)
	cmd := exec.Command(bin, "--library", library)
	// Whatever terminal the tests run in, the program sees a plain one, with
	// covers drawn in blocks, unless a test asks for pictures.
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor", "NO_COLOR=", "MM_COVERS=blocks")
	cmd.Env = append(cmd.Env, env...)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		t.Fatal(err)
	}
	s := &session{t: t, cmd: cmd, ptmx: ptmx, em: vt.NewSafeEmulator(cols, rows), done: make(chan error, 1)}
	go io.Copy(io.MultiWriter(s.em, (*rawLog)(s)), ptmx) // what the program draws
	go io.Copy(ptmx, s.em)                               // what the terminal answers to its questions
	go func() { s.done <- cmd.Wait() }()
	t.Cleanup(s.close)
	return s
}

// rawLog lets a session record the bytes the program writes.
type rawLog session

func (r *rawLog) Write(p []byte) (int, error) {
	r.rawMu.Lock()
	defer r.rawMu.Unlock()
	r.raw = append(r.raw, p...)
	return len(p), nil
}

// Raw is everything the program has written to the terminal so far.
func (s *session) Raw() []byte {
	s.rawMu.Lock()
	defer s.rawMu.Unlock()
	return append([]byte(nil), s.raw...)
}

func (s *session) close() {
	select {
	case <-s.done:
	default:
		s.cmd.Process.Kill()
		<-s.done
	}
	s.ptmx.Close()
	s.em.Close()
}

// Screen is the text currently on the screen.
func (s *session) Screen() string { return ansi.Strip(s.em.Render()) }

// Type sends keystrokes one at a time, as typing would, so the program sees
// separate key presses rather than one paste. Escape sequences such as the
// arrow keys (ESC [ A) are kept whole; control keys are written as their bytes.
func (s *session) Type(text string) {
	s.t.Helper()
	for i := 0; i < len(text); {
		n := 1
		switch {
		case text[i] == 0x1b && i+1 < len(text) && text[i+1] == '[':
			n = 2
			for i+n < len(text) && !(text[i+n] >= 0x40 && text[i+n] <= 0x7e) {
				n++
			}
			n = min(n+1, len(text)-i)
		case text[i] >= 0x80: // a multi-byte character
			for i+n < len(text) && text[i+n]&0xC0 == 0x80 {
				n++
			}
		}
		if _, err := s.ptmx.WriteString(text[i : i+n]); err != nil {
			s.t.Fatal(err)
		}
		i += n
		time.Sleep(15 * time.Millisecond)
	}
}

// WaitFor waits until the screen shows all of the strings.
func (s *session) WaitFor(timeout time.Duration, want ...string) bool {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		screen := s.Screen()
		ok := true
		for _, w := range want {
			ok = ok && strings.Contains(screen, w)
		}
		if ok {
			return true
		}
		time.Sleep(25 * time.Millisecond)
	}
	return false
}

// MustSee fails the test, printing the screen, unless the strings appear.
func (s *session) MustSee(want ...string) {
	s.t.Helper()
	if !s.WaitFor(3*time.Second, want...) {
		s.t.Fatalf("the screen never showed %q:\n%s", want, s.Screen())
	}
}

// Exited waits for the program to end and returns its error.
func (s *session) Exited(timeout time.Duration) (exited bool, err error) {
	select {
	case err = <-s.done:
		s.done <- err
		return true, err
	case <-time.After(timeout):
		return false, nil
	}
}

// pty library seeds a library on disk for the real binary to open.
func ptyLibrary(t *testing.T, specs ...seedTrack) string {
	t.Helper()
	a := testApp(t)
	seed(t, a, specs...)
	return a.Root()
}

func TestPTYStartsDrawsAndQuits(t *testing.T) {
	lib := ptyLibrary(t,
		seedTrack{"The Wanderer", "Shawn James", "Shadows", 1, 12, 0},
		seedTrack{"Current", "Phoria", "", 0, 0, 0})
	s := startMM(t, 120, 36, lib)
	s.MustSee("Library", "2 tracks", "q quit")
	t.Logf("\n%s", s.Screen())
	if !s.em.IsAltScreen() {
		t.Error("the interface should use the alternate screen")
	}

	s.Type("q")
	exited, err := s.Exited(3 * time.Second)
	if !exited || err != nil {
		t.Fatalf("exited %v, err %v\n%s", exited, err, s.Screen())
	}
	if s.em.IsAltScreen() {
		t.Error("the terminal was left in the alternate screen")
	}
}

// TestPTYExplore runs mm on a library of your choice and prints the screen,
// for looking at the interface rather than asserting anything:
//
//	MM_PTY_LIB=/some/library MM_PTY_SIZE=120x36 MM_PTY_KEYS=$'/shawn\r' \
//	  go test -tags pty -run PTYExplore -v ./internal/tui
func TestPTYExplore(t *testing.T) {
	lib := os.Getenv("MM_PTY_LIB")
	if lib == "" {
		t.Skip("set MM_PTY_LIB to explore")
	}
	cols, rows := 120, 36
	if size := os.Getenv("MM_PTY_SIZE"); size != "" {
		if _, err := fmt.Sscanf(size, "%dx%d", &cols, &rows); err != nil {
			t.Fatal(err)
		}
	}
	var env []string
	if covers := os.Getenv("MM_PTY_COVERS"); covers != "" {
		env = append(env, "MM_COVERS="+covers)
	}
	s := startMM(t, cols, rows, lib, env...)
	s.MustSee("Library")
	// Keys are sent in groups separated by "|", with a pause between groups.
	for _, group := range strings.Split(os.Getenv("MM_PTY_KEYS"), "|") {
		s.Type(group)
		time.Sleep(250 * time.Millisecond)
	}
	t.Logf("\n%s", s.Screen())
}

func TestPTYEditATitleAndItIsSaved(t *testing.T) {
	a := testApp(t)
	seed(t, a,
		seedTrack{"The Wanderer", "Shawn James", "Shadows", 1, 12, 0},
		seedTrack{"Flow", "Shawn James", "Shadows", 7, 12, 0})
	s := startMM(t, 100, 30, a.Root())
	s.MustSee("Shawn James")

	s.Type("/flow\r")            // find it
	s.Type("\x1b[B\x1b[B\x1b[B") // down to the track
	s.Type("\x1b[C")             // into the inspector
	s.Type("\r")                 // edit the title
	s.MustSee("enter save")
	s.Type("\x15Flow Again") // clear the field and type
	s.MustSee("Shawn James/Shadows/07 - Flow Again.mp3")
	s.Type("\r")
	s.MustSee("Saved")

	s.Type("q")
	if exited, err := s.Exited(3 * time.Second); !exited || err != nil {
		t.Fatalf("exited %v, err %v\n%s", exited, err, s.Screen())
	}
	m, err := a.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tr := range m.Tracks {
		found = found || tr.Title == "Flow Again"
	}
	if !found {
		t.Errorf("the edit never reached library.json: %+v", m.Tracks)
	}
}

func TestPTYTrimWithAPreview(t *testing.T) {
	a := testApp(t)
	seed(t, a, seedTrack{"Flow", "Shawn James", "Shadows", 7, 12, 400})
	bin := t.TempDir()
	argsFile := filepath.Join(bin, "args")
	existed := filepath.Join(bin, "existed")
	fake := "#!/bin/sh\necho \"$@\" > " + argsFile + "\nfor a in \"$@\"; do [ -f \"$a\" ] && echo yes > " + existed + "; done\nsleep 30\n"
	if err := os.WriteFile(filepath.Join(bin, "ffplay"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	s := startMM(t, 110, 30, a.Root(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	s.MustSee("Shawn James")
	s.Type("/flow\r\x1b[B\x1b[B\x1b[B") // find the track
	s.Type("\x1b[C")                    // inspector
	s.Type("\x1b[B\x1b[B\x1b[B\x1b[B")  // down to Cut start
	s.Type("}}")                        // cut two seconds
	s.MustSee("Saved", "Kept", "░")
	t.Logf("\n%s", s.Screen())

	s.Type("p")
	s.MustSee("Playing the start cut")
	time.Sleep(300 * time.Millisecond)
	args, _ := os.ReadFile(argsFile)
	if !strings.Contains(string(args), "-nodisp") || !strings.Contains(string(args), ".mp3") {
		t.Errorf("ffplay was run with %q", args)
	}
	if _, err := os.Stat(existed); err != nil {
		t.Error("the preview file did not exist while it was being played")
	}

	s.Type("\x1b") // esc stops the sound
	time.Sleep(300 * time.Millisecond)
	if strings.Contains(s.Screen(), "Playing the") {
		t.Errorf("still playing after esc:\n%s", s.Screen())
	}
	s.Type("q")
	if exited, err := s.Exited(3 * time.Second); !exited || err != nil {
		t.Fatalf("exited %v, err %v", exited, err)
	}
}

func TestPTYCoverIsDrawnInColour(t *testing.T) {
	a := testApp(t)
	tr := seed(t, a, seedTrack{"Flow", "Shawn James", "Shadows", 7, 12, 0})[0]
	if err := a.SetCover(tr.ID, encodePNGBytes(t)); err != nil {
		t.Fatal(err)
	}
	s := startMM(t, 110, 34, a.Root(), "COLORTERM=truecolor")
	s.MustSee("Shawn James")
	s.Type("/flow\r\x1b[B\x1b[B\x1b[B") // the track
	s.MustSee("40×40 PNG", "▀")

	raw := s.em.Render() // with the colour codes
	for name, rgb := range map[string]string{"red": "255;0;0", "blue": "0;0;255"} {
		if !strings.Contains(raw, rgb) {
			t.Errorf("no %s (%s) in the picture's colour codes", name, rgb)
		}
	}
}

func TestPTYIdentifyThroughThePicker(t *testing.T) {
	// A stand-in MusicBrainz that answers every search with a recorded response.
	canned, err := os.ReadFile("../musicbrainz/testdata/search_dream3.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/ws/2/recording") {
			w.Write(canned)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	a := testApp(t)
	seedTagged(t, a, "01 Dream", "Dream 3", "Max Richter", "From Sleep", "1/7")
	s := startMM(t, 120, 34, a.Root(), "MM_MUSICBRAINZ_URL="+srv.URL, "MM_COVERART_URL="+srv.URL)
	s.MustSee("Needs work")
	s.Type("\r\x1b[B\x1b[C") // open the group, go to the track, into the inspector
	s.Type("i")
	s.MustSee("Identify", "From Sleep", "This would change")
	t.Logf("\n%s", s.Screen())

	s.Type("\r")
	s.MustSee("Identified as")
	s.Type("q")
	if exited, err := s.Exited(3 * time.Second); !exited || err != nil {
		t.Fatalf("exited %v, err %v\n%s", exited, err, s.Screen())
	}
	m, err := a.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Tracks[0]; got.Status != "identified" || got.MBRecordingID == "" || got.Album != "From Sleep" {
		t.Errorf("track = %+v", got)
	}
}

func TestPTYAddFromALinkWithAFakeYtDlp(t *testing.T) {
	bin := t.TempDir()
	fixture := filepath.Join(bin, "fixture.mp3")
	if err := os.WriteFile(fixture, testmp3.Silent(150), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprev=\"\"; for a in \"$@\"; do [ \"$prev\" = \"-o\" ] && out=\"$a\"; prev=\"$a\"; done\n" +
		"file=$(printf '%s' \"$out\" | sed 's/%(id)s/vid123/; s/%(ext)s/mp3/')\ncp " + fixture + " \"$file\"\n" +
		"printf '%s\\n' '{\"id\":\"vid123\",\"title\":\"Pty Song\",\"uploader\":\"Pty Band - Topic\",\"album\":\"Pty Album\",\"track_number\":2}'\n"
	if err := os.WriteFile(filepath.Join(bin, "yt-dlp"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// A stand-in MusicBrainz that finds nothing.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"recordings":[]}`)) }))
	defer srv.Close()

	a := testApp(t)
	seed(t, a, seedTrack{"Anchor", "Band", "Album", 1, 1, 0})
	s := startMM(t, 120, 36, a.Root(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "MM_MUSICBRAINZ_URL="+srv.URL)
	s.MustSee("Band")
	s.Type("a")
	s.MustSee("Add from YouTube", "Paste a link")
	s.Type("https://youtu.be/abc")
	s.Type("\r")
	s.MustSee("Pty Song", "Pty Band", "Pty Album", "Pty Band/Pty Album/02 - Pty Song.mp3")
	t.Logf("\n%s", s.Screen())

	s.Type("\r")
	s.MustSee("Added")
	s.Type("q")
	if exited, err := s.Exited(3 * time.Second); !exited || err != nil {
		t.Fatalf("exited %v, err %v\n%s", exited, err, s.Screen())
	}
	m, err := a.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tr := range m.Tracks {
		found = found || (tr.Title == "Pty Song" && tr.SourceURL == "https://youtu.be/abc" && tr.Album == "Pty Album")
	}
	if !found {
		t.Errorf("the added track is not in library.json: %+v", m.Tracks)
	}
}

func TestPTYSyncWritesTheLibraryFolder(t *testing.T) {
	a := testApp(t)
	seed(t, a,
		seedTrack{"The Wanderer", "Shawn James", "Shadows", 1, 12, 0},
		seedTrack{"Flow", "Shawn James", "Shadows", 7, 12, 0},
		seedTrack{"Current", "Phoria", "Caught a Black Rabbit", 4, 10, 0})
	stray := filepath.Join(a.Root(), "Old", "leftover.mp3")
	os.MkdirAll(filepath.Dir(stray), 0o755)
	os.WriteFile(stray, []byte("old"), 0o644)

	s := startMM(t, 110, 32, a.Root())
	s.MustSee("3 tracks", "3 to sync")
	s.Type("s")
	s.MustSee("Where should the music be written?", "Library folder", "Another folder")
	s.Type("\r")
	s.MustSee("3 to create", "1 stray", "Shawn James/Shadows/07 - Flow.mp3", "Old/leftover.mp3")
	t.Logf("\n%s", s.Screen())

	s.Type("p")
	s.MustSee("1 to delete", "will be deleted")
	s.Type("\r")
	s.MustSee("Type delete")
	s.Type("delete\r")
	s.MustSee("Done", "3 created", "1 deleted")
	s.Type("\r")
	if strings.Contains(s.Screen(), "to sync") {
		// the status line catches up a moment later
		time.Sleep(300 * time.Millisecond)
	}
	if strings.Contains(s.Screen(), "to sync") {
		t.Errorf("still says there is something to sync:\n%s", s.Screen())
	}
	s.Type("q")
	if exited, err := s.Exited(3 * time.Second); !exited || err != nil {
		t.Fatalf("exited %v, err %v", exited, err)
	}
	if _, err := os.Stat(filepath.Join(a.Root(), "Phoria", "Caught a Black Rabbit", "04 - Current.mp3")); err != nil {
		t.Errorf("not written: %v", err)
	}
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Error("the stray should be gone")
	}
}

// mouseClick is what a terminal sends for a left click at column x, row y
// (counted from 0), pressed and released.
func mouseClick(x, y int) string {
	return fmt.Sprintf("\x1b[<0;%d;%dM\x1b[<0;%d;%dm", x+1, y+1, x+1, y+1)
}

func TestPTYTheMouseSelectsOpensAndEdits(t *testing.T) {
	lib := ptyLibrary(t,
		seedTrack{"The Wanderer", "Shawn James", "Shadows", 1, 12, 0},
		seedTrack{"Current", "Phoria", "", 0, 0, 0})
	s := startMM(t, 120, 36, lib)
	s.MustSee("Phoria", "Shawn James")

	s.Type(mouseClick(10, 4)) // the second row, Shawn James
	s.Type(mouseClick(10, 4)) // again: it opens
	s.MustSee("Shadows")
	s.Type(mouseClick(10, 5)) // the album
	s.Type(mouseClick(10, 5))
	s.MustSee("The Wanderer")
	s.Type(mouseClick(14, 6)) // the track
	s.Type(mouseClick(14, 6)) // again: into its fields
	s.MustSee("Cut start")
	s.Type(mouseClick(80, 3)) // the title field, already chosen: edit
	s.MustSee("enter save")
}
