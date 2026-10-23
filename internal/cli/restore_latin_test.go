package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A library migrated before the Latin forms could be kept has lost the ones set
// by hand; the backup of the old file still has them.

// migratedWithoutForms is oldLib migrated, with the forms that the migration
// kept taken away again, as in a library migrated by an earlier version.
func migratedWithoutForms(t *testing.T) string {
	t.Helper()
	lib := oldLib(t)
	if code, _, errs := run(t, lib, "migrate"); code != 0 {
		t.Fatalf("migrate: %s", errs)
	}
	for _, id := range []string{"aaaaaa", "bbbbbb"} {
		if code, _, errs := run(t, lib, "edit", id, "--title-latin", "", "--artist-latin", ""); code != 0 {
			t.Fatalf("edit: %s", errs)
		}
	}
	return lib
}

func TestRestoreLatinShowsWhatItWouldPutBackAndWritesNothing(t *testing.T) {
	lib := migratedWithoutForms(t)
	before, _ := os.ReadFile(filepath.Join(lib, "library.json"))
	code, out, errs := run(t, lib, "restore-latin", "--dry-run")
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, errs)
	}
	s := squash(out)
	for _, want := range []string{"title وطني Watani", "artist فيروز Fairouz (2 tracks)", "dry run"} {
		if !strings.Contains(s, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if after, _ := os.ReadFile(filepath.Join(lib, "library.json")); string(after) != string(before) {
		t.Error("a dry run changed the library")
	}
}

func TestRestoreLatinPutsBackTheFormsOfTheNamesThatAreStillThere(t *testing.T) {
	lib := migratedWithoutForms(t)
	run(t, lib, "edit", "bbbbbb", "--artist", "فيروز الكبيرة")  // renamed since: its form is not for this name
	run(t, lib, "edit", "aaaaaa", "--title-latin", "Watani II") // set again since: not overwritten

	code, out, errs := run(t, lib, "restore-latin")
	if code != 0 {
		t.Fatalf("code %d, out %q, stderr %q", code, out, errs)
	}
	s := squash(out)
	for _, want := range []string{"restored 2 forms on 2 tracks", "left alone 2: the name was changed since (1), the track has a form of its own (1)"} {
		if !strings.Contains(s, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	_, shown, _ := run(t, lib, "show", "aaaaaa")
	if !strings.Contains(squash(shown), "Latin artist Fairouz") || !strings.Contains(squash(shown), "Latin title Watani II") {
		t.Errorf("show aaaaaa:\n%s", shown)
	}
	_, shown, _ = run(t, lib, "show", "bbbbbb")
	if strings.Contains(shown, "Latin artist") || !strings.Contains(squash(shown), "Latin title Kifak") {
		t.Errorf("show bbbbbb has a form for a name it was not set for, or lacks the one that was:\n%s", shown)
	}

	// A second run finds nothing left to do.
	if code, out, _ = run(t, lib, "restore-latin"); code != 0 || !strings.Contains(out, "nothing to restore") {
		t.Errorf("second run: code %d, out %q", code, out)
	}
}

func TestRestoreLatinNeedsTheBackupOfTheOldLibrary(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	run(t, lib, "ls")
	code, _, errs := run(t, lib, "restore-latin")
	if code == 0 || !strings.Contains(errs, "library.json.v1") {
		t.Errorf("code %d, stderr %q, want a refusal that names the backup", code, errs)
	}
}
