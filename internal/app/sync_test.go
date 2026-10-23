package app_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/mp3"
	"music-manager/internal/render"
	"music-manager/internal/testmp3"
)

// libWithTracks ingests one file per entry of frames (a track of that many
// frames each) and returns the library with the tracks in ingestion order.
func libWithTracks(t *testing.T, frames ...int) *app.App {
	t.Helper()
	a := openLib(t)
	src := t.TempDir()
	for i, n := range frames {
		writeFile(t, filepath.Join(src, string(rune('a'+i))+".mp3"), testmp3.Silent(n))
	}
	if _, err := a.Ingest([]string{src}); err != nil {
		t.Fatal(err)
	}
	return a
}

func mustSync(t *testing.T, a *app.App, opts app.SyncOptions) []render.Op {
	t.Helper()
	plan := mustPlan(t, a, opts)
	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	return plan.Ops
}

func mustPlan(t *testing.T, a *app.App, opts app.SyncOptions) *app.SyncPlan {
	t.Helper()
	plan, err := a.PlanSync(opts)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func identify(t *testing.T, a *app.App, id, title, artist, album string, n, total int) {
	t.Helper()
	e := app.Edit{Title: &title, Artist: &artist, Album: &album, TrackNumber: &n, TrackTotal: &total}
	if _, err := a.Edit(id, e); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, root, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestSyncRendersTheTreeFromTheManifest(t *testing.T) {
	a := libWithTracks(t, 100, 60, 30)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "وطني", "فيروز", "Hits", 3, 12)
	identify(t, a, ts[1].ID, "Dream", "Max Richter", "", 0, 0)
	// ts[2] stays unidentified

	ops := mustSync(t, a, app.SyncOptions{})
	if len(ops) != 3 {
		t.Fatalf("ops = %v, want 3 creates", ops)
	}

	file := readFile(t, a.Root(), "Firuz/Hits/03 - Wtni.mp3")
	got := mp3.ReadTagSet(file)
	if got.Title != "Wtni" || got.Artist != "Firuz" || got.Album != "Hits" || got.Track != 3 || got.Total != 12 {
		t.Errorf("tags = %+v", got)
	}
	if got.Extra["ORIGINAL_TITLE"] != "وطني" || got.Extra["ORIGINAL_ARTIST"] != "فيروز" || got.Extra["MM_ID"] != ts[0].ID {
		t.Errorf("extra tags = %v", got.Extra)
	}
	audio, _, _ := mp3.Parse(file)
	if want := testmp3.Silent(100); !bytes.Equal(audio, want) {
		t.Errorf("audio changed: %d bytes, want %d", len(audio), len(want))
	}

	readFile(t, a.Root(), "Max Richter/Dream.mp3")
	inbox, _ := filepath.Glob(filepath.Join(a.Root(), "_Inbox", ts[2].ID+" - *.mp3"))
	if len(inbox) != 1 {
		t.Errorf("unidentified track not in _Inbox: %v", inbox)
	}

	if ops := mustSync(t, a, app.SyncOptions{}); len(ops) != 0 {
		t.Errorf("second sync wants %v, want nothing", ops)
	}
}

func TestSyncAppliesTrimsWithoutTouchingTheStoredAudio(t *testing.T) {
	a := libWithTracks(t, 100)
	tr := tracks(t, a)[0]
	identify(t, a, tr.ID, "Song", "Band", "", 0, 0)
	mustSync(t, a, app.SyncOptions{})
	full := readFile(t, a.Root(), "Band/Song.mp3")

	start, end := time.Second, 500*time.Millisecond
	if _, err := a.Edit(tr.ID, app.Edit{TrimStart: &start, TrimEnd: &end}); err != nil {
		t.Fatal(err)
	}
	ops := mustSync(t, a, app.SyncOptions{})
	if len(ops) != 1 || ops[0].Kind != render.Update {
		t.Fatalf("ops = %v, want one update", ops)
	}

	trimmed := readFile(t, a.Root(), "Band/Song.mp3")
	audio, _, _ := mp3.Parse(trimmed)
	want, _ := mp3.Trim(testmp3.Silent(100), start, end)
	if !bytes.Equal(audio, want) || len(trimmed) >= len(full) {
		t.Errorf("rendered audio is %d bytes, want the %d trimmed bytes", len(audio), len(want))
	}
	if got := mp3.ReadTagSet(trimmed).Length; got != 1112*time.Millisecond {
		t.Errorf("TLEN = %v, want the 1.112s playing length", got)
	}
	stored, _ := os.ReadFile(a.AudioPath(tr.AudioSHA256))
	if !bytes.Equal(stored, testmp3.Silent(100)) {
		t.Error("the stored audio was altered by trimming")
	}

	// Removing the trims restores the original audio.
	zero := time.Duration(0)
	a.Edit(tr.ID, app.Edit{TrimStart: &zero, TrimEnd: &zero})
	mustSync(t, a, app.SyncOptions{})
	if !bytes.Equal(readFile(t, a.Root(), "Band/Song.mp3"), full) {
		t.Error("untrimming did not restore the file")
	}
}

func TestSyncMovesRenamedTracksAndPrunesTheOldCopy(t *testing.T) {
	a := libWithTracks(t, 10)
	tr := tracks(t, a)[0]
	identify(t, a, tr.ID, "Old Name", "Band", "Album", 1, 1)
	mustSync(t, a, app.SyncOptions{})

	identify(t, a, tr.ID, "New Name", "Band", "Album", 1, 1)
	ops := mustSync(t, a, app.SyncOptions{})
	want := []render.Op{{Kind: render.Create, Path: "Band/Album/01 - New Name.mp3"}, {Kind: render.Stray, Path: "Band/Album/01 - Old Name.mp3"}}
	if len(ops) != 2 || ops[0] != want[0] || ops[1] != want[1] {
		t.Fatalf("ops = %v, want %v", ops, want)
	}
	readFile(t, a.Root(), "Band/Album/01 - Old Name.mp3") // still there without prune

	ops = mustSync(t, a, app.SyncOptions{Prune: true})
	if len(ops) != 1 || ops[0].Kind != render.Remove {
		t.Fatalf("ops = %v, want one removal", ops)
	}
	if _, err := os.Stat(filepath.Join(a.Root(), "Band/Album/01 - Old Name.mp3")); !os.IsNotExist(err) {
		t.Errorf("old copy not removed: %v", err)
	}
}

func TestSyncPruneRemovesEmptyDirsButLeavesOtherFilesAndInternals(t *testing.T) {
	a := libWithTracks(t, 10)
	tr := tracks(t, a)[0]
	identify(t, a, tr.ID, "Song", "Band", "", 0, 0)
	writeFile(t, filepath.Join(a.Root(), "Old Tree", "Deep", "x.mp3"), testmp3.Silent(3))
	writeFile(t, filepath.Join(a.Root(), "Old Tree", "Deep", "cover.jpg"), []byte{0xFF, 0xD8})
	writeFile(t, filepath.Join(a.Root(), "Notes", "todo.txt"), []byte("keep me"))
	writeFile(t, filepath.Join(a.Root(), "Mixed", "keep.txt"), []byte("keep me too"))
	writeFile(t, filepath.Join(a.Root(), "Mixed", "gone.mp3"), testmp3.Silent(3))

	ops := mustSync(t, a, app.SyncOptions{Prune: true})
	var removed []string
	for _, op := range ops {
		if op.Kind == render.Remove {
			removed = append(removed, op.Path)
		}
	}
	wantRemoved := []string{"Mixed/gone.mp3", "Old Tree/Deep/cover.jpg", "Old Tree/Deep/x.mp3"}
	if len(removed) != 3 || removed[0] != wantRemoved[0] || removed[1] != wantRemoved[1] || removed[2] != wantRemoved[2] {
		t.Errorf("removed %v, want %v", removed, wantRemoved)
	}
	for _, gone := range []string{"Old Tree"} {
		if _, err := os.Stat(filepath.Join(a.Root(), gone)); !os.IsNotExist(err) {
			t.Errorf("%s should be gone", gone)
		}
	}
	for _, kept := range []string{"Notes/todo.txt", "Mixed/keep.txt", "library.json", ".mm/audio", "Band/Song.mp3"} {
		if _, err := os.Stat(filepath.Join(a.Root(), kept)); err != nil {
			t.Errorf("%s was lost: %v", kept, err)
		}
	}
}

func TestPlanSyncWritesNothing(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	for _, tr := range tracks(t, a) {
		identify(t, a, tr.ID, "Song "+tr.ID, "Band", "", 0, 0)
	}
	before := listTree(t, a.Root())
	plan := mustPlan(t, a, app.SyncOptions{Prune: true})
	if len(plan.Ops) != 2 {
		t.Errorf("ops = %v", plan.Ops)
	}
	if after := listTree(t, a.Root()); !equalStrings(before, after) {
		t.Errorf("planning changed the tree:\nbefore %v\nafter  %v", before, after)
	}
}

func TestSyncToAnotherTargetLeavesTheLibraryRootAlone(t *testing.T) {
	a := libWithTracks(t, 10)
	identify(t, a, tracks(t, a)[0].ID, "Song", "Band", "", 0, 0)
	target := filepath.Join(t.TempDir(), "player")

	mustSync(t, a, app.SyncOptions{Target: target})
	readFile(t, target, "Band/Song.mp3")
	if _, err := os.Stat(filepath.Join(a.Root(), "Band")); !os.IsNotExist(err) {
		t.Error("rendering to a target also wrote into the library root")
	}
	if _, err := os.Stat(filepath.Join(target, ".mm")); !os.IsNotExist(err) {
		t.Error("internal state leaked into the target")
	}
	if ops := mustSync(t, a, app.SyncOptions{Target: target}); len(ops) != 0 {
		t.Errorf("second sync to the target wants %v", ops)
	}
}

func TestSyncGivesCollidingTracksDistinctFiles(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	for _, tr := range tracks(t, a) {
		identify(t, a, tr.ID, "Same", "Band", "Album", 1, 1)
	}
	mustSync(t, a, app.SyncOptions{})
	readFile(t, a.Root(), "Band/Album/01 - Same.mp3")
	readFile(t, a.Root(), "Band/Album/01 - Same (2).mp3")
}

func listTree(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			info, _ := d.Info()
			out = append(out, p+":"+info.ModTime().String()+":"+string(rune(info.Size())))
		}
		return nil
	})
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSyncWritesTheReleaseDateIntoTheTag(t *testing.T) {
	a := libWithTracks(t, 40, 41)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Dated", "Band", "", 0, 0)
	identify(t, a, ts[1].ID, "Undated", "Band", "", 0, 0)
	if _, err := a.Edit(ts[0].ID, app.Edit{Released: ptr("1999-05")}); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, app.SyncOptions{})
	if got := mp3.ReadTagSet(readFile(t, a.Root(), "Band/Dated.mp3")).Date; got != "1999-05" {
		t.Errorf("date in the tag = %q", got)
	}
	if got := mp3.ReadTagSet(readFile(t, a.Root(), "Band/Undated.mp3")).Date; got != "" {
		t.Errorf("a track with no date got %q", got)
	}
}

