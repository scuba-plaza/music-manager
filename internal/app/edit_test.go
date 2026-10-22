package app_test

import (
	"path/filepath"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/testmp3"
)

func ptr[T any](v T) *T { return &v }

// ingested returns a library holding one 100-frame (about 2.6s) track.
func ingested(t *testing.T) (*app.App, manifest.Track) {
	t.Helper()
	a := openLib(t)
	f := filepath.Join(t.TempDir(), "x.mp3")
	writeFile(t, f, testmp3.WithTags(testmp3.Silent(100), "old", "old", "old"))
	rep, err := a.Ingest([]string{f})
	if err != nil || len(rep.Added) != 1 {
		t.Fatalf("ingest: %+v, %v", rep, err)
	}
	return a, rep.Added[0]
}

func TestEditTitleAndArtistIdentifiesTheTrack(t *testing.T) {
	a, tr := ingested(t)
	got, err := a.Edit(tr.ID, app.Edit{Title: ptr("وطني"), Artist: ptr("فيروز"), Album: ptr("Hits"), TrackNumber: ptr(3), TrackTotal: ptr(12)})
	if err != nil {
		t.Fatal(err)
	}
	want := manifest.Track{
		Status: manifest.Identified,
		Title:  "Wtni", OriginalTitle: "وطني",
		Artist: "Firuz", OriginalArtist: "فيروز",
		Album: "Hits", OriginalAlbum: "Hits",
		TrackNumber: 3, TrackTotal: 12,
	}
	if got.Status != want.Status || got.Title != want.Title || got.OriginalTitle != want.OriginalTitle ||
		got.Artist != want.Artist || got.OriginalArtist != want.OriginalArtist ||
		got.Album != want.Album || got.OriginalAlbum != want.OriginalAlbum ||
		got.TrackNumber != 3 || got.TrackTotal != 12 {
		t.Errorf("got %+v", got)
	}
	if got.Hint != (manifest.Hint{}) {
		t.Errorf("hint not cleared: %+v", got.Hint)
	}
	if saved := tracks(t, a)[0]; saved.Title != "Wtni" {
		t.Errorf("edit was not saved: %+v", saved)
	}
}

func TestEditNeedsTitleAndArtistToIdentify(t *testing.T) {
	a, tr := ingested(t)
	got, err := a.Edit(tr.ID, app.Edit{Title: ptr("Only a title"), Album: ptr("Some album")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != manifest.Unidentified || got.Hint.Filename == "" {
		t.Errorf("status = %s, hint = %+v; want unidentified with hint kept", got.Status, got.Hint)
	}

	got, _ = a.Edit(tr.ID, app.Edit{Artist: ptr("An artist")})
	if got.Status != manifest.Identified {
		t.Errorf("status = %s after adding the artist, want identified", got.Status)
	}
	got, _ = a.Edit(tr.ID, app.Edit{Title: ptr("")})
	if got.Status != manifest.Unidentified {
		t.Errorf("status = %s after clearing the title, want unidentified", got.Status)
	}
}

func TestEditASCIIOverrideSurvivesChangesToTheOriginal(t *testing.T) {
	a, tr := ingested(t)
	got, _ := a.Edit(tr.ID, app.Edit{Artist: ptr("عبد الحليم حافظ"), ArtistASCII: ptr("Abdel Halim Hafez")})
	if got.Artist != "Abdel Halim Hafez" || got.OriginalArtist != "عبد الحليم حافظ" {
		t.Fatalf("override not applied: %+v", got)
	}
	// Changing the original must not re-derive an overridden ASCII value.
	got, _ = a.Edit(tr.ID, app.Edit{Artist: ptr("عبد الحليم حافظ ")})
	if got.Artist != "Abdel Halim Hafez" {
		t.Errorf("override lost: Artist = %q", got.Artist)
	}
	// An empty override goes back to deriving.
	got, _ = a.Edit(tr.ID, app.Edit{ArtistASCII: ptr("")})
	if got.Artist != "'Bd al-Hlim Hafz" {
		t.Errorf("Artist = %q, want derived value", got.Artist)
	}
	// And derived values follow the original again.
	got, _ = a.Edit(tr.ID, app.Edit{Artist: ptr("فيروز")})
	if got.Artist != "Firuz" {
		t.Errorf("Artist = %q, want Firuz", got.Artist)
	}
}

func TestEditClearingAlbumClearsBothForms(t *testing.T) {
	a, tr := ingested(t)
	a.Edit(tr.ID, app.Edit{Album: ptr("Sleep")})
	got, _ := a.Edit(tr.ID, app.Edit{Album: ptr("")})
	if got.Album != "" || got.OriginalAlbum != "" {
		t.Errorf("album = %q / %q, want both empty", got.Album, got.OriginalAlbum)
	}
}

func TestEditTrims(t *testing.T) {
	a, tr := ingested(t) // about 2.612s of audio
	got, err := a.Edit(tr.ID, app.Edit{TrimStart: ptr(500 * time.Millisecond), TrimEnd: ptr(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if got.TrimStart != 500*time.Millisecond || got.TrimEnd != time.Second {
		t.Errorf("trims = %v, %v", got.TrimStart, got.TrimEnd)
	}
	for name, e := range map[string]app.Edit{
		"negative start":     {TrimStart: ptr(-time.Second)},
		"negative end":       {TrimEnd: ptr(-time.Second)},
		"trims cover it all": {TrimStart: ptr(2 * time.Second), TrimEnd: ptr(time.Second)},
		"start past the end": {TrimStart: ptr(time.Hour)},
	} {
		if _, err := a.Edit(tr.ID, e); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if saved := tracks(t, a)[0]; saved.TrimStart != 500*time.Millisecond || saved.TrimEnd != time.Second {
		t.Errorf("a rejected edit changed the record: %+v", saved)
	}
}

func TestEditUnknownID(t *testing.T) {
	a, _ := ingested(t)
	if _, err := a.Edit("nope", app.Edit{Title: ptr("x")}); err == nil {
		t.Error("edit of unknown id succeeded")
	}
}
