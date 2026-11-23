package app_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/testmp3"
)

func listedTracks(n int) []app.ListedTrack {
	var out []app.ListedTrack
	for i := 1; i <= n; i++ {
		out = append(out, app.ListedTrack{URL: fmt.Sprintf("https://youtu.be/v%d", i), Title: fmt.Sprintf("Track %d", i), Length: time.Duration(i) * time.Minute})
	}
	return out
}

func TestFetchAlbumListsTheTracksAndKeepsTheCover(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	dl.listing = app.Listing{Title: "Kind of Blue", Uploader: "Miles Davis - Topic", Tracks: listedTracks(3), Cover: testmp3.JPEG}
	al, err := a.FetchAlbum(context.Background(), "https://youtube.com/playlist?list=PL1", dl)
	if err != nil {
		t.Fatal(err)
	}
	if al.Title != "Kind of Blue" || al.Uploader != "Miles Davis - Topic" || len(al.Tracks) != 3 || al.CoverSHA() == "" {
		t.Errorf("album = %+v", al)
	}
	if len(dl.listCalls) != 1 || dl.listCalls[0].limit != app.MaxAlbumTracks+1 || dl.listCalls[0].url != "https://youtube.com/playlist?list=PL1" {
		t.Errorf("list calls = %+v", dl.listCalls)
	}
}

func TestFetchAlbumAcceptsExactlyThirtyTracks(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	dl.listing = app.Listing{Title: "T", Tracks: listedTracks(30)}
	if _, err := a.FetchAlbum(context.Background(), "u", dl); err != nil {
		t.Fatal(err)
	}
}

func TestFetchAlbumRefusesMoreThanThirtyTracks(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	dl.listing = app.Listing{Title: "T", Tracks: listedTracks(31)}
	_, err := a.FetchAlbum(context.Background(), "u", dl)
	if err == nil || !strings.Contains(err.Error(), "more than 30 tracks") {
		t.Errorf("err = %v", err)
	}
}

func TestFetchAlbumRefusesAnEmptyPlaylist(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	_, err := a.FetchAlbum(context.Background(), "u", dl)
	if err == nil || !strings.Contains(err.Error(), "no tracks to add") {
		t.Errorf("err = %v", err)
	}
}

func TestFetchAlbumPassesOnWhatTheListingSaid(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	boom := errors.New("boom")
	dl.listErr = boom
	_, err := a.FetchAlbum(context.Background(), "https://youtu.be/list", dl)
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "https://youtu.be/list") {
		t.Errorf("err = %v", err)
	}
}

func TestFetchAlbumWithoutACoverStillWorks(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	dl.listing = app.Listing{Title: "T", Tracks: listedTracks(2), Cover: nil}
	al, err := a.FetchAlbum(context.Background(), "u", dl)
	if err != nil || al.CoverSHA() != "" {
		t.Errorf("album = %+v, err = %v", al, err)
	}
}

func albumFixture(t *testing.T) (*app.App, *fakeDownloader, *app.Album) {
	t.Helper()
	a, dl := openLib(t), newDownloader()
	dl.audioFor = func(url string) []byte {
		n := int(url[len(url)-1] - '0')
		return testmp3.Silent(100 + 50*n)
	}
	dl.listing = app.Listing{Title: "Kind of Blue", Uploader: "Miles Davis", Tracks: []app.ListedTrack{
		{URL: "https://example.org/watch?v=1", Title: "A"},
		{URL: "https://example.org/watch?v=2", Title: "B"},
		{URL: "https://example.org/watch?v=3", Title: "C"},
	}, Cover: testmp3.JPEG}
	al, err := a.FetchAlbum(context.Background(), "https://example.org/playlist?list=1", dl)
	if err != nil {
		t.Fatal(err)
	}
	return a, dl, al
}

func noProgress(int, int, string) {}

