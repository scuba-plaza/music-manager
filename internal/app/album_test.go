package app_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
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
