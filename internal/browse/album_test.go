package browse

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/testmp3"
)

type listingDownloader struct {
	listing  app.Listing
	calls    []string
	lists    int
	listURLs []string
	audio    map[string][]byte
	errs     map[string]error
	infos    map[string]app.Info
}

func (d *listingDownloader) Download(ctx context.Context, url, dir string) (app.Downloaded, error) {
	d.calls = append(d.calls, url)
	if err := d.errs[url]; err != nil {
		return app.Downloaded{}, err
	}
	audio, ok := d.audio[url]
	if !ok {
		return app.Downloaded{}, errors.New("no download in this test")
	}
	path := filepath.Join(dir, "vid.mp3")
	if err := os.WriteFile(path, audio, 0o644); err != nil {
		return app.Downloaded{}, err
	}
	return app.Downloaded{Path: path, Info: d.infos[url]}, nil
}

func (d *listingDownloader) List(ctx context.Context, url, dir string, limit int) (app.Listing, error) {
	d.lists++
	d.listURLs = append(d.listURLs, url)
	return d.listing, nil
}

func trackURL(i int) string { return "https://youtu.be/v" + string(rune('A'+i-1)) }

func tracksOf(n int) []app.ListedTrack {
	var out []app.ListedTrack
	for i := 1; i <= n; i++ {
		out = append(out, app.ListedTrack{URL: trackURL(i), Title: "Song " + string(rune('A'+i-1)), Length: time.Duration(i) * time.Minute})
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
		"https://WWW.YouTube.com/playlist?list=PLabc",
		"https://music.youtube.com:443/playlist?list=PLabc",
		"https://www.youtube.com/watch?v=abc&list=PLabc",
		"https://music.youtube.com/watch?v=abc&list=OLAK5uy_x&index=3",
		"https://www.youtube.com/watch?list=PLabc",
	}
	no := []string{
		"https://www.youtube.com/watch?v=abc",
		"https://www.youtube.com/watch?v=abc&list=",
		"https://www.youtube.com/shorts/abc?list=PLabc",
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

func TestAWatchLinkWithAListIsTheWholePlaylist(t *testing.T) {
	ed, dl := albumEditor(t, app.Listing{Title: "Kind of Blue", Tracks: tracksOf(2)})
	f, err := ed.Fetch(context.Background(), "https://www.youtube.com/watch?v=abc&list=PLx&index=2")
	if err != nil || f.Album == nil || f.Draft != nil {
		t.Fatalf("%+v %v", f, err)
	}
	if len(dl.calls) != 0 {
		t.Errorf("tracks were downloaded: %v", dl.calls)
	}
	if want := []string{"https://www.youtube.com/playlist?list=PLx"}; !slices.Equal(dl.listURLs, want) {
		t.Errorf("listed %v, want %v", dl.listURLs, want)
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

const playlistLink = "https://www.youtube.com/playlist?list=OLAK5uy_x"

func fetchedAlbum(t *testing.T, n int) (*Editor, *app.App, *listingDownloader, string) {
	t.Helper()
	_, a, _ := seeded(t)
	dl := &listingDownloader{
		listing: app.Listing{Title: "Kind of Blue", Uploader: "Miles Davis - Topic", Tracks: tracksOf(n)},
		audio:   map[string][]byte{},
		errs:    map[string]error{},
	}
	for i := 1; i <= n; i++ {
		dl.audio[trackURL(i)] = testmp3.Silent(300 + i)
	}
	ed := NewEditor(a, WithDownloader(dl))
	f, err := ed.Fetch(context.Background(), playlistLink)
	if err != nil {
		t.Fatal(err)
	}
	return ed, a, dl, f.Album.ID
}

func noProgress(int, int, string) {}

var kindOfBlue = AlbumForm{Album: "Kind of Blue", Artist: "Miles Davis"}

func libraryCount(t *testing.T, a *app.App) int {
	t.Helper()
	m, err := a.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	return len(m.Tracks)
}

func TestAddingAnAlbumRecordsOneStepToUndo(t *testing.T) {
	ed, a, _, id := fetchedAlbum(t, 3)
	res, err := ed.AddAlbum(context.Background(), id, kindOfBlue, []int{1, 3}, noProgress)
	if err != nil {
		t.Fatal(err)
	}
	if res.Message != `Added 2 tracks of "Kind of Blue" · u undoes it` || res.Reveal == "" || res.Same {
		t.Errorf("result = %+v", res)
	}
	if h := ed.History(); h.Undo != "addition of 2 tracks" {
		t.Errorf("history = %+v", h)
	}
	if n := libraryCount(t, a); n != 2 {
		t.Fatalf("library has %d tracks", n)
	}
	if _, err := ed.Undo(); err != nil {
		t.Fatal(err)
	}
	if n := libraryCount(t, a); n != 0 {
		t.Errorf("after undo the library has %d tracks", n)
	}
	if _, err := ed.Redo(); err != nil {
		t.Fatal(err)
	}
	if n := libraryCount(t, a); n != 2 {
		t.Errorf("after redo the library has %d tracks", n)
	}
}

func TestTheMessageSaysWhatWasAlreadyThereAndWhatFailed(t *testing.T) {
	ed, _, dl, id := fetchedAlbum(t, 4)
	dl.audio[trackURL(2)] = dl.audio[trackURL(1)]
	dl.errs[trackURL(3)] = errors.New("video unavailable")
	res, err := ed.AddAlbum(context.Background(), id, kindOfBlue, []int{1, 2, 3, 4}, noProgress)
	if err != nil {
		t.Fatal(err)
	}
	want := `Added 2 tracks of "Kind of Blue" · 1 already there · 1 left out: Song C: download ` + trackURL(3) + `: video unavailable · u undoes it`
	if res.Message != want {
		t.Errorf("message = %q", res.Message)
	}
}

func TestAnAlbumOfOnlyFailuresSaysNothingWasAdded(t *testing.T) {
	ed, a, dl, id := fetchedAlbum(t, 2)
	dl.errs[trackURL(1)] = errors.New("gone")
	dl.errs[trackURL(2)] = errors.New("blocked")
	res, err := ed.AddAlbum(context.Background(), id, kindOfBlue, []int{1, 2}, noProgress)
	if err != nil {
		t.Fatal(err)
	}
	want := "Nothing was added · 2 left out: Song A: download " + trackURL(1) + ": gone; Song B: download " + trackURL(2) + ": blocked"
	if res.Message != want || !res.Same || res.Reveal != "" {
		t.Errorf("result = %+v", res)
	}
	if h := ed.History(); h.Undo != "" {
		t.Errorf("history = %+v", h)
	}
	if n := libraryCount(t, a); n != 0 {
		t.Errorf("library has %d tracks", n)
	}
}

func TestCheckAlbumNamesWhatIsWrong(t *testing.T) {
	ed, _, _, id := fetchedAlbum(t, 3)
	if err := ed.CheckAlbum("nope", kindOfBlue, []int{1}); !errors.Is(err, ErrDraftGone) {
		t.Errorf("gone id: %v", err)
	}
	var fields app.FieldErrors
	if err := ed.CheckAlbum(id, AlbumForm{}, []int{1}); !errors.As(err, &fields) || fields["album"] == "" {
		t.Errorf("empty album: %v", err)
	}
	if err := ed.CheckAlbum(id, kindOfBlue, []int{}); err == nil || !strings.Contains(err.Error(), "at least one track") {
		t.Errorf("no numbers: %v", err)
	}
	if err := ed.CheckAlbum(id, kindOfBlue, []int{1, 3}); err != nil {
		t.Errorf("good: %v", err)
	}
	if _, err := ed.AddAlbum(context.Background(), id, kindOfBlue, []int{1}, noProgress); err != nil {
		t.Fatal(err)
	}
}

func TestAnAddedAlbumIsGone(t *testing.T) {
	ed, _, _, id := fetchedAlbum(t, 2)
	if _, err := ed.AddAlbum(context.Background(), id, kindOfBlue, []int{1}, noProgress); err != nil {
		t.Fatal(err)
	}
	if _, err := ed.AddAlbum(context.Background(), id, kindOfBlue, []int{2}, noProgress); !errors.Is(err, ErrDraftGone) {
		t.Errorf("err = %v", err)
	}
}

func TestACancelledAlbumAddsNothingAndStaysForAnotherTry(t *testing.T) {
	ed, a, _, id := fetchedAlbum(t, 2)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ed.AddAlbum(ctx, id, kindOfBlue, []int{1, 2}, noProgress); err == nil {
		t.Fatal("no error")
	}
	if n := libraryCount(t, a); n != 0 {
		t.Errorf("library has %d tracks", n)
	}
	res, err := ed.AddAlbum(context.Background(), id, kindOfBlue, []int{1, 2}, noProgress)
	if err != nil {
		t.Fatal(err)
	}
	if res.Message != `Added 2 tracks of "Kind of Blue" · u undoes it` {
		t.Errorf("message = %q", res.Message)
	}
}
