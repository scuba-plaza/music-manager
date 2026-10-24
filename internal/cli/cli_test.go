package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/cli"
	"music-manager/internal/testmp3"
)

// run executes the CLI against a library, returning exit code, stdout, stderr.
func run(t *testing.T, lib string, args ...string) (int, string, string) {
	t.Helper()
	return runWith(t, cli.Deps{}, lib, args...)
}

func runWith(t *testing.T, deps cli.Deps, lib string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := cli.Run(context.Background(), append([]string{"--library", lib}, args...),
		cli.IO{In: strings.NewReader(""), Out: &out, Err: &errb}, deps)
	return code, out.String(), errb.String()
}

func writeMP3(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIngestThenList(t *testing.T) {
	lib, src := filepath.Join(t.TempDir(), "lib"), t.TempDir()
	writeMP3(t, filepath.Join(src, "a.mp3"), testmp3.WithTags(testmp3.Silent(100), "Firuz", "Fairuz", "Hits"))
	writeMP3(t, filepath.Join(src, "b.mp3"), testmp3.Silent(50))
	writeMP3(t, filepath.Join(src, "dup.mp3"), testmp3.Silent(50))

	code, out, errs := run(t, lib, "ingest", src)
	if code != 0 || !strings.Contains(out, "added 2") || !strings.Contains(out, "1 duplicate") {
		t.Errorf("ingest: code %d, out %q, err %q", code, out, errs)
	}

	code, out, errs = run(t, lib, "ls")
	if code != 0 {
		t.Fatalf("ls: code %d, err %q", code, errs)
	}
	for _, want := range []string{"ID", "STATUS", "unidentified", "Firuz", "Fairuz", "b", "0:02"} {
		if !strings.Contains(out, want) {
			t.Errorf("ls output lacks %q:\n%s", want, out)
		}
	}
}

// ingestOne ingests a single 100-frame track and returns its ID.
func ingestOne(t *testing.T, lib string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "song.mp3")
	writeMP3(t, f, testmp3.Silent(100))
	if code, _, errs := run(t, lib, "ingest", f); code != 0 {
		t.Fatalf("ingest failed: %s", errs)
	}
	_, out, _ := run(t, lib, "ls")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return strings.Fields(lines[1])[0]
}

func TestEditThenShow(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)

	code, _, errs := run(t, lib, "edit", id, "--title", "وطني", "--artist", "فيروز",
		"--album", "Hits", "--track", "3", "--of", "12", "--trim-start", "0:01", "--trim-end", "0.5")
	if code != 0 {
		t.Fatalf("edit: code %d, err %q", code, errs)
	}

	code, out, errs := run(t, lib, "show", id)
	if code != 0 {
		t.Fatalf("show: code %d, err %q", code, errs)
	}
	for _, want := range []string{id, "identified", "وطني", "فيروز", "Hits", "3/12", "0:01", "0:00.5"} {
		if !strings.Contains(out, want) {
			t.Errorf("show output lacks %q:\n%s", want, out)
		}
	}

	_, out, _ = run(t, lib, "ls", "firuz")
	if !strings.Contains(out, "وطني") {
		t.Errorf("ls firuz (the Latin form of the artist) did not find the edited track:\n%s", out)
	}
}

func TestEditRejectsBadInput(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)
	for name, args := range map[string][]string{
		"nothing to change": {"edit", id},
		"bad timecode":      {"edit", id, "--trim-start", "1:99"},
		"unknown id":        {"edit", "nope", "--title", "x"},
		"trims too long":    {"edit", id, "--trim-start", "3"},
		"show unknown id":   {"show", "nope"},
	} {
		if code, _, errs := run(t, lib, args...); code == 0 || !strings.HasPrefix(errs, "mm: ") {
			t.Errorf("%s: code %d, stderr %q; want failure with an 'mm:' message", name, code, errs)
		}
	}
}