func TestAddAlbumAddsTheChosenTracksAsOneAlbum(t *testing.T) {
	a, dl, al := albumFixture(t)
	shared := app.Edit{Album: ptr("Kind of Blue"), Artist: ptr("Miles Davis")}
	rep, err := a.AddAlbum(context.Background(), al, []int{1, 3}, shared, dl, noProgress)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Added) != 2 || rep.Duplicates != 0 || len(rep.Failed) != 0 {
		t.Fatalf("report = %+v", rep)
	}
	for i, want := range []struct {
		title string
		n     int
	}{{"A", 1}, {"C", 3}} {
		got := rep.Added[i]
		if got.Title != want.title || got.TrackNumber != want.n || got.TrackTotal != 3 ||
			got.Album != "Kind of Blue" || got.Artist != "Miles Davis" ||
			got.Status != manifest.Identified || got.CoverSHA256 != al.CoverSHA() || got.CoverSHA256 == "" {
			t.Errorf("track %d = %+v", i, got)
		}
	}
	saved := tracks(t, a)
	if len(saved) != 2 || saved[0].ID != rep.Added[0].ID || saved[1].ID != rep.Added[1].ID {
		t.Errorf("manifest = %+v", saved)
	}
}

func TestAddAlbumFillsAnEmptyYearAndGenreFromEachTrack(t *testing.T) {
	a, dl, al := albumFixture(t)
	dl.infoFor = map[string]app.Info{
		"https://example.org/watch?v=1": {Released: "1959-08-17", Genre: "Jazz"},
		"https://example.org/watch?v=2": {Released: "1960"},
	}
	shared := app.Edit{Artist: ptr("Miles Davis"), Released: ptr(""), Genre: ptr("")}
	rep, err := a.AddAlbum(context.Background(), al, []int{1, 2}, shared, dl, noProgress)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Added) != 2 || rep.Added[0].Released != "1959-08-17" || rep.Added[0].Genre != "Jazz" ||
		rep.Added[1].Released != "1960" || rep.Added[1].Genre != "" {
		t.Errorf("added = %+v", rep.Added)
	}

	a2, dl2, al2 := albumFixture(t)
	dl2.infoFor = dl.infoFor
	shared.Released = ptr("2001")
	rep, err = a2.AddAlbum(context.Background(), al2, []int{1, 2}, shared, dl2, noProgress)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Added) != 2 || rep.Added[0].Released != "2001" || rep.Added[1].Released != "2001" {
		t.Errorf("added = %+v", rep.Added)
	}
}

func TestAddAlbumKeepsATrackCoverWhenTheAlbumHasNone(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	dl.cover = testmp3.JPEG
	dl.listing = app.Listing{Title: "T", Tracks: listedTracks(1)}
	al, err := a.FetchAlbum(context.Background(), "https://example.org/playlist?list=1", dl)
	if err != nil {
		t.Fatal(err)
	}
	if al.CoverSHA() != "" {
		t.Fatal("album has a cover")
	}
	rep, err := a.AddAlbum(context.Background(), al, []int{1}, app.Edit{Artist: ptr("X")}, dl, noProgress)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Added) != 1 || rep.Added[0].CoverSHA256 == "" {
		t.Errorf("report = %+v", rep)
	}
}

func TestAddAlbumSkipsAudioThatIsAlreadyInTheLibrary(t *testing.T) {
	a, dl, al := albumFixture(t)
	if _, err := a.Fetch(context.Background(), app.AddRequest{URL: "https://example.org/watch?v=2"}, dl); err != nil {
		t.Fatal(err)
	}
	d, err := a.Fetch(context.Background(), app.AddRequest{URL: "https://example.org/watch?v=2"}, dl)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Finish(d, app.Edit{Title: ptr("B"), Artist: ptr("Miles Davis")}); err != nil {
		t.Fatal(err)
	}
	rep, err := a.AddAlbum(context.Background(), al, []int{1, 2, 3}, app.Edit{Artist: ptr("Miles Davis")}, dl, noProgress)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Duplicates != 1 || len(rep.Added) != 2 || rep.Added[0].Title != "A" || rep.Added[1].Title != "C" || len(rep.Failed) != 0 {
		t.Errorf("report = %+v", rep)
	}
}

