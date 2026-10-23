package device_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// What a device needs is known in internal/device and nowhere else, and the
// library holds the truth only. These sweeps read the source to see that it
// stays so; an entry in an allow-list is a decision, made with a reason.

// sources returns the Go files of the module as slash paths relative to its
// root, with their text.
func sources(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("..", "..")
	files := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "testdata", "result", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 50 {
		t.Fatalf("found only %d Go files: is the test run from internal/device?", len(files))
	}
	return files
}

// allowed says whether a file is on a list: an entry ending in "/" is a
// directory.
func allowed(file string, list []string) bool {
	for _, a := range list {
		if file == a || strings.HasSuffix(a, "/") && strings.HasPrefix(file, a) {
			return true
		}
	}
	return false
}

// sweep reports every file outside the allow-list whose text matches. Tests
// are only looked at when tests is set.
func sweep(t *testing.T, pattern string, tests bool, allow []string) {
	t.Helper()
	re := regexp.MustCompile(pattern)
	for file, text := range sources(t) {
		if strings.HasSuffix(file, "_test.go") && !tests || allowed(file, allow) {
			continue
		}
		if loc := re.FindStringIndex(text); loc != nil {
			line := 1 + strings.Count(text[:loc[0]], "\n")
			t.Errorf("%s:%d: %q is not for this file", file, line, text[loc[0]:loc[1]])
		}
	}
}

func TestTheLibraryKnowsNoLatinFormsOrPlayerNames(t *testing.T) {
	sweep(t, `OriginalTitle|OriginalArtist|OriginalAlbum|Ascii(Title|Artist|Album)|(Artist|Title|Album)ASCII\b|PlayerArtist|PlayerAlbum|player_artist|player_album|player-artist|player-album`, true, []string{
		"internal/manifest/legacy.go", "internal/manifest/legacy_test.go", // the v1 shape, read by mm migrate
		"internal/cli/migrate_test.go", // a v1 library as a fixture
		"internal/cli/cmd_migrate.go",  // reports what the v1 library held
		"internal/cli/cli_test.go",     // the flags of the old names are gone
		"internal/app/review_test.go",  // review files must refuse the old keys
		"internal/device/boundary_test.go",
	})
}

func TestOnlyTheNamedPlacesMakeLatinForms(t *testing.T) {
	sweep(t, `translit\.`, false, []string{
		"internal/translit/", "internal/device/",
		"internal/manifest/",   // the search finds a name by its Latin form; the old shape is migrated
		"internal/app/sort.go", // names sort by their Latin form
		"internal/tui/tree.go", // and so do the lists
	})
}

func TestTheWolfgangIsNamedOnlyWhereItIsChosenOrExplained(t *testing.T) {
	sweep(t, `(?i)wolfgang|wmp01|wolfang`, false, []string{
		"internal/device/",
		"cmd/mm/main.go",                // wires the flag to the interface
		"internal/cli/",                 // the --wolfgang flag and its help
		"internal/tui/",                 // the format row and the preview tab
		"internal/app/playlist.go",      // the names of every device's playlists must not clash
		"internal/coverimg/coverimg.go", // says why baseline JPEGs exist
		"internal/manifest/legacy.go",   // says what the old shape held
	})
}

func TestOnlyTheDeviceWritesWhatTheWolfgangReads(t *testing.T) {
	sweep(t, `coverimg\.Baseline|"\.m3u"`, false, []string{
		"internal/device/",
		"internal/testmp3/",    // a test helper that makes covers the way the player wants them
		"internal/app/sync.go", // knows which extensions are media, to report strays
	})
}

func TestTheReadmeDescribesTheCommandsThereAre(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("the README is not part of the source the package is built from")
	}
	if err != nil {
		t.Fatal(err)
	}
	if loc := regexp.MustCompile(`(?i)player-artist|player-album|player_artist|title-ascii|original_title|wolfang|cover normalize`).FindIndex(data); loc != nil {
		t.Errorf("README.md still says %q", data[loc[0]:loc[1]])
	}
}
