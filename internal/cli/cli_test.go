package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

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
	for _, want := range []string{id, "identified", "Wtni", "وطني", "Firuz", "فيروز", "Hits", "3/12", "0:01", "0:00.5"} {
		if !strings.Contains(out, want) {
			t.Errorf("show output lacks %q:\n%s", want, out)
		}
	}

	_, out, _ = run(t, lib, "ls", "firuz")
	if !strings.Contains(out, "Wtni") {
		t.Errorf("ls firuz did not find the edited track:\n%s", out)
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
	file := filepath.Join(lib, "Firuz", "Hits", "03 - Wtni.mp3")

	code, out, errs := run(t, lib, "sync", "--dry-run")
	if code != 0 || !strings.Contains(squash(out), "create Firuz/Hits/03 - Wtni.mp3") || !strings.Contains(out, "dry run") {
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