// squash collapses runs of spaces, so tests do not depend on column padding.
func squash(s string) string {
	return strings.Join(strings.Split(regexp.MustCompile(` +`).ReplaceAllString(s, " "), "\n"), "\n")
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func TestSyncDryRunThenApply(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)
	run(t, lib, "edit", id, "--title", "وطني", "--artist", "فيروز", "--album", "Hits", "--track", "3", "--of", "12")
	file := filepath.Join(lib, "فيروز", "Hits", "03 - وطني.mp3")

	code, out, errs := run(t, lib, "sync", "--dry-run")
	if code != 0 || !strings.Contains(squash(out), "create فيروز/Hits/03 - وطني.mp3") || !strings.Contains(out, "dry run") {
		t.Errorf("dry run: code %d, out %q, err %q", code, out, errs)
	}
	if exists(file) {
		t.Error("dry run wrote a file")
	}

	code, out, errs = run(t, lib, "sync")
	if code != 0 || !strings.Contains(out, "1 created") || !exists(file) {
		t.Errorf("sync: code %d, out %q, err %q", code, out, errs)
	}
	if _, out, _ = run(t, lib, "sync"); !strings.Contains(out, "nothing to do") {
		t.Errorf("second sync said %q", out)
	}
}

func TestSyncPruneNeedsConfirmation(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)
	run(t, lib, "edit", id, "--title", "Song", "--artist", "Band")
	stray := filepath.Join(lib, "Old", "x.mp3")
	writeMP3(t, stray, testmp3.Silent(3))

	code, out, _ := run(t, lib, "sync")
	if code != 0 || !exists(stray) || !strings.Contains(squash(out), "stray Old/x.mp3") || !strings.Contains(out, "--prune") {
		t.Errorf("plain sync: code %d, out %q, stray exists %v", code, out, exists(stray))
	}

	code, out, errs := run(t, lib, "sync", "--prune")
	if code == 0 || !exists(stray) || !strings.Contains(errs, "--yes") || !strings.Contains(squash(out), "remove Old/x.mp3") {
		t.Errorf("prune without yes: code %d, out %q, err %q, stray exists %v", code, out, errs, exists(stray))
	}

	code, _, errs = run(t, lib, "sync", "--prune", "--yes")
	if code != 0 || exists(stray) || exists(filepath.Join(lib, "Old")) {
		t.Errorf("prune with yes: code %d, err %q, stray exists %v", code, errs, exists(stray))
	}
	if !exists(filepath.Join(lib, "Band", "Song.mp3")) {
		t.Error("the wanted track was lost")
	}
}

func TestSyncToATarget(t *testing.T) {
	lib, target := filepath.Join(t.TempDir(), "lib"), filepath.Join(t.TempDir(), "player")
	id := ingestOne(t, lib)
	run(t, lib, "edit", id, "--title", "Song", "--artist", "Band")
	if code, _, errs := run(t, lib, "sync", "--target", target); code != 0 || !exists(filepath.Join(target, "Band", "Song.mp3")) {
		t.Errorf("code %d, err %q", code, errs)
	}
}

func TestSyncWolfgangNeedsATarget(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)
	run(t, lib, "edit", id, "--title", "Song", "--artist", "Band")
	code, _, errs := run(t, lib, "sync", "--wolfgang")
	if code == 0 || !strings.Contains(errs, "--target") {
		t.Errorf("code %d, err %q: --wolfgang without --target should be refused and say why", code, errs)
	}
	if exists(filepath.Join(lib, "Band", "Song.mp3")) {
		t.Error("the library folder was written although the sync was refused")
	}
}

func TestSyncWolfgangSaysWhatItWritesFor(t *testing.T) {
	lib, target := filepath.Join(t.TempDir(), "lib"), filepath.Join(t.TempDir(), "player")
	id := ingestOne(t, lib)
	run(t, lib, "edit", id, "--title", "Song", "--artist", "Band")
	code, out, errs := run(t, lib, "sync", "--wolfgang", "--target", target, "--dry-run")
	if code != 0 || !strings.Contains(out, "Wolfgang WMP01") {
		t.Errorf("code %d, out %q, err %q", code, out, errs)
	}
	if code, out, errs = run(t, lib, "sync", "--wolfgang", "--target", target); code != 0 || !exists(filepath.Join(target, "Band", "Song.mp3")) {
		t.Errorf("code %d, out %q, err %q", code, out, errs)
	}
	if _, out, _ := run(t, lib, "sync", "--target", target, "--dry-run"); strings.Contains(out, "Wolfgang") {
		t.Errorf("a plain sync talks about the Wolfgang: %q", out)
	}
}