func TestChangingTheReleaseDateNeedsASyncButEditingTheClockDoesNot(t *testing.T) {
	a := libWithTracks(t, 40)
	tr := tracks(t, a)[0]
	identify(t, a, tr.ID, "Song", "Band", "", 0, 0)
	mustSync(t, a, app.SyncOptions{})

	if _, err := a.Edit(tr.ID, app.Edit{Released: ptr("")}); err != nil { // no change, but a save
		t.Fatal(err)
	}
	if n, _ := unsynced(t, a); n != 0 {
		t.Errorf("saving without a change: %d to sync", n)
	}
	if _, err := a.Edit(tr.ID, app.Edit{Released: ptr("1975")}); err != nil {
		t.Fatal(err)
	}
	if n, _ := unsynced(t, a); n != 1 {
		t.Errorf("after setting a date: %d to sync, want 1", n)
	}
	if ops := mustSync(t, a, app.SyncOptions{}); len(ops) != 1 {
		t.Errorf("the date should rewrite the file: %v", ops)
	}
}

func TestSyncWritesTheGenreIntoTheTagAndChangingItNeedsASync(t *testing.T) {
	a := libWithTracks(t, 40, 41)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Sorted", "Band", "", 0, 0)
	identify(t, a, ts[1].ID, "Unsorted", "Band", "", 0, 0)
	if _, err := a.Edit(ts[0].ID, app.Edit{Genre: ptr("Neoclassical")}); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, app.SyncOptions{})
	if got := mp3.ReadTagSet(readFile(t, a.Root(), "Band/Sorted.mp3")).Genre; got != "Neoclassical" {
		t.Errorf("genre in the tag = %q", got)
	}
	if got := mp3.ReadTagSet(readFile(t, a.Root(), "Band/Unsorted.mp3")).Genre; got != "" {
		t.Errorf("a track with no genre got %q", got)
	}

	if _, err := a.Edit(ts[0].ID, app.Edit{Genre: ptr("Folk")}); err != nil {
		t.Fatal(err)
	}
	if n, _ := unsynced(t, a); n != 1 {
		t.Errorf("after changing a genre: %d to sync, want 1", n)
	}
	if ops := mustSync(t, a, app.SyncOptions{}); len(ops) != 1 {
		t.Errorf("the genre should rewrite the file: %v", ops)
	}
}