func TestAddAlbumGoesOnWhenOneDownloadFails(t *testing.T) {
	a, dl, al := albumFixture(t)
	boom := errors.New("boom")
	dl.errFor = map[string]error{"https://example.org/watch?v=2": boom}
	rep, err := a.AddAlbum(context.Background(), al, []int{1, 2, 3}, app.Edit{Artist: ptr("Miles Davis")}, dl, noProgress)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Failed) != 1 || rep.Failed[0].Title != "B" || !errors.Is(rep.Failed[0].Err, boom) {
		t.Errorf("failed = %+v", rep.Failed)
	}
	if len(rep.Added) != 2 || rep.Added[0].Title != "A" || rep.Added[1].Title != "C" {
		t.Errorf("added = %+v", rep.Added)
	}
}

func TestAddAlbumReportsProgressBeforeEachTrackAndAtTheEnd(t *testing.T) {
	a, dl, al := albumFixture(t)
	type call struct {
		done, total int
		title       string
	}
	var got []call
	_, err := a.AddAlbum(context.Background(), al, []int{1, 2, 3}, app.Edit{Artist: ptr("X")}, dl, func(done, total int, title string) {
		got = append(got, call{done, total, title})
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []call{{0, 3, "A"}, {1, 3, "B"}, {2, 3, "C"}, {3, 3, ""}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("progress = %v, want %v", got, want)
	}
}

func TestAddAlbumAddsNothingWhenCancelled(t *testing.T) {
	a, dl, al := albumFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dl.onDownload = func(url string) {
		if url == "https://example.org/watch?v=2" {
			cancel()
		}
	}
	rep, err := a.AddAlbum(ctx, al, []int{1, 2, 3}, app.Edit{Artist: ptr("X")}, dl, noProgress)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if len(rep.Added) != 0 || rep.Duplicates != 0 || len(rep.Failed) != 0 {
		t.Errorf("report = %+v", rep)
	}
	if saved := tracks(t, a); len(saved) != 0 {
		t.Errorf("manifest = %+v", saved)
	}
}

func TestAddAlbumTakesBackTheTracksAlreadyAddedWhenCancelledWhileFinishing(t *testing.T) {
	a, dl, al := albumFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	progress := func(done, total int, _ string) {
		if done == total {
			cancel()
		}
	}
	rep, err := a.AddAlbum(ctx, al, []int{1, 2, 3}, app.Edit{Artist: ptr("X")}, dl, progress)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if len(rep.Added) != 0 || rep.Duplicates != 0 || len(rep.Failed) != 0 {
		t.Errorf("report = %+v", rep)
	}
	if saved := tracks(t, a); len(saved) != 0 {
		t.Errorf("manifest = %+v", saved)
	}
}

func TestAlbumCheckRefusesBadChoices(t *testing.T) {
	_, _, al := albumFixture(t)
	for _, c := range []struct {
		nums []int
		want string
	}{
		{nil, "pick at least one track"},
		{[]int{}, "pick at least one track"},
		{[]int{0}, "the chosen tracks are not in this playlist"},
		{[]int{4}, "the chosen tracks are not in this playlist"},
		{[]int{2, 1}, "the chosen tracks are not in this playlist"},
		{[]int{1, 1}, "the chosen tracks are not in this playlist"},
		{[]int{1}, ""},
		{[]int{1, 2, 3}, ""},
	} {
		err := al.Check(c.nums)
		if c.want == "" {
			if err != nil {
				t.Errorf("Check(%v) = %v", c.nums, err)
			}
		} else if err == nil || err.Error() != c.want {
			t.Errorf("Check(%v) = %v, want %q", c.nums, err, c.want)
		}
	}
}
