package manifest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestChangesToTheLibraryTakeTurns(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "library.json"))
	inside, release := make(chan struct{}), make(chan struct{})
	first := make(chan error, 1)
	go func() {
		first <- s.Update(func(m *Manifest) error {
			close(inside)
			<-release
			m.Add(Track{Title: "first"})
			return nil
		})
	}()
	<-inside

	second := make(chan error, 1)
	go func() {
		second <- s.Update(func(m *Manifest) error {
			m.Add(Track{Title: "second"})
			return nil
		})
	}()
	select {
	case <-second:
		t.Fatal("a second change ran while the first was still being made")
	case <-time.After(150 * time.Millisecond):
	}

	close(release)
	for _, done := range []chan error{first, second} {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	m, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Tracks) != 2 {
		t.Errorf("%d tracks, want both changes: %+v", len(m.Tracks), m.Tracks)
	}
}

func TestAChangeThatFailsSavesNothing(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "library.json"))
	if err := s.Update(func(m *Manifest) error { m.Add(Track{Title: "kept"}); return nil }); err != nil {
		t.Fatal(err)
	}
	err := s.Update(func(m *Manifest) error {
		m.Add(Track{Title: "dropped"})
		return os.ErrInvalid
	})
	if err != os.ErrInvalid {
		t.Fatalf("err = %v, want the change's own error", err)
	}
	m, _ := s.Load()
	if len(m.Tracks) != 1 || m.Tracks[0].Title != "kept" {
		t.Errorf("got %+v, want only the first change", m.Tracks)
	}
}

func TestHoldingTheLibraryKeepsChangesWaitingButChangesNothing(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "library.json"))
	if err := s.Update(func(m *Manifest) error { m.Add(Track{Title: "one"}); return nil }); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(s.path)

	var seen int
	if err := s.Hold(func(m *Manifest) error { seen = len(m.Tracks); m.Add(Track{Title: "not saved"}); return nil }); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(s.path); string(after) != string(before) || seen != 1 {
		t.Errorf("Hold saw %d tracks and left the file %s", seen, map[bool]string{true: "as it was", false: "changed"}[string(after) == string(before)])
	}

	inside, release := make(chan struct{}), make(chan struct{})
	held := make(chan error, 1)
	go func() {
		held <- s.Hold(func(*Manifest) error { close(inside); <-release; return nil })
	}()
	<-inside
	changed := make(chan error, 1)
	go func() { changed <- s.Update(func(*Manifest) error { return nil }) }()
	select {
	case <-changed:
		t.Fatal("a change was made while the library was held")
	case <-time.After(150 * time.Millisecond):
	}
	close(release)
	if err := <-held; err != nil {
		t.Fatal(err)
	}
	if err := <-changed; err != nil {
		t.Fatal(err)
	}
}

func TestWaitingForAnotherMmGivesUpWithAMessage(t *testing.T) {
	defer func(d time.Duration) { lockWait = d }(lockWait)
	lockWait = 100 * time.Millisecond
	s := NewStore(filepath.Join(t.TempDir(), "library.json"))
	inside, release := make(chan struct{}), make(chan struct{})
	go s.Hold(func(*Manifest) error { close(inside); <-release; return nil })
	<-inside
	defer close(release)

	err := s.Update(func(*Manifest) error { t.Error("the change ran without the lock"); return nil })
	if err == nil || !strings.Contains(err.Error(), "another mm") {
		t.Errorf("err = %v, want it to say that another mm has the library", err)
	}
}

// TestAnMmThatDiesHoldingTheLibraryHoldsItNoLonger runs another process, the
// test binary itself, that takes the library and never gives it back, and
// kills it: a lock that outlived its process would lock the library for good.
func TestAnMmThatDiesHoldingTheLibraryHoldsItNoLonger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	s := NewStore(path)

	cmd := exec.Command(os.Args[0], "-test.run=^TestHoldTheLibraryUntilKilled$")
	cmd.Env = append(os.Environ(), "MM_TEST_HOLD="+path)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	for start := time.Now(); ; time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(path + ".held"); err == nil {
			break
		}
		if time.Since(start) > 10*time.Second {
			t.Fatal("the other process never took the library")
		}
	}

	defer func(d time.Duration) { lockWait = d }(lockWait)
	lockWait = 200 * time.Millisecond
	if err := s.Update(func(*Manifest) error { return nil }); err == nil {
		t.Fatal("the library could be changed while another process held it")
	}

	cmd.Process.Kill()
	cmd.Wait()
	lockWait = 10 * time.Second
	if err := s.Update(func(m *Manifest) error { m.Add(Track{Title: "after"}); return nil }); err != nil {
		t.Fatalf("the library is still locked after the process that held it died: %v", err)
	}
}

// TestHoldTheLibraryUntilKilled is the other process of the test above; on its
// own it does nothing.
func TestHoldTheLibraryUntilKilled(t *testing.T) {
	path := os.Getenv("MM_TEST_HOLD")
	if path == "" {
		t.Skip("only runs as the other process of another test")
	}
	err := NewStore(path).Hold(func(*Manifest) error {
		os.WriteFile(path+".held", nil, 0o644)
		time.Sleep(time.Minute)
		return nil
	})
	t.Fatal(err)
}
