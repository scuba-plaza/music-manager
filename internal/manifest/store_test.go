package manifest_test

import (
	"os"
	"path/filepath"
	"reflect"
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
	if len(m.Tracks) != 0 {
		t.Errorf("got %d tracks, want 0", len(m.Tracks))
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	s := manifest.NewStore(filepath.Join(t.TempDir(), "library.json"))
	want := manifest.Track{
		ID:             "k7f2qd",
		AudioSHA256:    "abc123",
		SourceURL:      "https://youtu.be/xyz",
		Status:         manifest.Identified,
		Title:          "Firuz",
		OriginalTitle:  "فيروز",
		Artist:         "Max Richter",
		OriginalArtist: "Max Richter",
		Album:          "Sleep",
		OriginalAlbum:  "Sleep",
		TrackNumber:    3,
		TrackTotal:     8,
		SourceLength:   93 * time.Second,
		TrimStart:      2 * time.Second,
		TrimEnd:        1500 * time.Millisecond,
		Hint:           manifest.Hint{Title: "old title", Artist: "old artist", Album: "old album", Filename: "01 Old.mp3"},
		CoverSHA256:    "def456",
		AddedAt:        time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC),
	}
	if err := s.Save(&manifest.Manifest{Tracks: []manifest.Track{want}}); err != nil {
		t.Fatal(err)
	}
	m, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Tracks) != 1 || !reflect.DeepEqual(m.Tracks[0], want) {
		t.Errorf("got %+v, want %+v", m.Tracks, want)
	}
}

func TestLoadRejectsUnknownVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	if err := os.WriteFile(path, []byte(`{"version": 99, "tracks": []}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := manifest.NewStore(path).Load(); err == nil {
		t.Fatal("Load accepted version 99, want error")
	}
}

func TestSaveKeepsPreviousVersionAsBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	s := manifest.NewStore(path)
	if err := s.Save(&manifest.Manifest{Tracks: []manifest.Track{{ID: "first"}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(&manifest.Manifest{Tracks: []manifest.Track{{ID: "second"}}}); err != nil {
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
		if err := s.Save(&manifest.Manifest{}); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	want := []string{"library.json", "library.json.bak"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("dir = %v, want %v", names, want)
	}
}

func TestFailedSaveKeepsExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")
	s := manifest.NewStore(path)
	if err := s.Save(&manifest.Manifest{Tracks: []manifest.Track{{ID: "keep"}}}); err != nil {
		t.Fatal(err)
	}
	// An unencodable value makes marshalling fail before anything is written.
	bad := &manifest.Manifest{Tracks: []manifest.Track{{ID: "bad", AddedAt: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}}}
	if err := s.Save(bad); err == nil {
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