func TestEditCover(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)
	img := filepath.Join(t.TempDir(), "front.jpg")
	writeMP3(t, img, testmp3.JPEG)
	run(t, lib, "edit", id, "--title", "Song", "--artist", "Band", "--album", "Album")

	if code, _, errs := run(t, lib, "edit", id, "--cover", img); code != 0 {
		t.Fatalf("edit --cover: %s", errs)
	}
	run(t, lib, "sync")
	if !exists(filepath.Join(lib, "Band", "Album", "cover.jpg")) {
		t.Error("cover.jpg not written")
	}
	if code, _, _ := run(t, lib, "edit", id, "--cover", filepath.Join(t.TempDir(), "missing.jpg")); code == 0 {
		t.Error("a missing cover file was accepted")
	}
	if code, _, _ := run(t, lib, "edit", id, "--cover", ""); code != 0 {
		t.Error("clearing the cover failed")
	}
	if _, out, _ := run(t, lib, "sync", "--prune", "--yes"); !strings.Contains(squash(out), "remove Band/Album/cover.jpg") {
		t.Errorf("cover.jpg not pruned after clearing: %q", out)
	}
}

func TestRemoveThenSyncCollects(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)
	run(t, lib, "edit", id, "--title", "Song", "--artist", "Band")
	run(t, lib, "sync")

	if code, out, errs := run(t, lib, "rm", id, "nope"); code == 0 || !strings.Contains(errs, "nope") {
		t.Errorf("rm with an unknown id: code %d, out %q, err %q", code, out, errs)
	}
	if _, out, _ := run(t, lib, "ls"); !strings.Contains(out, id) {
		t.Error("a failed rm removed something")
	}

	code, out, errs := run(t, lib, "rm", id)
	if code != 0 || !strings.Contains(out, "removed "+id) {
		t.Errorf("rm: code %d, out %q, err %q", code, out, errs)
	}
	if _, out, _ := run(t, lib, "ls"); strings.Contains(out, id) {
		t.Error("track still listed")
	}
	_, out, _ = run(t, lib, "sync")
	if !strings.Contains(out, "1 unused file") {
		t.Errorf("sync did not report the collected blob:\n%s", out)
	}
	if !exists(filepath.Join(lib, "Band", "Song.mp3")) {
		t.Error("the rendered file must stay until --prune")
	}
}

func TestDoctor(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	ingestOne(t, lib)

	code, out, _ := run(t, lib, "doctor")
	if code != 0 || !strings.Contains(out, "warning") || !strings.Contains(out, "not identified") {
		t.Errorf("warnings only: code %d, out %q", code, out)
	}

	matches, _ := filepath.Glob(filepath.Join(lib, ".mm", "audio", "*.mp3"))
	if len(matches) != 1 {
		t.Fatalf("blobs: %v", matches)
	}
	os.Remove(matches[0])
	code, out, _ = run(t, lib, "doctor")
	if code == 0 || !strings.Contains(out, "error") || !strings.Contains(out, "missing") {
		t.Errorf("damage: code %d, out %q", code, out)
	}

	clean := filepath.Join(t.TempDir(), "empty")
	if code, out, _ := run(t, clean, "doctor"); code != 0 || !strings.Contains(out, "no problems") {
		t.Errorf("empty library: code %d, out %q", code, out)
	}
}

func TestVersionFlag(t *testing.T) {
	code, out, errs := run(t, filepath.Join(t.TempDir(), "lib"), "--version")
	if code != 0 || !strings.HasPrefix(out, "mm ") || strings.Contains(errs, "mm:") {
		t.Errorf("code %d, out %q, err %q", code, out, errs)
	}
}

// ingestFrames ingests a track of the given number of silent frames and
// returns its ID. Tracks of different lengths are different tracks.
func ingestFrames(t *testing.T, lib string, frames int) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "song.mp3")
	writeMP3(t, f, testmp3.Silent(frames))
	if code, _, errs := run(t, lib, "ingest", f); code != 0 {
		t.Fatalf("ingest failed: %s", errs)
	}
	_, out, _ := run(t, lib, "ls")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return strings.Fields(lines[len(lines)-1])[0]
}

