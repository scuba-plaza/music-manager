package manifest_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"music-manager/internal/manifest"
)

func TestLoadMissingFileIsEmpty(t *testing.T) {
	s := manifest.NewStore(filepath.Join(t.TempDir(), "library.json"))
	m, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Tracks) != 0 || len(m.Profiles) != 0 {
		t.Errorf("got %+v, want no tracks and no profiles", m)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	s := manifest.NewStore(filepath.Join(t.TempDir(), "library.json"))
	want := &manifest.Manifest{
		Tracks: []manifest.Track{{
			ID:           "k7f2qd",
			AudioSHA256:  "abc123",
			SourceURL:    "https://youtu.be/xyz",
			Status:       manifest.Identified,
			Title:        "وطني",
			Artist:       "Max Richter",
			Album:        "Sleep",
			TrackNumber:  3,
			TrackTotal:   8,
			SourceLength: 93 * time.Second,
			TrimStart:    2 * time.Second,
			TrimEnd:      1500 * time.Millisecond,
			Hint:         manifest.Hint{Title: "old title", Artist: "old artist", Album: "old album", Filename: "01 Old.mp3"},
			TitleLatin:   "Watani",
			CoverSHA256:  "def456",
			AddedAt:      time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC),
		}},
		Playlists: []manifest.Playlist{{Name: "دراسة", NameLatin: "Study", Tracks: []string{"k7f2qd"}}},
		Profiles:  []manifest.Profile{manifest.WolfgangWMP01},
	}
	if err := replace(s, want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestLoadRefusesAVersionItDoesNotKnow(t *testing.T) {
	for _, version := range []string{"1", "99"} {
		path := filepath.Join(t.TempDir(), "library.json")
		data := `{"version": ` + version + `, "tracks": []}`
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := manifest.NewStore(path).Load(); err == nil || !strings.Contains(err.Error(), "unsupported library version "+version) {
			t.Errorf("version %s: err = %v", version, err)
		}
		if after, _ := os.ReadFile(path); string(after) != data {
			t.Errorf("version %s: loading changed the file", version)
		}
	}
}

func TestSaveKeepsPreviousVersionAsBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	s := manifest.NewStore(path)
	if err := replace(s, &manifest.Manifest{Tracks: []manifest.Track{{ID: "first"}}}); err != nil {
		t.Fatal(err)
	}
	if err := replace(s, &manifest.Manifest{Tracks: []manifest.Track{{ID: "second"}}}); err != nil {
		t.Fatal(err)
	}
	bak, err := manifest.NewStore(path + ".bak").Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(bak.Tracks) != 1 || bak.Tracks[0].ID != "first" {
		t.Errorf("backup = %+v, want the first save", bak.Tracks)
	}
}

func TestSaveLeavesNoTempFilesBehind(t *testing.T) {
	dir := t.TempDir()
	s := manifest.NewStore(filepath.Join(dir, "library.json"))
	for i := 0; i < 3; i++ {
		if err := replace(s, &manifest.Manifest{}); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	want := []string{"library.json", "library.json.bak", "library.json.lock"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("dir = %v, want %v", names, want)
	}
}

func TestFailedSaveKeepsExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")
	s := manifest.NewStore(path)
	if err := replace(s, &manifest.Manifest{Tracks: []manifest.Track{{ID: "keep"}}}); err != nil {
		t.Fatal(err)
	}
	// An unencodable value makes marshalling fail before anything is written.
	bad := &manifest.Manifest{Tracks: []manifest.Track{{ID: "bad", AddedAt: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}}}
	if err := replace(s, bad); err == nil {
		t.Fatal("Save succeeded, want error")
	}
	m, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Tracks) != 1 || m.Tracks[0].ID != "keep" {
		t.Errorf("after failed save: %+v, want the original", m.Tracks)
	}
}

func TestAGenreIsWrittenOnlyWhenThereIsOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	s := manifest.NewStore(path)
	m := &manifest.Manifest{Tracks: []manifest.Track{{ID: "aaaaaa", Genre: "Neoclassical"}, {ID: "bbbbbb"}}}
	if err := replace(s, m); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), `"genre"`); got != 1 {
		t.Errorf("%d genre keys in the file, want 1 (a track without one writes none):\n%s", got, data)
	}
	back, err := s.Load()
	if err != nil || back.Tracks[0].Genre != "Neoclassical" || back.Tracks[1].Genre != "" {
		t.Errorf("loaded %+v, %v", back.Tracks, err)
	}
}

// replace makes the records of the store exactly m, as a change.
func replace(s *manifest.Store, m *manifest.Manifest) error {
	return s.Update(func(cur *manifest.Manifest) error { *cur = *m; return nil })
}
