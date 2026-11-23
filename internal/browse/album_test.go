package browse

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/testmp3"
)

type listingDownloader struct {
	listing app.Listing
	calls   []string
	lists   int
}

func (d *listingDownloader) Download(ctx context.Context, url, dir string) (app.Downloaded, error) {
	d.calls = append(d.calls, url)
	return app.Downloaded{}, errors.New("no download in this test")
}

func (d *listingDownloader) List(ctx context.Context, url, dir string, limit int) (app.Listing, error) {
	d.lists++
	return d.listing, nil
}

func tracksOf(n int) []app.ListedTrack {
	var out []app.ListedTrack
	for i := 1; i <= n; i++ {
		out = append(out, app.ListedTrack{URL: "https://youtu.be/v", Title: "Song " + string(rune('A'+i-1)), Length: time.Duration(i) * time.Minute})
	}
	return out
}

func albumEditor(t *testing.T, l app.Listing) (*Editor, *listingDownloader) {
	t.Helper()
	_, a, _ := seeded(t)
	dl := &listingDownloader{listing: l}
	return NewEditor(a, WithDownloader(dl)), dl
}

func TestOnlyAPlaylistPageIsAPlaylistLink(t *testing.T) {
	yes := []string{
		"https://www.youtube.com/playlist?list=OLAK5uy_x",
		"https://music.youtube.com/playlist?list=PLabc",
		"https://youtube.com/playlist?list=PLabc&si=zz",
		"https://m.youtube.com/playlist?list=PLabc",
	}
	no := []string{
		"https://www.youtube.com/watch?v=abc&list=PLabc",
		"https://www.youtube.com/playlist",
		"https://www.youtube.com/playlist?list=",
		"https://example.org/playlist?list=PLabc",
		"https://youtu.be/abc?list=PLabc",
		"ftp://www.youtube.com/playlist?list=a",
		"not a link",
	}
	for _, l := range yes {
		if !IsPlaylistLink(l) {
			t.Errorf("%q should be a playlist link", l)
		}
	}
	for _, l := range no {
		if IsPlaylistLink(l) {
			t.Errorf("%q should not be a playlist link", l)
		}
	}
}

func TestTheAlbumFormStartsWithWhatThePlaylistSaid(t *testing.T) {
	f := albumDefaults(&app.Album{Title: "Kind of Blue", Uploader: "Miles Davis - Topic"})
	if f != (AlbumForm{Album: "Kind of Blue", Artist: "Miles Davis"}) {
		t.Errorf("form = %+v", f)
	}
}

func TestTheAlbumFormChecksWhatItMustHave(t *testing.T) {
	_, err := AlbumForm{Released: "x"}.Edit()
	var fields app.FieldErrors
	if !errors.As(err, &fields) {
		t.Fatalf("err = %v", err)
	}
	for _, k := range []string{"album", "artist", "released"} {
		if fields[k] == "" {
			t.Errorf("no error for %s: %v", k, fields)
		}
	}
	if fields["album"] != "Album cannot be empty" || fields["artist"] != "Artist cannot be empty" {
		t.Errorf("fields = %v", fields)
	}
	e, err := AlbumForm{Album: " Kind of Blue ", Artist: "Miles Davis"}.Edit()
	if err != nil {
		t.Fatal(err)
	}
	if e.Album == nil || *e.Album != "Kind of Blue" || e.Artist == nil || *e.Artist != "Miles Davis" {
		t.Errorf("edit = %+v", e)
	}
}

func TestFetchingAPlaylistLinkGivesAnAlbumToReview(t *testing.T) {
	ed, dl := albumEditor(t, app.Listing{Title: "Kind of Blue", Uploader: "Miles Davis - Topic", Tracks: tracksOf(3), Cover: testmp3.JPEG})
	f, err := ed.Fetch(context.Background(), "https://www.youtube.com/playlist?list=OLAK5uy_x")
	if err != nil {
		t.Fatal(err)
	}
	if f.Album == nil || f.Draft != nil || f.Duplicate != nil || f.Album.ID == "" || dl.lists != 1 {
		t.Fatalf("fetched = %+v", f)
	}
	if f.Album.Form != (AlbumForm{Album: "Kind of Blue", Artist: "Miles Davis"}) {
		t.Errorf("form = %+v", f.Album.Form)
	}
	if f.Album.Note != "From the playlist: check the album and the artist. A year or genre left empty is taken from each track." {
		t.Errorf("note = %q", f.Album.Note)
	}
	if len(f.Album.Tracks) != 3 {
		t.Fatalf("tracks = %+v", f.Album.Tracks)
	}
	for i, tr := range f.Album.Tracks {
		if tr.Number != i+1 || tr.Title != tracksOf(3)[i].Title || tr.Seconds != (i+1)*60 {
			t.Errorf("track %d = %+v", i, tr)
		}
	}
	if f.Album.Cover == "" {
		t.Error("no cover")
	}
	sha, ok := ed.DraftCover(f.Album.ID)
	if !ok || sha != f.Album.Cover {
		t.Errorf("DraftCover = %q, %v", sha, ok)
	}
}

func TestAWatchLinkWithAListIsStillOneTrack(t *testing.T) {
	ed, dl := albumEditor(t, app.Listing{})
	_, err := ed.Fetch(context.Background(), "https://www.youtube.com/watch?v=abc&list=PLx")
	if dl.lists != 0 {
		t.Errorf("the playlist was listed %d times", dl.lists)
	}
	if len(dl.calls) != 1 {
		t.Errorf("downloads = %v, err = %v", dl.calls, err)
	}
}

func TestDiscardForgetsAnAlbum(t *testing.T) {
	ed, _ := albumEditor(t, app.Listing{Title: "A", Tracks: tracksOf(1), Cover: testmp3.JPEG})
	f, err := ed.Fetch(context.Background(), "https://www.youtube.com/playlist?list=PLabc")
	if err != nil {
		t.Fatal(err)
	}
	ed.Discard(f.Album.ID)
	if _, ok := ed.DraftCover(f.Album.ID); ok {
		t.Error("the album is still there")
	}
}

func TestAPlaylistOfMoreThanThirtyTracksIsRefusedWithTheAppMessage(t *testing.T) {
	var many []app.ListedTrack
	for i := 0; i < app.MaxAlbumTracks+1; i++ {
		many = append(many, app.ListedTrack{Title: "x"})
	}
	ed, _ := albumEditor(t, app.Listing{Title: "Big", Tracks: many})
	_, err := ed.Fetch(context.Background(), "https://www.youtube.com/playlist?list=PLabc")
	if err == nil || !strings.Contains(err.Error(), "more than 30 tracks") {
		t.Errorf("err = %v", err)
	}
}
