package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/render"
)

func TestOpCounts(t *testing.T) {
	ops := []render.Op{
		{Kind: render.Create, Path: "a"}, {Kind: render.Create, Path: "b"},
		{Kind: render.Update, Path: "c"}, {Kind: render.Stray, Path: "d"}, {Kind: render.Remove, Path: "e"}, {Kind: render.Remove, Path: "f"},
	}
	c := countOps(ops)
	if c.create != 2 || c.update != 1 || c.stray != 1 || c.remove != 2 {
		t.Errorf("= %+v", c)
	}
	for _, want := range []string{"2 to create", "1 to update", "1 stray", "2 to delete"} {
		if !strings.Contains(c.text(), want) {
			t.Errorf("%q lacks %q", c.text(), want)
		}
	}
	if got := (opCounts{}).text(); got != "Nothing to do" {
		t.Errorf("= %q", got)
	}
	if c.writes() != 3 {
		t.Errorf("writes = %d", c.writes())
	}
}

func TestMountCandidatesAreTheFoldersUnderTheMountRoots(t *testing.T) {
	root := t.TempDir()
	userRoot := filepath.Join(root, "run", "media", "tau")
	plain := filepath.Join(root, "media")
	for _, d := range []string{filepath.Join(userRoot, "PLAYER"), filepath.Join(userRoot, "STICK"), filepath.Join(plain, "OTHER"), filepath.Join(plain, "tau")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(userRoot, "a file"), nil, 0o644)

	got := mountCandidates([]string{userRoot, plain, filepath.Join(root, "missing")}, "tau")
	want := []string{filepath.Join(userRoot, "PLAYER"), filepath.Join(userRoot, "STICK"), filepath.Join(plain, "OTHER")}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("= %v, want %v (files, missing folders and the user's own folder are not drives)", got, want)
	}
}

// syncModel is a library whose files have not been written yet.
func syncModel(t *testing.T) (Model, *editHook) {
	t.Helper()
	m, lib := browserModel(t, 120, 36)
	hook := &editHook{Library: lib}
	m.lib = hook
	m.mounts = func() []string { return nil } // not the drives of the machine the tests run on
	return m, hook
}

func TestSOpensTheChooserWithTheLibraryFolderFirst(t *testing.T) {
	m, hook := syncModel(t)
	m.mounts = func() []string { return []string{"/run/media/tau/PLAYER"} }
	m = keys(t, m, "s")
	contains(t, m, "Sync", "Library folder", tilde(hook.Library.Root()), "/run/media/tau/PLAYER", "Another folder")
	if d := asSync(t, m); d.cursor != 0 {
		t.Errorf("cursor = %d, want the library folder", d.cursor)
	}
	m = keys(t, m, "esc")
	if m.overlay != nil {
		t.Error("esc should close the chooser")
	}
}

func asSync(t *testing.T, m Model) *syncDialog {
	t.Helper()
	d, ok := m.overlay.(*syncDialog)
	if !ok {
		t.Fatalf("no sync dialog:\n%s", screen(m))
	}
	return d
}

func TestSyncShowsThePlanBeforeWritingAnything(t *testing.T) {
	m, hook := syncModel(t)
	m = keys(t, m, "s", "enter")
	contains(t, m, "6 to create", "+", "Shawn James/Shadows/01 - The Wanderer.mp3", "enter apply")
	if _, err := os.Stat(filepath.Join(hook.Library.Root(), "Shawn James")); !os.IsNotExist(err) {
		t.Error("the plan wrote files")
	}
}

func TestApplyingASyncWritesTheFilesAndClearsUndo(t *testing.T) {
	m, hook := syncModel(t)
	m = selectTrack(t, m, "Flow") // first make something undoable
	m = keys(t, m, "right", "enter")
	m = typed(t, m, "!")
	m = keys(t, m, "enter", "left")

	m = keys(t, m, "s", "enter", "enter")
	contains(t, m, "created", "Done")
	if _, err := os.Stat(filepath.Join(hook.Library.Root(), "Shawn James", "Shadows", "07 - Flow!.mp3")); err != nil {
		t.Errorf("the file was not written: %v", err)
	}
	m = keys(t, m, "enter") // close
	m = keys(t, m, "u")
	contains(t, m, "Nothing to undo")
	lacks(t, m, "to sync")
}

