package web

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"music-manager/internal/browse"
	"music-manager/internal/web/webtest"
)

const kindOfBlueLink = "https://music.youtube.com/playlist?list=kob"

func albumFetch(t *testing.T, base, link string) JobView {
	t.Helper()
	var v JobView
	if code := sendInto(t, base, "POST", "/api/add/fetch", obj{"url": link}, &v); code != 200 {
		t.Fatalf("fetch: %d", code)
	}
	return waitJob(t, base, v.ID)
}

func fetchedAlbum(t *testing.T, base, link string) *browse.AlbumView {
	t.Helper()
	done := albumFetch(t, base, link)
	data, _ := json.Marshal(done.Data)
	var out browse.Fetched
	if err := json.Unmarshal(data, &out); err != nil || done.State != jobDone || out.Album == nil {
		t.Fatalf("%+v %s %v", done, data, err)
	}
	noNullCollections(t, "fetched album", data, reflect.TypeFor[browse.Fetched]())
	return out.Album
}

func albumForm() obj {
	return obj{"album": "Kind of Blue", "artist": "Miles Davis", "released": "1959", "genre": "Jazz"}
}

func TestAPlaylistLinkIsFetchedForReviewAndTheChosenTracksAreAdded(t *testing.T) {
	a, ts := seed(t, sample...)
	dl := &webtest.FakeDownloader{Listing: webtest.KindOfBlue()}
	srv := start(t, a, WithDownloader(dl))

	al := fetchedAlbum(t, srv.URL, kindOfBlueLink)
	if len(al.Tracks) != 5 || al.Form.Album != "Kind of Blue" || al.Form.Artist != "Miles Davis" {
		t.Fatalf("%+v", al)
	}
	var v JobView
	if code := sendInto(t, srv.URL, "POST", "/api/add/album", obj{"id": al.ID, "form": albumForm(), "tracks": []int{1, 2, 4}}, &v); code != 200 || v.Kind != "addalbum" {
		t.Fatalf("%d %+v", code, v)
	}
	done := waitJob(t, srv.URL, v.ID)
	data, _ := json.Marshal(done.Data)
	var out ChangeResponse
	json.Unmarshal(data, &out)
	if done.State != jobDone || out.Message != `Added 3 tracks of "Kind of Blue" · u undoes it` {
		t.Fatalf("%+v %s", done, data)
	}
	noNullCollections(t, "album outcome", data, reflect.TypeFor[ChangeResponse]())
	m, _ := a.Manifest()
	var numbers []int
	for _, tr := range m.Tracks {
		if tr.Album == "Kind of Blue" {
			numbers = append(numbers, tr.TrackNumber)
			if tr.TrackTotal != 5 {
				t.Errorf("track %d of %d", tr.TrackNumber, tr.TrackTotal)
			}
		}
	}
	slices.Sort(numbers)
	if !slices.Equal(numbers, []int{1, 2, 4}) || len(m.Tracks) != len(ts)+3 {
		t.Errorf("numbers %v, %d tracks", numbers, len(m.Tracks))
	}
	if code, _ := send(t, srv, "POST", "/api/undo", nil); code != 200 {
		t.Fatal("undo")
	}
	m, _ = a.Manifest()
	if len(m.Tracks) != len(ts) {
		t.Errorf("%d tracks after undo", len(m.Tracks))
	}
}

func TestAnAlbumNeedsAnArtistAnAlbumAndATrack(t *testing.T) {
	a, _ := seed(t, sample...)
	srv := start(t, a, WithDownloader(&webtest.FakeDownloader{Listing: webtest.KindOfBlue()}))
	al := fetchedAlbum(t, srv.URL, kindOfBlueLink)

	code, r := send(t, srv, "POST", "/api/add/album", obj{"id": al.ID, "form": obj{}, "tracks": []int{1}})
	if code != 422 || r.Fields["album"] == "" || r.Fields["artist"] == "" {
		t.Errorf("%d %+v", code, r)
	}
	code, r = send(t, srv, "POST", "/api/add/album", obj{"id": al.ID, "form": albumForm(), "tracks": []int{}})
	if code != 422 || !strings.Contains(r.Error, "at least one track") {
		t.Errorf("%d %+v", code, r)
	}
	code, r = send(t, srv, "POST", "/api/add/album", obj{"id": "nope", "form": albumForm(), "tracks": []int{1}})
	if code != 422 || !strings.Contains(r.Error, "download is gone") {
		t.Errorf("%d %+v", code, r)
	}
}

