package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/device"
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
	t.Setenv("HOME", "/home/me")
	m, _ := syncModel(t)
	m.lib = atMusic{m.lib} // a folder with a short name: a long one is cut off
	m.mounts = func() []string { return []string{"/run/media/tau/PLAYER"} }
	m = keys(t, m, "s")
	contains(t, m, "Sync", "Library folder", "~/Music/mp3-player", "/run/media/tau/PLAYER", "Another folder")
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

func TestTheFooterOffersSyncAndTheHeaderSaysWhenSomethingIsWaiting(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {80, 24}} {
		m, _ := browserModel(t, size[0], size[1])
		if footer := lines(m)[size[1]-1]; !strings.Contains(footer, "s sync") {
			t.Errorf("%dx%d: nothing in the footer offers a sync: %q", size[0], size[1], footer)
		}
		if header := lines(m)[0]; !strings.Contains(header, "6 to sync · press s") {
			t.Errorf("%dx%d: the header does not say how to sync: %q", size[0], size[1], header)
		}
	}
}

func TestClickingWhatTheHeaderSaysIsWaitingOpensTheSyncScreen(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	header := []rune(lines(m)[0])
	at := strings.Index(string(header), "to sync")
	x := len([]rune(string(header)[:at])) // column of the chip
	m = send(t, m, click(x, 0))
	if _, ok := m.overlay.(*syncDialog); !ok {
		t.Fatalf("a click on the sync note in the header opened %T", m.overlay)
	}

	m, _ = browserModel(t, 120, 36)
	m = send(t, m, click(10, 0)) // the library's name
	if m.overlay != nil {
		t.Errorf("a click elsewhere in the header opened %T", m.overlay)
	}
}

func TestTheHeaderStaysQuietWhenNothingNeedsSyncing(t *testing.T) {
	m, hook := syncModel(t)
	a := appOf(t, hook.Library)
	plan, err := a.PlanSync(app.SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	m = send(t, m, m.Init())
	lacks(t, m, "press s")
}

// planSpy remembers the options of every plan the dialog asked for.
type planSpy struct {
	Library
	opts []app.SyncOptions
}

func (s *planSpy) PlanSync(o app.SyncOptions) (Plan, error) {
	s.opts = append(s.opts, o)
	return s.Library.PlanSync(o)
}

func driveModel(t *testing.T) (Model, *planSpy) {
	t.Helper()
	m, _ := syncModel(t)
	spy := &planSpy{Library: m.lib}
	m.lib = spy
	drive := filepath.Join(t.TempDir(), "PLAYER")
	os.MkdirAll(drive, 0o755)
	m.mounts = func() []string { return []string{drive} }
	return m, spy
}

func TestTheChooserOffersAFormatForADriveButNotForTheLibraryFolder(t *testing.T) {
	m, _ := driveModel(t)
	m = keys(t, m, "s")
	contains(t, m, "Format", "Generic")
	lacks(t, m, "Wolfgang")
	m = keys(t, m, "w") // on the library folder: nothing to choose
	lacks(t, m, "Wolfgang WMP01")
	m = keys(t, m, "down", "w")
	contains(t, m, "Format", "Wolfgang WMP01")
	m = keys(t, m, "w")
	lacks(t, m, "Wolfgang WMP01")
	contains(t, m, "Generic")
}

func TestAPlanForADriveAsksForTheFormatThatWasChosen(t *testing.T) {
	m, spy := driveModel(t)
	m = keys(t, m, "s", "down", "enter") // generic by default
	contains(t, m, "to create")
	lacks(t, m, "Wolfgang")
	if got := spy.opts[len(spy.opts)-1].Device; got != nil && got != device.Generic {
		t.Errorf("device = %v, want the generic one", got)
	}
	m = keys(t, m, "esc", "s", "down", "w", "enter")
	contains(t, m, "to create", "Wolfgang WMP01")
	if got := spy.opts[len(spy.opts)-1].Device; got != device.Wolfgang {
		t.Errorf("device = %v, want the Wolfgang one", got)
	}
}

func TestTheFormatIsNotRememberedBetweenSyncs(t *testing.T) {
	m, _ := driveModel(t)
	m = keys(t, m, "s", "down", "w", "esc", "s", "down")
	contains(t, m, "Format", "Generic")
	lacks(t, m, "Wolfgang WMP01")
}

func TestTheLibraryFolderIsAlwaysPlannedTheStandardWay(t *testing.T) {
	m, spy := driveModel(t)
	m = keys(t, m, "s", "down", "w", "up", "enter")
	contains(t, m, "to create")
	if got := spy.opts[len(spy.opts)-1].Device; got != nil && got != device.Generic {
		t.Errorf("the library folder was planned for %v", got)
	}
}

func TestTheReviewOfAWolfgangPlanListsWhatThePlayerWillNotShowWell(t *testing.T) {
	m, _ := driveModel(t)
	a := appOf(t, m.lib.(*planSpy).Library)
	for _, tr := range libTracks(t, m.lib) {
		if tr.Title == "Current" {
			if _, err := a.Edit(tr.ID, app.Edit{Genre: ptr("Alternative Rock")}); err != nil {
				t.Fatal(err)
			}
		}
	}
	m = keys(t, m, "s", "down", "enter") // the standard way: nothing to say
	contains(t, m, "to create")
	lacks(t, m, "Alternative Rock")
	m = keys(t, m, "esc", "s", "down", "w", "enter")
	contains(t, m, "to create", "Wolfgang WMP01", "Alternative Rock", "12")
}
