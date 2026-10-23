package app_test

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestIngestKeepsOldTagsAndFilenameAsHintsOnly(t *testing.T) {
	a := openLib(t)
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "01 Bahlam Bek.mp3"),
		testmp3.WithTags(testmp3.Silent(5), "بحلم بيك", "Abdel Halim Hafez", "Rawaeaa"))
	writeFile(t, filepath.Join(src, "Franz Kafka", "01 - The Metamorphosis.mp3"), testmp3.Silent(6))

	if _, err := a.Ingest([]string{src}); err != nil {
		t.Fatal(err)
	}
	hints := map[string]manifest.Hint{}
	for _, tr := range tracks(t, a) {
		hints[tr.Hint.Filename] = tr.Hint
		if tr.Title != "" || tr.Artist != "" || tr.Album != "" {
			t.Errorf("old tags leaked into the record: %+v", tr)
		}
	}
	want := map[string]manifest.Hint{
		"01 Bahlam Bek":          {Title: "بحلم بيك", Artist: "Abdel Halim Hafez", Album: "Rawaeaa", Filename: "01 Bahlam Bek"},
		"01 - The Metamorphosis": {Filename: "01 - The Metamorphosis"},
	}
	for k, w := range want {
		if hints[k] != w {
			t.Errorf("hint for %q = %+v, want %+v", k, hints[k], w)
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

func TestIngestAcceptsASingleFilePath(t *testing.T) {
	a := openLib(t)
	f := filepath.Join(t.TempDir(), "x.mp3")
	writeFile(t, f, testmp3.Silent(3))
	if rep, err := a.Ingest([]string{f}); err != nil || len(rep.Added) != 1 {
		t.Errorf("rep = %+v, err = %v", rep, err)
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