func TestHeaderSaysHowManyTracksNeedASync(t *testing.T) {
	m, hook := syncModel(t)
	a := appOf(t, hook.Library)
	plan, err := a.PlanSync(app.SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	m = send(t, m, m.Init()) // look again now that there is a baseline
	lacks(t, m, "to sync")

	m = selectTrack(t, m, "Flow")
	m = keys(t, m, "right", "enter")
	m = typed(t, m, "!")
	m = keys(t, m, "enter")
	if first := lines(m)[0]; !strings.Contains(first, "1 to sync") {
		t.Errorf("header = %q", first)
	}
	m = keys(t, m, "u")
	if first := lines(m)[0]; strings.Contains(first, "to sync") {
		t.Errorf("undoing the edit should make it in sync again: %q", first)
	}
}

func TestWithoutAPriorSyncTheHeaderCountsWhatWouldBeWritten(t *testing.T) {
	m, _ := syncModel(t)
	if first := lines(m)[0]; !strings.Contains(first, "to sync") {
		t.Errorf("header = %q", first)
	}
}

func TestStraysAreOnlyReportedUntilPruneIsSwitchedOn(t *testing.T) {
	m, hook := syncModel(t)
	stray := filepath.Join(hook.Library.Root(), "Old", "x.mp3")
	if err := os.MkdirAll(filepath.Dir(stray), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(stray, []byte("old"), 0o644)

	m = keys(t, m, "s", "enter")
	contains(t, m, "1 stray", "!", "Old/x.mp3", "p also delete")
	m = keys(t, m, "p")
	contains(t, m, "1 to delete", "−", "will be deleted")

	m = keys(t, m, "enter") // asks to be told
	contains(t, m, "Type delete")
	m = typed(t, m, "yes")
	m = keys(t, m, "enter")
	contains(t, m, "Type delete")
	if _, err := os.Stat(stray); err != nil {
		t.Fatal("deleted without the right word")
	}

	m = keys(t, m, "ctrl+u")
	m = typed(t, m, "delete")
	m = keys(t, m, "enter")
	contains(t, m, "Done")
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Error("the stray was not deleted")
	}
}

func TestEscInTheDeleteQuestionGoesBackToThePlan(t *testing.T) {
	m, hook := syncModel(t)
	stray := filepath.Join(hook.Library.Root(), "x.mp3")
	os.WriteFile(stray, []byte("old"), 0o644)
	m = keys(t, m, "s", "enter", "p", "enter")
	contains(t, m, "Type delete")
	m = keys(t, m, "esc")
	contains(t, m, "to create")
	if _, err := os.Stat(stray); err != nil {
		t.Error("esc must not delete")
	}
}

func TestSyncToAnotherFolderLeavesTheLibraryFolderAlone(t *testing.T) {
	m, hook := syncModel(t)
	target := filepath.Join(t.TempDir(), "player")
	m = keys(t, m, "s", "down", "enter") // "Another folder…"
	contains(t, m, "Folder")
	m = typed(t, m, target)
	m = keys(t, m, "enter")
	contains(t, m, "to create")
	m = keys(t, m, "enter")
	contains(t, m, "Done")
	if _, err := os.Stat(filepath.Join(target, "Shawn James", "Shadows", "01 - The Wanderer.mp3")); err != nil {
		t.Errorf("not written to the target: %v", err)
	}
	if _, err := os.Stat(filepath.Join(hook.Library.Root(), "Shawn James")); !os.IsNotExist(err) {
		t.Error("the library folder was written to as well")
	}
	m = keys(t, m, "enter")
	if first := lines(m)[0]; !strings.Contains(first, "to sync") {
		t.Errorf("the library folder is still not synced: %q", first)
	}
}

func TestADetectedDriveCanBeChosen(t *testing.T) {
	m, _ := syncModel(t)
	drive := filepath.Join(t.TempDir(), "PLAYER")
	os.MkdirAll(drive, 0o755)
	m.mounts = func() []string { return []string{drive} }
	m = keys(t, m, "s", "down", "enter")
	contains(t, m, "to create")
	m = keys(t, m, "enter")
	contains(t, m, "Done")
	if _, err := os.Stat(filepath.Join(drive, "Phoria", "Caught a Black Rabbit", "04 - Current.mp3")); err != nil {
		t.Errorf("not on the drive: %v", err)
	}
}

func TestNothingToDo(t *testing.T) {
	m, hook := syncModel(t)
	a := appOf(t, hook.Library)
	plan, _ := a.PlanSync(app.SyncOptions{})
	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	m = keys(t, m, "s", "enter")
	contains(t, m, "Nothing to do", "up to date")
	m = keys(t, m, "enter")
	if m.overlay != nil {
		t.Error("enter should close it")
	}
}

func TestAPlanThatCannotBeMadeShowsWhy(t *testing.T) {
	m, _ := syncModel(t)
	file := filepath.Join(t.TempDir(), "a file")
	os.WriteFile(file, []byte("x"), 0o644)
	m = keys(t, m, "s", "down", "enter")
	m = typed(t, m, filepath.Join(file, "inside"))
	m = keys(t, m, "enter")
	contains(t, m, "directory")
	m = keys(t, m, "esc")
	if m.overlay != nil {
		t.Error("esc should close it")
	}
}

func TestSyncCanBeCancelledWhilePlanning(t *testing.T) {
	m, _ := syncModel(t)
	m = keys(t, m, "s")
	next, plan := m.Update(press("enter"))
	m = next.(Model)
	contains(t, m, "Preparing")
	m = keys(t, m, "esc")
	if m.overlay != nil {
		t.Fatal("esc should close the dialog")
	}
	m = send(t, m, plan) // the answer arrives late
	if m.overlay != nil {
		t.Error("a late plan must not reopen the dialog")
	}
}

func TestLongPlansScroll(t *testing.T) {
	a := testApp(t)
	var specs []seedTrack
	for i := 0; i < 30; i++ {
		specs = append(specs, seedTrack{"Song " + string(rune('A'+i%26)) + string(rune('a'+i/26)), "Band", "Album", i + 1, 30, 0})
	}
	seed(t, a, specs...)
	m := newTestModel(t, NewLibrary(a), 120, 30)
	m = keys(t, m, "s", "enter")
	contains(t, m, "30 to create")
	for i := 0; i < 28; i++ {
		m = keys(t, m, "down")
	}
	contains(t, m, "30 - ")
	lacks(t, m, "01 - Song")
}

func TestEditingInALibraryNeverSyncedStillSaysSomethingNeedsSyncing(t *testing.T) {
	m, _ := syncModel(t)
	m = selectTrack(t, m, "Flow")
	m = keys(t, m, "right", "enter")
	m = typed(t, m, "!")
	m = keys(t, m, "enter")
	if first := lines(m)[0]; !strings.Contains(first, "to sync") {
		t.Errorf("after an edit the header = %q", first)
	}
}
