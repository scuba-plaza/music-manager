package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"music-manager/internal/app"
)

func plainBar(total, start, end time.Duration, width int) string {
	return ansi.Strip(trimBar(total, start, end, width))
}

func TestTrimBarShowsWhatIsKeptAndWhatIsCut(t *testing.T) {
	s := time.Second
	cases := []struct {
		total, start, end time.Duration
		width             int
		want              string
	}{
		{10 * s, 0, 0, 10, "██████████"},
		{10 * s, 2 * s, 3 * s, 10, "░░█████░░░"},
		{10 * s, 2 * s, 0, 10, "░░████████"},
		{10 * s, 0, 5 * s, 10, "█████░░░░░"},
		{10 * s, 10 * s, 0, 10, "░░░░░░░░░░"},
		{10 * s, 6 * s, 6 * s, 10, "░░░░░░░░░░"},
		{10 * s, 0, 0, 1, "█"},
		{20 * s, 5 * s, 0, 4, "░███"},
		{0, 0, 0, 5, "     "},
		{10 * s, 0, 0, 0, ""},
	}
	for _, c := range cases {
		if got := plainBar(c.total, c.start, c.end, c.width); got != c.want {
			t.Errorf("trimBar(%v, %v, %v, %d) = %q, want %q", c.total, c.start, c.end, c.width, got, c.want)
		}
	}
}

func TestNudgingTheCutsFromTheKeyboard(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = focusField(t, m, fCutStart)
	if trackFields[m.field] != fCutStart {
		t.Fatalf("on %v", trackFields[m.field])
	}
	start := func() time.Duration { return trackNamed(t, hook.Library, "Flow").TrimStart }
	end := func() time.Duration { return trackNamed(t, hook.Library, "Flow").TrimEnd }

	m = keys(t, m, "]")
	if start() != 100*time.Millisecond {
		t.Errorf("] = %v, want +0.1s", start())
	}
	m = keys(t, m, "}")
	if start() != 1100*time.Millisecond {
		t.Errorf("} = %v, want +1s", start())
	}
	m = keys(t, m, "[")
	if start() != time.Second {
		t.Errorf("[ = %v, want -0.1s", start())
	}
	m = keys(t, m, "{")
	contains(t, m, "Saved")
	m = keys(t, m, "{")
	if start() != 0 {
		t.Errorf("never below zero, got %v", start())
	}
	lacks(t, m, "Saved") // nothing changed, so nothing was saved

	m = keys(t, m, "down", "}")
	if end() != time.Second {
		t.Errorf("the end cut was not nudged: %v", end())
	}
	m = keys(t, m, "0")
	if end() != 0 {
		t.Errorf("0 should clear the cut, got %v", end())
	}
}

func TestACutCannotSwallowTheWholeTrack(t *testing.T) {
	m, hook := editingModel(t, "Flow") // about 2.6s long
	m = focusField(t, m, fCutStart)
	for i := 0; i < 6; i++ {
		m = keys(t, m, "}")
	}
	tr := trackNamed(t, hook.Library, "Flow")
	if tr.TrimStart >= tr.SourceLength || tr.PlayingLength() < 100*time.Millisecond {
		t.Errorf("cut start %v of %v leaves %v", tr.TrimStart, tr.SourceLength, tr.PlayingLength())
	}
	m = keys(t, m, "down", "}", "}", "}")
	tr = trackNamed(t, hook.Library, "Flow")
	if tr.PlayingLength() < 100*time.Millisecond {
		t.Errorf("both cuts leave only %v", tr.PlayingLength())
	}
}

func TestOneUndoRevertsAWholeRunOfNudges(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = focusField(t, m, fCutStart)
	m = keys(t, m, "]", "]", "]", "}")
	if got := trackNamed(t, hook.Library, "Flow").TrimStart; got != 1300*time.Millisecond {
		t.Fatalf("setup: %v", got)
	}
	m = keys(t, m, "u")
	if got := trackNamed(t, hook.Library, "Flow").TrimStart; got != 0 {
		t.Errorf("one undo left the cut at %v", got)
	}
	m = keys(t, m, "u")
	contains(t, m, "Nothing to undo")

	// Moving to another field starts a new step.
	m = keys(t, m, "]", "down", "]")
	m = keys(t, m, "u")
	tr := trackNamed(t, hook.Library, "Flow")
	if tr.TrimStart != 100*time.Millisecond || tr.TrimEnd != 0 {
		t.Errorf("after one undo: start %v, end %v", tr.TrimStart, tr.TrimEnd)
	}
}

func TestTheBarFollowsTheCuts(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "right", "2") // the bar is on the audio tab
	count := func() int { return strings.Count(screen(m), "█") }
	before := count()
	if before == 0 {
		t.Fatal("no bar on screen")
	}
	m = focusField(t, m, fCutStart)
	m = keys(t, m, "}", "}")
	if after := count(); after >= before {
		t.Errorf("cutting %d cells of bar became %d", before, after)
	}
	contains(t, m, "░")
}

func TestTheBarFollowsATypedCutBeforeItIsSaved(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = focusField(t, m, fCutStart)
	whole := strings.Count(screen(m), "█")
	m = keys(t, m, "enter")
	m = typed(t, m, "0:01")
	if now := strings.Count(screen(m), "█"); now >= whole {
		t.Errorf("the bar should shrink as you type: %d -> %d", whole, now)
	}
}

// fakePlayer records what it was asked to play and lets a test end it.
type fakePlayer struct {
	played   []string
	stopped  int
	finished chan error
	length   time.Duration
}

func newFakePlayer() *fakePlayer { return &fakePlayer{finished: make(chan error, 1)} }