func TestEditTheReleaseDateAndShowAllThreeDates(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)

	if code, _, errs := run(t, lib, "edit", id, "--released", "1999-05"); code != 0 {
		t.Fatalf("edit: code %d, err %q", code, errs)
	}
	_, out, _ := run(t, lib, "show", id)
	today := time.Now().Format("2006-01-02")
	for _, want := range []string{"Released 1999-05", "Added " + today, "Edited " + today} {
		if !strings.Contains(squash(out), want) {
			t.Errorf("show lacks %q:\n%s", want, out)
		}
	}

	run(t, lib, "edit", id, "--released", "")
	if _, out, _ := run(t, lib, "show", id); strings.Contains(out, "Released") {
		t.Errorf("a cleared date is still shown:\n%s", out)
	}

	code, _, errs := run(t, lib, "edit", id, "--released", "last summer")
	if code == 0 || !strings.Contains(errs, "1999-05-17") {
		t.Errorf("a bad date: code %d, err %q; want a failure that shows the format", code, errs)
	}
}

func TestAddTakesTheReleaseDateFromAFlag(t *testing.T) {
	dl := &fakeDownloader{info: app.Info{Title: "Song", Uploader: "Band - Topic"}}
	lib := filepath.Join(t.TempDir(), "lib")
	code, _, errs := runWith(t, cli.Deps{Downloader: dl}, lib, "add", "u1", "--no-input", "--released", "2001")
	if code != 0 {
		t.Fatalf("code %d, err %q", code, errs)
	}
	_, out, _ := run(t, lib, "ls")
	id := strings.Fields(strings.Split(strings.TrimSpace(out), "\n")[1])[0]
	if _, out, _ := run(t, lib, "show", id); !strings.Contains(squash(out), "Released 2001") {
		t.Errorf("show:\n%s", out)
	}
}

func TestListSortedByAnyField(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	long := ingestFrames(t, lib, 300)
	mid := ingestFrames(t, lib, 200)
	short := ingestFrames(t, lib, 100)
	run(t, lib, "edit", long, "--title", "Beta", "--artist", "Zed", "--album", "Second", "--released", "2001")
	run(t, lib, "edit", mid, "--title", "Gamma", "--artist", "Abe", "--album", "First", "--released", "1999-05")
	run(t, lib, "edit", short, "--title", "Alpha", "--artist", "Mo", "--released", "")

	titles := func(args ...string) string {
		t.Helper()
		code, out, errs := run(t, lib, append([]string{"ls"}, args...)...)
		if code != 0 {
			t.Fatalf("ls %v: code %d, err %q", args, code, errs)
		}
		var got []string
		for _, l := range strings.Split(strings.TrimSpace(out), "\n")[1:] {
			for _, name := range []string{"Alpha", "Beta", "Gamma"} {
				if strings.Contains(l, name) {
					got = append(got, name)
				}
			}
		}
		return strings.Join(got, " ")
	}
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"--sort", "title"}, "Alpha Beta Gamma"},
		{[]string{"--sort", "title", "--reverse"}, "Gamma Beta Alpha"},
		{[]string{"--sort", "artist"}, "Gamma Alpha Beta"},
		{[]string{"--sort", "length"}, "Alpha Gamma Beta"},
		{[]string{"--sort", "released"}, "Beta Gamma Alpha"}, // newest first; no date goes last
		{[]string{"--sort", "released", "--reverse"}, "Gamma Beta Alpha"},
		{[]string{"--sort", "added"}, "Alpha Gamma Beta"}, // the last one added comes first
		{[]string{"--sort", "TITLE", "beta"}, "Beta"},     // a query still filters; the key is not case sensitive
	} {
		if got := titles(c.args...); got != c.want {
			t.Errorf("ls %v = %s, want %s", c.args, got, c.want)
		}
	}

	if _, out, _ := run(t, lib, "ls", "--sort", "released"); !strings.Contains(squash(out), "RELEASED") || !strings.Contains(out, "1999-05") {
		t.Errorf("sorting by a date should show it:\n%s", out)
	}
	if _, out, _ := run(t, lib, "ls", "--sort", "title"); strings.Contains(out, "RELEASED") {
		t.Errorf("a date column without a date sort:\n%s", out)
	}
	code, _, errs := run(t, lib, "ls", "--sort", "colour")
	if code == 0 || !strings.Contains(errs, "artist") || !strings.Contains(errs, "released") {
		t.Errorf("unknown key: code %d, err %q; want the valid keys listed", code, errs)
	}
}

