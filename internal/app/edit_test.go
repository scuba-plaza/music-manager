package app_test

import (
	"path/filepath"
	"strings"
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
	if got.Status != manifest.Identified || got.OriginalTitle != "وطني" || got.OriginalArtist != "فيروز" || got.OriginalAlbum != "Hits" ||
		got.TrackNumber != 3 || got.TrackTotal != 12 {
		t.Errorf("got %+v", got)
	}
	if got.Hint != (manifest.Hint{}) {
		t.Errorf("hint not cleared: %+v", got.Hint)
	}
	if saved := tracks(t, a)[0]; saved.OriginalTitle != "وطني" {
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

func TestEditKeepsTheNamesAsTyped(t *testing.T) {
	a, tr := ingested(t)
	got, _ := a.Edit(tr.ID, app.Edit{Artist: ptr("  عبد الحليم حافظ ")})
	if got.OriginalArtist != "عبد الحليم حافظ" {
		t.Errorf("Artist = %q, want the name without the spaces around it", got.OriginalArtist)
	}
	got, _ = a.Edit(tr.ID, app.Edit{Artist: ptr("فيروز")})
	if got.OriginalArtist != "فيروز" {
		t.Errorf("Artist = %q", got.OriginalArtist)
	}
}

func TestEditClearingAlbumClearsIt(t *testing.T) {
	a, tr := ingested(t)
	a.Edit(tr.ID, app.Edit{Album: ptr("Sleep")})
	got, _ := a.Edit(tr.ID, app.Edit{Album: ptr("")})
	if got.OriginalAlbum != "" {
		t.Errorf("album = %q, want empty", got.OriginalAlbum)
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

func TestEditReleaseDate(t *testing.T) {
	a, tr := ingested(t)
	for _, c := range []struct{ in, want string }{
		{"1999", "1999"},
		{"1999-05", "1999-05"},
		{" 1999-05-17 ", "1999-05-17"},
		{"1996-02-29", "1996-02-29"},
		{"", ""},
	} {
		got, err := a.Edit(tr.ID, app.Edit{Released: ptr(c.in)})
		if err != nil || got.Released != c.want {
			t.Errorf("%q: released = %q, %v; want %q", c.in, got.Released, err, c.want)
		}
		if saved := tracks(t, a)[0]; saved.Released != c.want {
			t.Errorf("%q: saved as %q", c.in, saved.Released)
		}
	}
}

func TestEditRejectsDatesThatAreNotDates(t *testing.T) {
	a, tr := ingested(t)
	a.Edit(tr.ID, app.Edit{Released: ptr("2001-02")})
	before := tracks(t, a)[0]
	for _, bad := range []string{"99", "yesterday", "1999-13", "1999-00", "1999-02-30", "1999-5-7", "1999/05/17", "19990517", "0000", "1999-05-17T10:00"} {
		_, err := a.Edit(tr.ID, app.Edit{Released: ptr(bad)})
		if err == nil {
			t.Errorf("%q accepted", bad)
			continue
		}
		if !strings.Contains(err.Error(), "1999-05-17") {
			t.Errorf("%q: error %q does not show the format", bad, err)
		}
	}
	if after := tracks(t, a)[0]; after.Released != "2001-02" || !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Errorf("a rejected edit changed the record: %+v", after)
	}
}

func TestEditingTheReleaseDateCountsAsAnEdit(t *testing.T) {
	a, tr := ingested(t)
	got, err := a.Edit(tr.ID, app.Edit{Released: ptr("1975")})
	if err != nil {
		t.Fatal(err)
	}
	if !got.UpdatedAt.After(tr.UpdatedAt) || !got.AddedAt.Equal(tr.AddedAt) {
		t.Errorf("added %v → %v, updated %v → %v", tr.AddedAt, got.AddedAt, tr.UpdatedAt, got.UpdatedAt)
	}
}

func TestEditSourceURL(t *testing.T) {
	a, tr := ingested(t)
	for _, c := range []struct{ in, want string }{
		{"https://www.youtube.com/watch?v=abc123", "https://www.youtube.com/watch?v=abc123"},
		{"  http://example.org/a  ", "http://example.org/a"},
		{"", ""},
	} {
		got, err := a.Edit(tr.ID, app.Edit{SourceURL: ptr(c.in)})
		if err != nil || got.SourceURL != c.want {
			t.Errorf("%q: source = %q, %v; want %q", c.in, got.SourceURL, err, c.want)
		}
		if saved := tracks(t, a)[0]; saved.SourceURL != c.want {
			t.Errorf("%q: saved as %q", c.in, saved.SourceURL)
		}
	}
}

func TestEditRejectsSourcesThatAreNotLinks(t *testing.T) {
	a, tr := ingested(t)
	a.Edit(tr.ID, app.Edit{SourceURL: ptr("https://youtu.be/xyz")})
	before := tracks(t, a)[0]
	for _, bad := range []string{"youtube.com/watch?v=abc", "watch?v=abc", "ftp://example.org/a", "https://", "not a link", "https://exa mple.org/"} {
		_, err := a.Edit(tr.ID, app.Edit{SourceURL: ptr(bad)})
		if err == nil {
			t.Errorf("%q accepted", bad)
			continue
		}
		if !strings.Contains(err.Error(), "https://") {
			t.Errorf("%q: error %q does not show what a link looks like", bad, err)
		}
	}
	if after := tracks(t, a)[0]; after.SourceURL != "https://youtu.be/xyz" || !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Errorf("a rejected edit changed the record: %+v", after)
	}
}

func TestEditGenre(t *testing.T) {
	a, tr := ingested(t)
	for _, c := range []struct{ in, want string }{
		{"Neoclassical", "Neoclassical"},
		{"  Hip-Hop/Rap  ", "Hip-Hop/Rap"},
		{"", ""},
	} {
		got, err := a.Edit(tr.ID, app.Edit{Genre: ptr(c.in)})
		if err != nil || got.Genre != c.want {
			t.Errorf("%q: genre = %q, %v; want %q", c.in, got.Genre, err, c.want)
		}
		if saved := tracks(t, a)[0]; saved.Genre != c.want {
			t.Errorf("%q: saved as %q", c.in, saved.Genre)
		}
	}
}

func TestOtherEditsLeaveTheGenreAlone(t *testing.T) {
	a, tr := ingested(t)
	if _, err := a.Edit(tr.ID, app.Edit{Genre: ptr("Folk")}); err != nil {
		t.Fatal(err)
	}
	got, err := a.Edit(tr.ID, app.Edit{Title: ptr("Song"), Artist: ptr("Band")})
	if err != nil || got.Genre != "Folk" {
		t.Errorf("genre = %q, %v after editing the title", got.Genre, err)
	}
}

func TestEditRejectsAGenreWithMoreThanOneLine(t *testing.T) {
	a, tr := ingested(t)
	if _, err := a.Edit(tr.ID, app.Edit{Genre: ptr("Folk\nRock")}); err == nil {
		t.Error("accepted a genre with a line break")
	}
}