func (f *fakePlayer) Play(path string) (stop func(), done <-chan error, err error) {
	f.played = append(f.played, path)
	if data, rerr := os.ReadFile(path); rerr == nil {
		f.length = int64Len(data)
	}
	return func() {
		f.stopped++
		select {
		case f.finished <- nil:
		default:
		}
	}, f.finished, nil
}

func int64Len(b []byte) time.Duration { return time.Duration(len(b)) } // only used to see the file existed

func playerModel(t *testing.T, title string, p Player) (Model, *editHook) {
	t.Helper()
	m, hook := editingModel(t, title)
	m.player = p
	return m, hook
}

func TestPreviewPlaysTheAudioAroundTheCut(t *testing.T) {
	p := newFakePlayer()
	m, hook := playerModel(t, "Flow", p)
	tr := trackNamed(t, hook.Library, "Flow")
	if _, err := hook.Library.Edit(tr.ID, appEditStart(300*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	m = send(t, m, m.load())

	m = focusField(t, m, fCutStart)
	m = keys(t, m, "p")
	if len(p.played) != 1 {
		t.Fatalf("played %v", p.played)
	}
	if p.length == 0 {
		t.Error("the preview file did not exist when it was played")
	}
	contains(t, m, "Playing")
	if m.playing == nil {
		t.Fatal("the model should know it is playing")
	}

	// Stopping cleans the temporary file up.
	path := p.played[0]
	m = keys(t, m, "p")
	if p.stopped != 1 || m.playing != nil {
		t.Errorf("stopped %d times, playing %v", p.stopped, m.playing)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the temporary preview %s was left behind", path)
	}
}

func TestPreviewEndsByItselfAndCleansUp(t *testing.T) {
	p := newFakePlayer()
	m, _ := playerModel(t, "Flow", p)
	m = focusField(t, m, fCutEnd) // the end cut
	m = keys(t, m, "p")
	path := p.played[0]
	p.finished <- nil
	m = send(t, m, playbackDoneMsg{id: m.playing.id})
	if m.playing != nil {
		t.Error("still playing after it finished")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("%s left behind", path)
	}
}

func TestEscapeStopsThePreview(t *testing.T) {
	p := newFakePlayer()
	m, _ := playerModel(t, "Flow", p)
	m = focusField(t, m, fCutStart)
	m = keys(t, m, "p", "esc")
	if p.stopped != 1 || m.playing != nil {
		t.Errorf("stopped %d, playing %v", p.stopped, m.playing)
	}
	if m.focus != paneInspector {
		t.Error("esc should only stop the sound the first time")
	}
}

func TestQuittingStopsThePreview(t *testing.T) {
	p := newFakePlayer()
	m, _ := playerModel(t, "Flow", p)
	m = focusField(t, m, fCutStart)
	m = keys(t, m, "p")
	if _, cmd := m.Update(press("ctrl+c")); !quits(cmd) || p.stopped != 1 {
		t.Errorf("stopped %d", p.stopped)
	}
}

func TestPreviewOnlyWorksOnTheCuts(t *testing.T) {
	p := newFakePlayer()
	m, _ := playerModel(t, "Flow", p)
	m = keys(t, m, "right", "p") // on Title
	if len(p.played) != 0 {
		t.Error("played from the title field")
	}
}

func TestWithoutAPlayerPreviewExplainsWhatToInstall(t *testing.T) {
	m, _ := playerModel(t, "Flow", nil)
	m = focusField(t, m, fCutStart)
	m = keys(t, m, "p")
	contains(t, m, "ffplay", "mpv")
}

func TestAPlayerThatCannotStartIsReported(t *testing.T) {
	m, _ := playerModel(t, "Flow", brokenPlayer{})
	m = focusField(t, m, fCutStart)
	m = keys(t, m, "p")
	contains(t, m, "no sound card")
	if m.playing != nil {
		t.Error("playing after a failure")
	}
}

type brokenPlayer struct{}

func (brokenPlayer) Play(string) (func(), <-chan error, error) { return nil, nil, errNoSound }

func TestDetectPlayerPrefersFfplayThenMpvAndOtherwiseNone(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\nexit 0\n"
	t.Setenv("PATH", bin)
	if DetectPlayer() != nil {
		t.Error("found a player in an empty PATH")
	}
	os.WriteFile(filepath.Join(bin, "mpv"), []byte(script), 0o755)
	if p, ok := DetectPlayer().(*execPlayer); !ok || filepath.Base(p.bin) != "mpv" {
		t.Errorf("expected mpv, got %#v", DetectPlayer())
	}
	os.WriteFile(filepath.Join(bin, "ffplay"), []byte(script), 0o755)
	if p, ok := DetectPlayer().(*execPlayer); !ok || filepath.Base(p.bin) != "ffplay" {
		t.Errorf("expected ffplay to win, got %#v", DetectPlayer())
	}
}

func TestExecPlayerRunsTheProgramOnTheFileAndCanBeStopped(t *testing.T) {
	bin := t.TempDir()
	argsFile := filepath.Join(bin, "args")
	os.WriteFile(filepath.Join(bin, "ffplay"), []byte("#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> "+argsFile+"; done\nsleep 30\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	p := DetectPlayer()
	stop, done, err := p.Play("/tmp/clip.mp3")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	var args string
	for time.Now().Before(deadline) {
		if b, _ := os.ReadFile(argsFile); len(b) > 0 {
			args = string(b)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, want := range []string{"-nodisp", "-autoexit", "/tmp/clip.mp3"} {
		if !strings.Contains(args, want) {
			t.Errorf("ffplay was run with %q, lacking %q", args, want)
		}
	}
	stop()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("stop did not end the player")
	}
}

var errNoSound = errors.New("no sound card")

func appEditStart(d time.Duration) app.Edit { return app.Edit{TrimStart: &d} }