func TestEditTheSourceLink(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)

	if code, _, errs := run(t, lib, "edit", id, "--source-url", "https://www.youtube.com/watch?v=abc123"); code != 0 {
		t.Fatalf("edit: code %d, err %q", code, errs)
	}
	if _, out, _ := run(t, lib, "show", id); !strings.Contains(squash(out), "Source https://www.youtube.com/watch?v=abc123") {
		t.Errorf("show lacks the link:\n%s", out)
	}

	run(t, lib, "edit", id, "--source-url", "")
	if _, out, _ := run(t, lib, "show", id); strings.Contains(out, "Source") {
		t.Errorf("a cleared link is still shown:\n%s", out)
	}

	code, _, errs := run(t, lib, "edit", id, "--source-url", "youtube.com/watch?v=abc")
	if code == 0 || !strings.Contains(errs, "https://") {
		t.Errorf("a link without a scheme: code %d, err %q; want a failure that shows the form", code, errs)
	}

	// add takes its link as an argument, so it has no flag for it
	if code, _, _ := run(t, lib, "add", "u1", "--source-url", "https://x.org/a"); code == 0 {
		t.Error("add accepted --source-url")
	}
}

func TestEditTheGenreAndFindTracksByIt(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)

	if code, _, errs := run(t, lib, "edit", id, "--title", "Song", "--artist", "Band", "--genre", "Neoclassical"); code != 0 {
		t.Fatalf("edit: code %d, err %q", code, errs)
	}
	if _, out, _ := run(t, lib, "show", id); !strings.Contains(squash(out), "Genre Neoclassical") {
		t.Errorf("show lacks the genre:\n%s", out)
	}
	if _, out, _ := run(t, lib, "ls", "genre:neo"); !strings.Contains(out, id) {
		t.Errorf("ls genre:neo lacks the track:\n%s", out)
	}
	if _, out, _ := run(t, lib, "ls", "genre:folk"); strings.Contains(out, id) {
		t.Errorf("ls genre:folk lists the track:\n%s", out)
	}

	run(t, lib, "edit", id, "--genre", "")
	if _, out, _ := run(t, lib, "show", id); strings.Contains(out, "Genre") {
		t.Errorf("a cleared genre is still shown:\n%s", out)
	}
}

func TestAddTakesTheGenreFromAFlag(t *testing.T) {
	dl := &fakeDownloader{info: app.Info{Title: "Song", Uploader: "Band - Topic"}}
	lib := filepath.Join(t.TempDir(), "lib")
	code, _, errs := runWith(t, cli.Deps{Downloader: dl}, lib, "add", "u1", "--no-input", "--genre", "Folk")
	if code != 0 {
		t.Fatalf("code %d, err %q", code, errs)
	}
	_, out, _ := run(t, lib, "ls")
	id := strings.Fields(strings.Split(strings.TrimSpace(out), "\n")[1])[0]
	if _, out, _ := run(t, lib, "show", id); !strings.Contains(squash(out), "Genre Folk") {
		t.Errorf("show:\n%s", out)
	}
}

func TestShowSaysWhatTheWolfgangWillShowOnlyWhenAsked(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)
	if code, _, errs := run(t, lib, "edit", id, "--title", "Song", "--artist", "Abdel Halim Hafez", "--album", "Abd El Halim Hafez"); code != 0 {
		t.Fatalf("edit: code %d, err %q", code, errs)
	}
	_, out, _ := run(t, lib, "show", id)
	if strings.Contains(out, "On the Wolfgang") {
		t.Errorf("show talks about the Wolfgang unasked:\n%s", out)
	}
	_, out, _ = run(t, lib, "show", id, "--wolfgang")
	if !strings.Contains(squash(out), "On the Wolfgang A. H. Hafez · Abd El Halim") {
		t.Errorf("show --wolfgang lacks what the player will show:\n%s", out)
	}

	// The library keeps no names for the player: the flags are gone.
	for _, args := range [][]string{{"edit", id, "--player-artist", "X"}, {"edit", id, "--player-album", "X"}} {
		if code, _, errs := run(t, lib, args...); code == 0 || !strings.Contains(errs, "unknown flag") {
			t.Errorf("%v: code %d, stderr %q; want an unknown flag", args, code, errs)
		}
	}
}

