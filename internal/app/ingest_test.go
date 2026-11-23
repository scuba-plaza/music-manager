package app_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/testmp3"
)

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func openLib(t *testing.T) *app.App {
	t.Helper()
	a, err := app.Open(filepath.Join(t.TempDir(), "lib"))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func tracks(t *testing.T, a *app.App) []manifest.Track {
	t.Helper()
	m, err := a.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	return m.Tracks
}

func TestIngestCreatesUnidentifiedTracksAndStoresAudio(t *testing.T) {
	a := openLib(t)
	src := t.TempDir()
	audio := testmp3.Silent(100)
	original := testmp3.WithID3v1(testmp3.WithID3v2(audio, 3, 500))
	writeFile(t, filepath.Join(src, "a.mp3"), original)

	rep, err := a.Ingest([]string{src})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Added) != 1 || len(rep.Duplicates) != 0 || len(rep.Failed) != 0 {
		t.Fatalf("report = %+v, want exactly one added", rep)
	}

	ts := tracks(t, a)
	if len(ts) != 1 {
		t.Fatalf("%d tracks in manifest, want 1", len(ts))
	}
	tr := ts[0]
	if tr.Status != manifest.Unidentified || tr.ID == "" || tr.SourceURL != "" {
		t.Errorf("track = %+v", tr)
	}
	if d := tr.SourceLength - 2612*time.Millisecond; d < -time.Millisecond || d > time.Millisecond {
		t.Errorf("SourceLength = %v, want about 2.612s", tr.SourceLength)
	}
	blob, err := os.ReadFile(a.AudioPath(tr.AudioSHA256))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(blob, audio) {
		t.Errorf("stored blob is %d bytes, want the %d bytes of bare audio", len(blob), len(audio))
	}
	if after, _ := os.ReadFile(filepath.Join(src, "a.mp3")); !bytes.Equal(after, original) {
		t.Error("ingest modified the source file")
	}
}

// ingestTagged ingests a file with the given name and old tags.
func ingestTagged(t *testing.T, a *app.App, name, title, artist, album, track string) manifest.Track {
	t.Helper()
	f := filepath.Join(t.TempDir(), name+".mp3")
	writeFile(t, f, testmp3.WithTagsTrack(testmp3.Silent(20+len(name)), title, artist, album, track))
	rep, err := a.Ingest([]string{f})
	if err != nil || len(rep.Added) != 1 {
		t.Fatalf("ingest %s: %+v, %v", name, rep, err)
	}
	return rep.Added[0]
}

func TestIngestFillsTheRecordFromTheFilesOwnTags(t *testing.T) {
	a := openLib(t)
	added := ingestTagged(t, a, "01 Bahlam Bek", "بحلم بيك", "Abdel Halim Hafez", "Rawaeaa", "1/6")
	got := tracks(t, a)[0]
	for _, tr := range []manifest.Track{added, got} {
		if tr.Status != manifest.Identified || tr.Title != "بحلم بيك" || tr.Artist != "Abdel Halim Hafez" ||
			tr.Album != "Rawaeaa" || tr.TrackNumber != 1 || tr.TrackTotal != 6 {
			t.Errorf("track = %+v", tr)
		}
		if tr.Hint != (manifest.Hint{}) {
			t.Errorf("hint %+v should be gone once the record holds what it said", tr.Hint)
		}
	}
}

func TestIngestFallsBackToTheNumberInTheFileName(t *testing.T) {
	a := openLib(t)
	ingestTagged(t, a, "07 - Some Song", "Some Song", "Band", "", "")
	if got := tracks(t, a)[0]; got.TrackNumber != 7 || got.Album != "" || got.Status != manifest.Identified {
		t.Errorf("track = %+v", got)
	}
}

func TestIngestKeepsAFileThatLacksATitleOrArtistInNeedsWork(t *testing.T) {
	a := openLib(t)
	ingestTagged(t, a, "x", "Only A Title", "", "", "")
	ingestTagged(t, a, "yy", "", "Only An Artist", "", "")
	untagged := filepath.Join(t.TempDir(), "01 - The Metamorphosis.mp3")
	writeFile(t, untagged, testmp3.Silent(6))
	if _, err := a.Ingest([]string{untagged}); err != nil {
		t.Fatal(err)
	}
	for _, tr := range tracks(t, a) {
		if tr.Status != manifest.Unidentified || tr.Title != "" || tr.Artist != "" || tr.Album != "" {
			t.Errorf("%+v was filled from tags that are not enough", tr)
		}
	}
	hints := map[string]manifest.Hint{}
	for _, tr := range tracks(t, a) {
		hints[tr.Hint.Filename] = tr.Hint
	}
	if h := hints["x"]; h.Title != "Only A Title" {
		t.Errorf("what the old tags did say is kept to show: %+v", h)
	}
	if h := hints["01 - The Metamorphosis"]; h != (manifest.Hint{Filename: "01 - The Metamorphosis"}) {
		t.Errorf("hint = %+v", h)
	}
}

