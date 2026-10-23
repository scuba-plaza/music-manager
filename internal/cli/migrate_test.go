package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const oldLibrary = `{"version": 1, "tracks": [
  {"id": "aaaaaa", "status": "identified", "title": "Watani", "original_title": "وطني",
   "artist": "Fairouz", "original_artist": "فيروز", "album": "Hits", "original_album": "Hits", "player_artist": "Fairuz", "source_length_ns": 93000000000},
  {"id": "bbbbbb", "status": "identified", "title": "Kifak", "original_title": "كيفك",
   "artist": "Fairouz", "original_artist": "فيروز", "album": "", "original_album": "", "source_length_ns": 93000000000}
]}`

func oldLib(t *testing.T) string {
	t.Helper()
	lib := filepath.Join(t.TempDir(), "lib")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lib, "library.json"), []byte(oldLibrary), 0o644); err != nil {
		t.Fatal(err)
	}
	return lib
}

func TestAnOldLibraryAsksForMigrate(t *testing.T) {
	lib := oldLib(t)
	code, _, errs := run(t, lib, "ls")
	if code == 0 || !strings.Contains(errs, "mm migrate") {
		t.Errorf("ls on a version 1 library: code %d, stderr %q; want it to say mm migrate", code, errs)
	}
}

func TestMigrateDryRunShowsWhatChangesAndWritesNothing(t *testing.T) {
	lib := oldLib(t)
	code, out, errs := run(t, lib, "migrate", "--dry-run")
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, errs)
	}
	s := squash(out)
	for _, want := range []string{"2 tracks", "title وطني Watani → Wtni", "artist فيروز Fairouz → Firuz (2 tracks)", "artist names on 1 track", "dry run"} {
		if !strings.Contains(s, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if got, _ := os.ReadFile(filepath.Join(lib, "library.json")); string(got) != oldLibrary {
		t.Error("a dry run changed the library")
	}
}

func TestMigrateThenTheLibraryWorks(t *testing.T) {
	lib := oldLib(t)
	code, out, errs := run(t, lib, "migrate")
	if code != 0 || !strings.Contains(out, "migrated 2 tracks") || !strings.Contains(out, "library.json.v1") {
		t.Fatalf("migrate: code %d, out %q, stderr %q", code, out, errs)
	}
	if kept, _ := os.ReadFile(filepath.Join(lib, ".mm", "library.json.v1")); string(kept) != oldLibrary {
		t.Error("the old library is not kept as it was")
	}
	_, out, _ = run(t, lib, "ls")
	for _, want := range []string{"وطني", "فيروز", "كيفك"} {
		if !strings.Contains(out, want) {
			t.Errorf("ls lacks %q:\n%s", want, out)
		}
	}
	if code, _, errs = run(t, lib, "migrate"); code == 0 || !strings.Contains(errs, "already version 2") {
		t.Errorf("a second migrate: code %d, stderr %q", code, errs)
	}
}
