package app_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
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
	if err := plan.ApplyWith(nil); err != nil {
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

	file := readFile(t, a.Root(), "فيروز/Hits/03 - وطني.mp3")
	got := mp3.ReadTagSet(file)
	if got.Title != "وطني" || got.Artist != "فيروز" || got.Album != "Hits" || got.Track != 3 || got.Total != 12 {
		t.Errorf("tags = %+v", got)
	}
	if got.Extra["MM_ID"] != ts[0].ID {
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
	if after := listTree(t, a.Root()); !slices.Equal(before, after) {
		t.Errorf("planning changed the tree:\nbefore %v\nafter  %v", before, after)
	}
}

func TestSyncLeavesEmptyDirectoriesItDidNotEmpty(t *testing.T) {
	a := libWithTracks(t, 10)
	identify(t, a, tracks(t, a)[0].ID, "Song", "Band", "", 0, 0)
	for _, dir := range []string{"RECORD", "Voice/Old"} {
		if err := os.MkdirAll(filepath.Join(a.Root(), filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(a.Root(), "Old Tree", "x.mp3"), testmp3.Silent(3))

	mustSync(t, a, app.SyncOptions{Prune: true})

	for _, dir := range []string{"RECORD", "Voice/Old"} {
		if _, err := os.Stat(filepath.Join(a.Root(), filepath.FromSlash(dir))); err != nil {
			t.Errorf("%s was removed although the sync never emptied it: %v", dir, err)
		}
	}
	if _, err := os.Stat(filepath.Join(a.Root(), "Old Tree")); !os.IsNotExist(err) {
		t.Error("Old Tree was emptied by the sync and should be gone")
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

func TestChangingWhatTheFileSaysNeedsASyncButSavingItUnchangedDoesNot(t *testing.T) {
	for name, change := range map[string]app.Edit{
		"release date": {Released: ptr("1975")},
		"genre":        {Genre: ptr("Folk")},
		"volume":       {GainDB: ptr(-3.0)},
	} {
		t.Run(name, func(t *testing.T) {
			a, tr := loudTrack(t)
			mustSync(t, a, app.SyncOptions{})

			if _, err := a.Edit(tr.ID, app.Edit{}); err != nil {
				t.Fatal(err)
			}
			if n, _ := unsynced(t, a); n != 0 {
				t.Errorf("saving without a change: %d to sync", n)
			}
			if _, err := a.Edit(tr.ID, change); err != nil {
				t.Fatal(err)
			}
			if n, _ := unsynced(t, a); n != 1 {
				t.Errorf("after the change: %d to sync, want 1", n)
			}
			if ops := mustSync(t, a, app.SyncOptions{}); len(ops) != 1 {
				t.Errorf("the change should rewrite the file: %v", ops)
			}
		})
	}
}

func TestSyncDoesNotCollectTheFilesOfATrackAddedAfterThePlanWasMade(t *testing.T) {
	a := libWithTracks(t, 40)
	plan := mustPlan(t, a, app.SyncOptions{}) // the review screen stays open while...

	other, err := app.Open(a.Root()) // ...another mm adds a track
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "new.mp3")
	writeFile(t, f, testmp3.WithCover(testmp3.Silent(25), "image/jpeg", testmp3.JPEG))
	if _, err := other.Ingest([]string{f}); err != nil {
		t.Fatal(err)
	}
	added := tracks(t, a)[1]
	if added.CoverSHA256 == "" {
		t.Fatal("the added track has no cover to lose")
	}

	if err := plan.ApplyWith(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.AudioPath(added.AudioSHA256)); err != nil {
		t.Errorf("the audio of the track added meanwhile was collected: %v", err)
	}
	if _, err := os.Stat(a.CoverPath(added.CoverSHA256)); err != nil {
		t.Errorf("the cover of the track added meanwhile was collected: %v", err)
	}
	issues, err := a.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range issues {
		if p.Severity == app.Error {
			t.Errorf("doctor: %s: %s", p.TrackID, p.Message)
		}
	}
}