func TestIngestTakesTheDateTheTagsSaid(t *testing.T) {
	for _, c := range []struct{ tag, want string }{
		{"1987", "1987"},
		{"2015-09-04", "2015-09-04"},
		{"2015-09-04T10:00:00", "2015-09-04"},
		{"sometime", ""},
		{"", ""},
	} {
		a := openLib(t)
		f := filepath.Join(t.TempDir(), "x.mp3")
		writeFile(t, f, testmp3.WithTextFrames(testmp3.Silent(30),
			[2]string{"TIT2", "Song"}, [2]string{"TPE1", "Band"}, [2]string{"TDRC", c.tag}))
		rep, err := a.Ingest([]string{f})
		if err != nil || len(rep.Added) != 1 {
			t.Fatalf("%q: %+v, %v", c.tag, rep, err)
		}
		if got := tracks(t, a)[0]; got.Released != c.want {
			t.Errorf("tag %q: released = %q, want %q", c.tag, got.Released, c.want)
		}
	}
}

func TestIngestTakesTheGenreTheTagsSaid(t *testing.T) {
	a := openLib(t)
	f := filepath.Join(t.TempDir(), "x.mp3")
	writeFile(t, f, testmp3.WithTextFrames(testmp3.Silent(30),
		[2]string{"TIT2", "Song"}, [2]string{"TPE1", "Band"}, [2]string{"TCON", "(17)Folk"}))
	if _, err := a.Ingest([]string{f}); err != nil {
		t.Fatal(err)
	}
	if got := tracks(t, a)[0]; got.Genre != "Folk" {
		t.Errorf("genre = %q, want Folk", got.Genre)
	}
}

func TestIngestTakesTheFirstOfSeveralArtistsOnlyWhereTheTagsSaySo(t *testing.T) {
	cases := []struct{ tag, want string }{
		{"Fairuz; Ziad Rahbani", "Fairuz"},
		{"Fairuz\x00Ziad Rahbani", "Fairuz"},
		{"Nils Frahm feat. Anne Müller", "Nils Frahm"},
		{"Nils Frahm Feat. Anne Müller", "Nils Frahm"},
		{"Nils Frahm ft. Anne Müller", "Nils Frahm"},
		{"Nils Frahm featuring Anne Müller", "Nils Frahm"},
		{"Simon & Garfunkel", "Simon & Garfunkel"},
		{"Earth, Wind & Fire", "Earth, Wind & Fire"},
		{"AC/DC", "AC/DC"},
		{"Craft Beer Anthem", "Craft Beer Anthem"},
	}
	for i, c := range cases {
		a := openLib(t)
		ingestTagged(t, a, strings.Repeat("x", i+1), "Song", c.tag, "", "")
		if got := tracks(t, a)[0].Artist; got != c.want {
			t.Errorf("tag %q: artist %q, want %q", c.tag, got, c.want)
		}
	}
}

func TestIngestSkipsAudioAlreadyInTheLibrary(t *testing.T) {
	a := openLib(t)
	src := t.TempDir()
	audio := testmp3.Silent(7)
	writeFile(t, filepath.Join(src, "one.mp3"), testmp3.WithTags(audio, "Title", "", ""))
	writeFile(t, filepath.Join(src, "copy.mp3"), testmp3.WithID3v1(audio)) // same audio, other tags

	rep, err := a.Ingest([]string{src})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Added) != 1 || len(rep.Duplicates) != 1 {
		t.Fatalf("report = %+v, want 1 added and 1 duplicate", rep)
	}

	rep, err = a.Ingest([]string{src})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Added) != 0 || len(rep.Duplicates) != 2 || len(tracks(t, a)) != 1 {
		t.Errorf("second ingest: report = %+v, %d tracks", rep, len(tracks(t, a)))
	}
}

func TestIngestIgnoresOtherFilesAndReportsBrokenMP3s(t *testing.T) {
	a := openLib(t)
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "notes.txt"), []byte("hi"))
	writeFile(t, filepath.Join(src, "cover.jpg"), []byte{0xFF, 0xD8})
	writeFile(t, filepath.Join(src, "broken.mp3"), []byte("not really an mp3"))
	writeFile(t, filepath.Join(src, "LOUD.MP3"), testmp3.Silent(3))

	rep, err := a.Ingest([]string{src})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Added) != 1 || len(rep.Failed) != 1 || filepath.Base(rep.Failed[0].Path) != "broken.mp3" {
		t.Errorf("report = %+v, want LOUD.MP3 added and broken.mp3 failed", rep)
	}
}

func TestIngestSkipsTheLibrarysOwnStore(t *testing.T) {
	a := openLib(t)
	writeFile(t, filepath.Join(a.Root(), "Old", "song.mp3"), testmp3.Silent(10))
	if rep, err := a.Ingest([]string{a.Root()}); err != nil || len(rep.Added) != 1 {
		t.Fatalf("first ingest: %+v, %v", rep, err)
	}
	// The stored copy of that song now sits in .mm/audio inside the root.
	rep, err := a.Ingest([]string{a.Root()})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Added) != 0 || len(rep.Duplicates) != 1 || len(rep.Failed) != 0 {
		t.Errorf("second ingest = %+v; only Old/song.mp3 should be seen, as a duplicate", rep)
	}
}
