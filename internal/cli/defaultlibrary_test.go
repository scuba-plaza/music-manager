package cli_test

import (
	"bytes"
	"context"
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

func TestTheDefaultLibraryIsMusicLibrary(t *testing.T) {
	home, src := t.TempDir(), t.TempDir()
	writeMP3(t, filepath.Join(src, "a.mp3"), testmp3.Silent(10))
	if code, _, errs := runAt(t, home, "ingest", src); code != 0 {
		t.Fatalf("code %d, err %q", code, errs)
	}
	if !exists(filepath.Join(home, "Music", "Library", "library.json")) {
		t.Error("no library in ~/Music/Library")
	}
}