func TestSetTheVolumeAndTheFadesFromTheCommandLine(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib) // 2.6 s

	if code, _, errs := run(t, lib, "edit", id, "--title", "Song", "--artist", "Band", "--gain", "+3dB", "--fade-in", "1", "--fade-out", "0:00.5"); code != 0 {
		t.Fatalf("edit: code %d, err %q", code, errs)
	}
	_, out, _ := run(t, lib, "show", id)
	s := squash(out)
	for _, want := range []string{"Volume +3.0 dB (2 steps)", "Fade in 0:01", "Fade out 0:00.5"} {
		if !strings.Contains(s, want) {
			t.Errorf("show lacks %q:\n%s", want, out)
		}
	}

	// A fade counts from the cuts, including cuts given in the same command.
	if code, _, errs := run(t, lib, "edit", id, "--trim-start", "0:01", "--fade-in", "1"); code != 0 {
		t.Fatalf("edit: code %d, err %q", code, errs)
	}
	if _, out, _ = run(t, lib, "show", id); !strings.Contains(squash(out), "Fade in 0:01") {
		t.Errorf("a fade-in from the new cut:\n%s", out)
	}
	if _, out, _ = run(t, lib, "show", id); !strings.Contains(squash(out), "Fade out 0:00.5") {
		t.Errorf("the other fade was lost:\n%s", out)
	}

	// Each can be taken away again.
	if code, _, errs := run(t, lib, "edit", id, "--gain", "0", "--fade-in", "", "--fade-out", "0"); code != 0 {
		t.Fatalf("edit: code %d, err %q", code, errs)
	}
	if _, out, _ = run(t, lib, "show", id); strings.Contains(out, "Volume") || strings.Contains(out, "Fade") {
		t.Errorf("loudness is still shown:\n%s", out)
	}
}

func TestVolumeAndFadeFlagsSayWhatIsWrong(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)
	for name, c := range map[string]struct {
		args []string
		want string
	}{
		"not a number":     {[]string{"--gain", "loud"}, "loud"},
		"too much":         {[]string{"--gain", "40"}, "24 dB"},
		"not a time":       {[]string{"--fade-in", "soon"}, "soon"},
		"past the track":   {[]string{"--fade-out", "1:00"}, "fade-out"},
		"fades that cross": {[]string{"--fade-in", "2", "--fade-out", "2"}, "must not start before"},
	} {
		code, _, errs := run(t, lib, append([]string{"edit", id}, c.args...)...)
		if code == 0 || !strings.Contains(errs, c.want) {
			t.Errorf("%s: code %d, err %q; want one that says %q", name, code, errs, c.want)
		}
	}
	if _, out, _ := run(t, lib, "show", id); strings.Contains(out, "Volume") || strings.Contains(out, "Fade") {
		t.Errorf("a refused edit changed the track:\n%s", out)
	}
}

func TestSyncWolfgangListsWhatThePlayerWillNotShowWell(t *testing.T) {
	lib, target := filepath.Join(t.TempDir(), "lib"), filepath.Join(t.TempDir(), "player")
	id := ingestOne(t, lib)
	run(t, lib, "edit", id, "--title", "Song", "--artist", "Band", "--genre", "Alternative Rock")
	_, out, _ := run(t, lib, "sync", "--wolfgang", "--target", target, "--dry-run")
	if !strings.Contains(out, "warning") || !strings.Contains(out, id) || !strings.Contains(out, "Alternative Rock") {
		t.Errorf("the long genre is not reported:\n%s", out)
	}
	if _, out, _ := run(t, lib, "sync", "--target", target, "--dry-run"); strings.Contains(out, "warning") {
		t.Errorf("a plain sync warns about the player:\n%s", out)
	}
}

