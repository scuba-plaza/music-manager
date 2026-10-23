package app_test

import (
	"path/filepath"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/testmp3"
)

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

func TestAdoptTakesWhatTheOldTagsSaid(t *testing.T) {
	a := openLib(t)
	tr := ingestTagged(t, a, "01 Bahlam Bek", "بحلم بيك", "Abdel Halim Hafez", "Rawaeaa", "1/6")
	if tr.Hint.Track != 1 || tr.Hint.Total != 6 {
		t.Fatalf("hint = %+v; the old track number should be kept", tr.Hint)
	}
	res, err := a.Adopt(nil)
	if err != nil || len(res) != 1 || !res[0].Adopted {
		t.Fatalf("%+v, %v", res, err)
	}
	got := tracks(t, a)[0]
	if got.Status != manifest.Identified || got.OriginalTitle != "بحلم بيك" ||
		got.OriginalArtist != "Abdel Halim Hafez" || got.OriginalAlbum != "Rawaeaa" || got.TrackNumber != 1 || got.TrackTotal != 6 {
		t.Errorf("track = %+v", got)
	}
	if got.Hint != (manifest.Hint{}) || got.MBRecordingID != "" {
		t.Errorf("hint %+v, mb %q", got.Hint, got.MBRecordingID)
	}
}

func TestAdoptFallsBackToTheNumberInTheFileName(t *testing.T) {
	a := openLib(t)
	ingestTagged(t, a, "07 - Some Song", "Some Song", "Band", "", "")
	if _, err := a.Adopt(nil); err != nil {
		t.Fatal(err)
	}
	if got := tracks(t, a)[0]; got.TrackNumber != 7 || got.OriginalAlbum != "" {
		t.Errorf("track = %+v", got)
	}
}

func TestAdoptLeavesTracksThatLackATitleOrArtist(t *testing.T) {
	a := openLib(t)
	ingestTagged(t, a, "x", "Only A Title", "", "", "")
	ingestTagged(t, a, "yy", "", "Only An Artist", "", "")
	res, err := a.Adopt(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if r.Adopted || r.Reason == "" {
			t.Errorf("result = %+v", r)
		}
	}
	for _, tr := range tracks(t, a) {
		if tr.Status != manifest.Unidentified {
			t.Errorf("%+v was identified", tr)
		}
	}
}

func TestAdoptOnlyTouchesUnidentifiedTracksAndNamedOnes(t *testing.T) {
	a := openLib(t)
	done := ingestTagged(t, a, "a1", "Edited", "Band", "", "")
	identify(t, a, done.ID, "My Title", "My Artist", "", 0, 0)
	pending := ingestTagged(t, a, "b22", "Old Title", "Old Artist", "", "")

	res, _ := a.Adopt(nil)
	if len(res) != 1 || res[0].TrackID != pending.ID {
		t.Fatalf("results = %+v", res)
	}
	res, err := a.Adopt([]string{done.ID})
	if err != nil || len(res) != 1 || res[0].Adopted || res[0].Reason == "" {
		t.Errorf("an identified track must be left alone: %+v, %v", res, err)
	}
	if got, _ := (func() (manifest.Track, bool) { m, _ := a.Manifest(); return m.Get(done.ID) })(); got.OriginalTitle != "My Title" {
		t.Errorf("identified track changed: %+v", got)
	}
	if _, err := a.Adopt([]string{"nope"}); err == nil {
		t.Error("unknown id accepted")
	}
}

func TestAdoptTakesTheDateTheOldTagsSaid(t *testing.T) {
	for _, c := range []struct{ tag, want string }{
		{"1987", "1987"},
		{"2015-09-04", "2015-09-04"},
		{"2015-09-04T10:00:00", "2015-09-04"}, // tag editors write full timestamps
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
		if got := rep.Added[0].Hint.Date; got != c.want {
			t.Errorf("tag %q: hint date = %q, want %q", c.tag, got, c.want)
		}
		if _, err := a.Adopt(nil); err != nil {
			t.Fatal(err)
		}
		got := tracks(t, a)[0]
		if got.Released != c.want || got.Hint != (manifest.Hint{}) {
			t.Errorf("tag %q: released = %q, hint %+v; want %q", c.tag, got.Released, got.Hint, c.want)
		}
	}
}

func TestAdoptTakesTheGenreTheOldTagsSaid(t *testing.T) {
	a := openLib(t)
	f := filepath.Join(t.TempDir(), "x.mp3")
	writeFile(t, f, testmp3.WithTextFrames(testmp3.Silent(30),
		[2]string{"TIT2", "Song"}, [2]string{"TPE1", "Band"}, [2]string{"TCON", "(17)Folk"}))
	rep, err := a.Ingest([]string{f})
	if err != nil || len(rep.Added) != 1 {
		t.Fatalf("%+v, %v", rep, err)
	}
	if got := rep.Added[0].Hint.Genre; got != "Folk" {
		t.Errorf("hint genre = %q, want Folk", got)
	}
	if _, err := a.Adopt(nil); err != nil {
		t.Fatal(err)
	}
	if got := tracks(t, a)[0]; got.Genre != "Folk" || got.Hint != (manifest.Hint{}) {
		t.Errorf("genre = %q, hint %+v", got.Genre, got.Hint)
	}
}