func TestAWatchLinkWithAListIsTheWholePlaylist(t *testing.T) {
	a, _ := seed(t, sample...)
	dl := &webtest.FakeDownloader{Listing: webtest.KindOfBlue()}
	srv := start(t, a, WithDownloader(dl))
	if al := fetchedAlbum(t, srv.URL, "https://www.youtube.com/watch?v=1&list=PLx&index=2"); len(al.Tracks) != 5 {
		t.Fatalf("%+v", al)
	}
	if got := dl.ListCalls(); len(got) != 1 || got[0] != "https://www.youtube.com/playlist?list=PLx" {
		t.Errorf("listed %v", got)
	}
}

func TestAPlaylistOfMoreThanThirtyTracksIsRefused(t *testing.T) {
	a, _ := seed(t, sample...)
	srv := start(t, a, WithDownloader(&webtest.FakeDownloader{}))
	done := albumFetch(t, srv.URL, webtest.TooLongLink)
	if done.State != jobFailed || !strings.Contains(done.Error, "more than 30 tracks") {
		t.Errorf("%+v", done)
	}
}

func TestTheAlbumCoverIsServedAndDiscardedLikeADraftCover(t *testing.T) {
	a, _ := seed(t, sample...)
	srv := start(t, a, WithDownloader(&webtest.FakeDownloader{Listing: webtest.KindOfBlue()}))
	al := fetchedAlbum(t, srv.URL, kindOfBlueLink)
	code, kind, _ := getBytes(t, srv.URL+"/api/add/"+al.ID+"/cover")
	if code != 200 || kind != "image/jpeg" {
		t.Errorf("%d %q", code, kind)
	}
	if code := sendInto(t, srv.URL, "DELETE", "/api/add/"+al.ID, nil, nil); code != 204 {
		t.Errorf("discard %d", code)
	}
	if code, _, _ := getBytes(t, srv.URL+"/api/add/"+al.ID+"/cover"); code != 404 {
		t.Errorf("after discard %d", code)
	}
}

func TestStoppingAnAlbumDownloadAddsNothing(t *testing.T) {
	a, ts := seed(t, sample...)
	hold := make(chan struct{})
	dl := &webtest.FakeDownloader{Listing: webtest.KindOfBlue(), Hold: hold}
	srv := start(t, a, WithDownloader(dl))
	al := fetchedAlbum(t, srv.URL, kindOfBlueLink)
	var v JobView
	sendInto(t, srv.URL, "POST", "/api/add/album", obj{"id": al.ID, "form": albumForm(), "tracks": []int{1, 2}}, &v)
	if code := sendInto(t, srv.URL, "DELETE", "/api/jobs/"+v.ID, nil, nil); code != 204 {
		t.Fatalf("stop %d", code)
	}
	if done := waitJob(t, srv.URL, v.ID); done.State != jobStopped {
		t.Errorf("%+v", done)
	}
	m, _ := a.Manifest()
	if len(m.Tracks) != len(ts) {
		t.Errorf("%d tracks", len(m.Tracks))
	}
}

func TestOnlyOneAlbumIsAddedAtATime(t *testing.T) {
	a, _ := seed(t, sample...)
	hold := make(chan struct{})
	dl := &webtest.FakeDownloader{Listing: webtest.KindOfBlue(), Hold: hold}
	srv := start(t, a, WithDownloader(dl))
	al := fetchedAlbum(t, srv.URL, kindOfBlueLink)
	body := obj{"id": al.ID, "form": albumForm(), "tracks": []int{1}}
	var v JobView
	if code := sendInto(t, srv.URL, "POST", "/api/add/album", body, &v); code != 200 {
		t.Fatalf("%d", code)
	}
	code, r := send(t, srv, "POST", "/api/add/album", body)
	if code != 409 || r.Error != "An album is being added already" {
		t.Errorf("%d %+v", code, r)
	}
	sendInto(t, srv.URL, "DELETE", "/api/jobs/"+v.ID, nil, nil)
	waitJob(t, srv.URL, v.ID)
}
