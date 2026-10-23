package manifest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"music-manager/internal/manifest"
)

// A version 1 library: every name was kept twice, in Latin letters (maybe set
// by hand) and as it really is, and the Wolfgang's names were kept per track.
const libraryV1 = `{
  "version": 1,
  "tracks": [
    {"id": "aaaaaa", "audio_sha256": "s1", "status": "identified",
     "title": "Watani", "original_title": "وطني",
     "artist": "Fairouz", "original_artist": "فيروز",
     "album": "Hits", "original_album": "Hits",
     "track_number": 3, "track_total": 12, "source_length_ns": 93000000000, "trim_start_ns": 2000000000,
     "released": "1999", "genre": "Folk", "player_artist": "Fairuz", "player_album": "Hits!",
     "gain_steps": 2, "cover_sha256": "c1", "source_url": "https://youtu.be/x",
     "added_at": "2026-10-22T12:00:00Z", "updated_at": "2026-10-23T12:00:00Z"},
    {"id": "bbbbbb", "status": "identified",
     "title": "Dream", "original_title": "Dream", "artist": "Max Richter", "original_artist": "Max Richter",
     "album": "", "original_album": ""},
    {"id": "cccccc", "status": "identified",
     "title": "Old Title", "original_title": "", "artist": "Old Artist", "original_artist": "",
     "album": "Old Album", "original_album": ""},
    {"id": "dddddd", "status": "unidentified", "title": "", "artist": "", "album": "",
     "hint": {"title": "x", "filename": "01 x.mp3"}}
  ],
  "playlists": [{"name": "Study", "tracks": ["aaaaaa", "bbbbbb"]}]
}`

func writeV1(t *testing.T) (path, backup string) {
	t.Helper()
	dir := t.TempDir()
	path = filepath.Join(dir, "library.json")
	if err := os.WriteFile(path, []byte(libraryV1), 0o644); err != nil {
		t.Fatal(err)
	}
	return path, filepath.Join(dir, ".mm", "library.json.v1")
}

func TestLoadOfAVersion1LibrarySaysToMigrate(t *testing.T) {
	path, _ := writeV1(t)
	_, err := manifest.NewStore(path).Load()
	if err == nil || !strings.Contains(err.Error(), "mm migrate") {
		t.Fatalf("err = %v, want one that says to run mm migrate", err)
	}
}

func TestMigrateKeepsTheTrueNamesAndEverythingElse(t *testing.T) {
	path, backup := writeV1(t)
	if _, err := manifest.Migrate(path, backup, false); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]manifest.Track{}
	for _, tr := range m.Tracks {
		byID[tr.ID] = tr
	}
	if len(m.Tracks) != 4 {
		t.Fatalf("%d tracks, want 4", len(m.Tracks))
	}
	a := byID["aaaaaa"]
	if a.Title != "وطني" || a.Artist != "فيروز" || a.Album != "Hits" {
		t.Errorf("names = %q / %q / %q, want the true ones", a.Title, a.Artist, a.Album)
	}
	if a.AudioSHA256 != "s1" || a.TrackNumber != 3 || a.TrackTotal != 12 || a.SourceLength != 93*time.Second || a.TrimStart != 2*time.Second ||
		a.Released != "1999" || a.Genre != "Folk" || a.GainSteps != 2 || a.CoverSHA256 != "c1" || a.SourceURL != "https://youtu.be/x" ||
		a.Status != manifest.Identified || !a.AddedAt.Equal(time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("the rest of the record changed: %+v", a)
	}
	if b := byID["bbbbbb"]; b.Title != "Dream" || b.Artist != "Max Richter" || b.Album != "" {
		t.Errorf("a track with Latin names = %+v", b)
	}
	if c := byID["cccccc"]; c.Title != "Old Title" || c.Artist != "Old Artist" || c.Album != "Old Album" {
		t.Errorf("a track with no original names keeps the ones it has: %+v", c)
	}
	if d := byID["dddddd"]; d.Status != manifest.Unidentified || d.Title != "" || d.Hint.Filename != "01 x.mp3" {
		t.Errorf("an unidentified track = %+v", d)
	}
	if len(m.Playlists) != 1 || m.Playlists[0].Name != "Study" || len(m.Playlists[0].Tracks) != 2 {
		t.Errorf("playlists = %+v", m.Playlists)
	}
}

func TestMigrateWritesNothingBackOfWhatItDrops(t *testing.T) {
	path, backup := writeV1(t)
	if _, err := manifest.Migrate(path, backup, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"original_", "player_artist", "player_album", "Watani", "Fairouz"} {
		if strings.Contains(string(data), gone) {
			t.Errorf("the migrated library still says %q", gone)
		}
	}
	if !strings.Contains(string(data), `"version": 2`) {
		t.Error("the migrated library is not version 2")
	}
	kept, err := os.ReadFile(backup)
	if err != nil || string(kept) != libraryV1 {
		t.Errorf("the backup is not the old library byte for byte (err %v)", err)
	}
}

func TestMigrateRefusesToRunTwice(t *testing.T) {
	path, backup := writeV1(t)
	if _, err := manifest.Migrate(path, backup, false); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if _, err := manifest.Migrate(path, backup, false); err == nil || !strings.Contains(err.Error(), "version 2") {
		t.Errorf("a second migrate: err = %v, want a refusal that names the version", err)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Error("the second migrate changed the library")
	}
}

func TestMigrateDoesNotOverwriteAnExistingBackup(t *testing.T) {
	path, backup := writeV1(t)
	if err := os.MkdirAll(filepath.Dir(backup), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, []byte("an older backup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := manifest.Migrate(path, backup, false); err == nil {
		t.Error("migrate replaced a backup that was already there")
	}
	if got, _ := os.ReadFile(path); string(got) != libraryV1 {
		t.Error("the library changed although the backup was in the way")
	}
}

func TestMigrateDryRunReportsWhatWouldGoAndWritesNothing(t *testing.T) {
	path, backup := writeV1(t)
	rep, err := manifest.Migrate(path, backup, true)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != libraryV1 {
		t.Error("a dry run changed the library")
	}
	if _, err := os.Stat(backup); err == nil {
		t.Error("a dry run wrote a backup")
	}
	if rep.Tracks != 4 {
		t.Errorf("tracks = %d, want 4", rep.Tracks)
	}
	// The Latin forms that were set by hand and differ from what is worked out
	// now: "Watani" / "Fairouz" for وطني / فيروز (which come out as Wtni and
	// Firuz). The names that are Latin to begin with are not among them.
	want := []manifest.HandSetName{
		{ID: "aaaaaa", Field: "title", Name: "وطني", Held: "Watani", Now: "Wtni"},
		{ID: "aaaaaa", Field: "artist", Name: "فيروز", Held: "Fairouz", Now: "Firuz"},
	}
	if len(rep.HandSet) != len(want) {
		t.Fatalf("hand-set names = %+v, want %+v", rep.HandSet, want)
	}
	for i := range want {
		if rep.HandSet[i] != want[i] {
			t.Errorf("hand-set name %d = %+v, want %+v", i, rep.HandSet[i], want[i])
		}
	}
	if rep.PlayerArtists != 1 || rep.PlayerAlbums != 1 {
		t.Errorf("player names dropped = %d artists, %d albums; want 1 and 1", rep.PlayerArtists, rep.PlayerAlbums)
	}
}
