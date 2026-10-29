package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"music-manager/internal/cli"
	"music-manager/internal/testmp3"
)

// runAt runs mm for someone whose home is home and who names no library, so the
// default one is used.
func runAt(t *testing.T, home string, args ...string) (code int, out, errs string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("MM_LIBRARY", "")
	var o, e bytes.Buffer
	code = cli.Run(context.Background(), args, cli.IO{In: strings.NewReader(""), Out: &o, Err: &e}, cli.Deps{})
	return code, o.String(), e.String()
}

// The folder used to be ~/Music/mp3-player, from when it only fed a player. It is
// what the desktop player plays now.
func TestTheDefaultLibraryIsMusicLibrary(t *testing.T) {
	home, src := t.TempDir(), t.TempDir()
	writeMP3(t, filepath.Join(src, "a.mp3"), testmp3.Silent(10))
	if code, _, errs := runAt(t, home, "ingest", src); code != 0 {
		t.Fatalf("code %d, err %q", code, errs)
	}
	if !exists(filepath.Join(home, "Music", "Library", "library.json")) {
		t.Error("no library in ~/Music/Library")
	}
	if exists(filepath.Join(home, "Music", "mp3-player")) {
		t.Error("the old folder name came back")
	}
}

// mm makes the folder of a library on any command. Run before the move, it would
// leave an empty ~/Music/Library, and `mv mp3-player Library` would then put the
// library inside it. So until the folder is moved, mm says so and makes nothing.
func TestAnOldLibraryIsNotLeftBehindByTheNewDefault(t *testing.T) {
	home := t.TempDir()
	old := filepath.Join(home, "Music", "mp3-player")
	writeMP3(t, filepath.Join(old, "library.json"), []byte(`{"version":2,"tracks":[]}`))

	code, _, errs := runAt(t, home, "ls")
	if code == 0 {
		t.Fatal("mm went on with a library that is not where it looks")
	}
	for _, want := range []string{"mv " + old + " " + filepath.Join(home, "Music", "Library"), "--library"} {
		if !strings.Contains(errs, want) {
			t.Errorf("the message lacks %q:\n%s", want, errs)
		}
	}
	if exists(filepath.Join(home, "Music", "Library")) {
		t.Error("an empty ~/Music/Library was made, and mv would now put the library inside it")
	}
}

func TestTheOldLibraryDoesNotGetInTheWayOfAChoice(t *testing.T) {
	home := t.TempDir()
	old := filepath.Join(home, "Music", "mp3-player")
	writeMP3(t, filepath.Join(old, "library.json"), []byte(`{"version":2,"tracks":[]}`))

	// --library names the folder: nothing to guess.
	elsewhere := filepath.Join(t.TempDir(), "mine")
	t.Setenv("HOME", home)
	t.Setenv("MM_LIBRARY", "")
	if code, _, errs := runWith(t, cli.Deps{}, elsewhere, "ls"); code != 0 {
		t.Errorf("--library: code %d, err %q", code, errs)
	}
	// So does MM_LIBRARY.
	t.Setenv("MM_LIBRARY", elsewhere)
	var o, e bytes.Buffer
	if code := cli.Run(context.Background(), []string{"ls"}, cli.IO{In: strings.NewReader(""), Out: &o, Err: &e}, cli.Deps{}); code != 0 {
		t.Errorf("MM_LIBRARY: code %d, err %q", code, e.String())
	}
	// Once the move is done there is no old folder to mind.
	if err := os.Rename(old, filepath.Join(home, "Music", "Library")); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := runAt(t, home, "ls"); code != 0 {
		t.Errorf("after the move: code %d, err %q", code, errs)
	}
}