func TestShowWolfgangSaysWhatTheWolfgangWillNotShowWell(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)
	run(t, lib, "edit", id, "--title", "Song", "--artist", "Band", "--genre", "Alternative Rock")
	if _, out, _ := run(t, lib, "show", id, "--wolfgang"); !strings.Contains(squash(out), "Warning the genre") {
		t.Errorf("show --wolfgang has no warning:\n%s", out)
	}
	if _, out, _ := run(t, lib, "show", id); strings.Contains(out, "Warning") {
		t.Errorf("show warns without --wolfgang:\n%s", out)
	}
}

func TestEditSetsTheLatinFormsAndShowSaysSoOnlyWhereTheyAreSet(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)
	code, _, errs := run(t, lib, "edit", id, "--title", "وطني", "--artist", "فيروز", "--album", "روائع", "--genre", "طرب",
		"--title-latin", "Watani", "--artist-latin", "Fairouz", "--album-latin", "Rawa'e", "--genre-latin", "Tarab")
	if code != 0 {
		t.Fatalf("code %d, err %q", code, errs)
	}
	_, out, _ := run(t, lib, "show", id)
	for _, want := range []string{"Latin title Watani", "Latin artist Fairouz", "Latin album Rawa'e", "Latin genre Tarab"} {
		if !strings.Contains(squash(out), want) {
			t.Errorf("show lacks %q:\n%s", want, out)
		}
	}
	if code, _, errs := run(t, lib, "edit", id, "--artist-latin", ""); code != 0 {
		t.Fatalf("clearing: code %d, err %q", code, errs)
	}
	_, out, _ = run(t, lib, "show", id)
	if strings.Contains(out, "Latin artist") || !strings.Contains(out, "Latin title") {
		t.Errorf("after clearing the artist's form:\n%s", out)
	}
}

func TestShowWolfgangUsesTheLatinFormsSetByHand(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)
	run(t, lib, "edit", id, "--title", "Bahlam Bek", "--artist", "عبد الحليم حافظ", "--album", "روائع",
		"--artist-latin", "Abdel Halim Hafez", "--album-latin", "Rawaa'e")
	_, out, _ := run(t, lib, "show", id, "--wolfgang")
	if !strings.Contains(squash(out), "On the Wolfgang A. H. Hafez · Rawaa'e") {
		t.Errorf("show --wolfgang:\n%s", out)
	}
}

func TestEditRefusesALatinFormOnSeveralLines(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)
	if code, _, errs := run(t, lib, "edit", id, "--title-latin", "a\nb"); code == 0 || !strings.Contains(errs, "one line") {
		t.Errorf("code %d, err %q", code, errs)
	}
}

func TestRemoveWithARepeatedIDRemovesNothing(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)

	code, out, errs := run(t, lib, "rm", id, id)
	if code == 0 || !strings.Contains(errs, id) {
		t.Errorf("rm with a repeated id: code %d, out %q, err %q", code, out, errs)
	}
	if strings.Contains(out, "removed") {
		t.Errorf("rm said it removed something and then failed: %q", out)
	}
	if _, out, _ := run(t, lib, "ls"); !strings.Contains(out, id) {
		t.Error("a failed rm removed the track")
	}
}

func TestEditWithACoverThatIsNotAnImageChangesNothing(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)
	notAnImage := filepath.Join(t.TempDir(), "front.txt")
	writeMP3(t, notAnImage, []byte("not a picture"))

	code, _, errs := run(t, lib, "edit", id, "--title", "Song", "--artist", "Band", "--cover", notAnImage)
	if code == 0 || !strings.Contains(errs, "JPEG or PNG") {
		t.Fatalf("code %d, err %q", code, errs)
	}
	if _, out, _ := run(t, lib, "show", id); strings.Contains(out, "Song") || strings.Contains(out, "Band") {
		t.Errorf("the edit was half applied:\n%s", out)
	}
}

func TestOnlyTheRightSpellingOfWolfgangIsAFlag(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	for _, args := range [][]string{{"sync", "--wolfang"}, {"web", "--wolfang"}, {"--wolfang"}, {"show", "x", "--wolfang"}} {
		code, _, errs := run(t, lib, args...)
		if code == 0 || !strings.Contains(errs, "unknown flag: --wolfang") {
			t.Errorf("mm %s: code %d, err %q; the misspelling is not a flag", strings.Join(args, " "), code, errs)
		}
	}
}
