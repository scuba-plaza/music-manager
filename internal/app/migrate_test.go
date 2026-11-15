package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"music-manager/internal/app"
)

const v2Hints = `{"version": 2, "tracks": [
  {"id": "aaaaaa", "audio_sha256": "x", "status": "unidentified", "source_length_ns": 1000000000,
   "hint": {"title": "So What", "artist": "Miles Davis feat. Someone", "album": "Kind of Blue", "track": 1, "of": 5, "date": "1959-08-17", "genre": "Jazz", "filename": "01 so what.mp3"}},
  {"id": "bbbbbb", "audio_sha256": "y", "status": "unidentified", "source_length_ns": 1000000000,
   "hint": {"artist": "Basement Tapes", "album": "Demos", "track": 7, "filename": "rough.mp3"}},
  {"id": "cccccc", "audio_sha256": "z", "status": "identified", "title": "Song", "artist": "Band", "source_length_ns": 1000000000}
]}
`

func openV2(t *testing.T) (*app.App, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "library.json"), []byte(v2Hints), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := app.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return a, root
}

func TestOpeningAVersion2LibraryFillsTheTracksThatHaveATitleAndAnArtistInTheirHints(t *testing.T) {
	a, _ := openV2(t)

	m, err := a.Manifest()
	if err != nil {
		t.Fatal(err)
	}

	filled, _ := m.Get("aaaaaa")
	if filled.Status != "identified" || filled.Title != "So What" || filled.Artist != "Miles Davis" || filled.Album != "Kind of Blue" ||
		filled.TrackNumber != 1 || filled.TrackTotal != 5 || filled.Released != "1959-08-17" || filled.Genre != "Jazz" {
		t.Errorf("%+v", filled)
	}
	if filled.Hint.Title != "" || filled.Hint.Artist != "" {
		t.Errorf("a filled track has no hint left: %+v", filled.Hint)
	}
}

func TestATrackWithOnlyPartOfItsHintStaysToBeWorkedOn(t *testing.T) {
	a, _ := openV2(t)

	m, _ := a.Manifest()

	rough, _ := m.Get("bbbbbb")
	if rough.Status != "unidentified" || rough.Hint.Artist != "Basement Tapes" || rough.Hint.Album != "Demos" || rough.Hint.Track != 7 {
		t.Errorf("%+v", rough)
	}
	known, _ := m.Get("cccccc")
	if known.Title != "Song" || known.Artist != "Band" {
		t.Errorf("%+v", known)
	}
}

func TestTheMigrationWritesVersion3KeepsTheOldFileAndIsDoneOnce(t *testing.T) {
	_, root := openV2(t)
	path := filepath.Join(root, "library.json")

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"version": 3`) {
		t.Errorf("not version 3: %.60s", data)
	}
	if kept, _ := os.ReadFile(path + ".v2.bak"); string(kept) != v2Hints {
		t.Errorf("the old file is not kept as it was: %.60s", kept)
	}
	if _, err := app.Open(root); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(path); string(again) != string(data) {
		t.Error("opening the library again changed it")
	}
}

func TestOpeningALibraryThatIsNotThereWritesNothing(t *testing.T) {
	root := t.TempDir()

	if _, err := app.Open(root); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, "library.json")); !os.IsNotExist(err) {
		t.Errorf("opening made a library file: %v", err)
	}
}
