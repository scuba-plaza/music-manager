package webtest_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/testmp3"
	"music-manager/internal/web/webtest"
)

func fetch(t *testing.T, link string) (*app.Draft, error) {
	t.Helper()
	a, err := webtest.Seed(filepath.Join(t.TempDir(), "lib"))
	if err != nil {
		t.Fatal(err)
	}
	return a.Fetch(context.Background(), app.AddRequest{URL: link}, &webtest.FakeDownloader{})
}

func TestFakeDownloaderBringsNewAudioForAnyOrdinaryLink(t *testing.T) {
	if _, err := fetch(t, "https://www.youtube.com/watch?v=abc"); err != nil {
		t.Fatal(err)
	}
}

func TestFakeDownloaderBringsAudioTheSeededLibraryHasAlreadyForDuplicateLink(t *testing.T) {
	_, err := fetch(t, webtest.DuplicateLink)
	var dup *app.DuplicateError
	if !errors.As(err, &dup) {
		t.Fatalf("err = %v, want a *app.DuplicateError", err)
	}
	if dup.ID == "" {
		t.Error("the duplicate does not say which track it is")
	}
}

func TestDifferentLinksBringDifferentAudio(t *testing.T) {
	d := &webtest.FakeDownloader{}
	get := func(link string) []byte {
		res, err := d.Download(context.Background(), link, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(res.Path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	first, second, again := get("https://x.test/1"), get("https://x.test/2"), get("https://x.test/1")
	if bytes.Equal(first, second) {
		t.Error("two links brought the same audio")
	}
	if !bytes.Equal(first, again) {
		t.Error("one link brought different audio twice")
	}
	if !bytes.Equal(first, testmp3.Silent(700)) {
		t.Error("the first link no longer brings 700 frames")
	}
}

func TestListHonoursItsLimit(t *testing.T) {
	d := &webtest.FakeDownloader{Listing: webtest.KindOfBlue()}
	l, err := d.List(context.Background(), "https://music.youtube.com/playlist?list=k", t.TempDir(), 3)
	if err != nil || len(l.Tracks) != 3 || l.Title != "Kind of Blue" {
		t.Fatalf("%+v %v", l, err)
	}
	l, err = d.List(context.Background(), webtest.TooLongLink, t.TempDir(), 31)
	if err != nil || len(l.Tracks) != 31 {
		t.Fatalf("%d %v", len(l.Tracks), err)
	}
	if got := d.ListCalls(); len(got) != 2 || got[1] != webtest.TooLongLink {
		t.Errorf("%v", got)
	}
}

func TestHoldBlocksUntilReleasedOrCancelled(t *testing.T) {
	hold := make(chan struct{})
	d := &webtest.FakeDownloader{Hold: hold}
	ctx, cancel := context.WithCancel(context.Background())
	errs := make(chan error, 1)
	go func() {
		_, err := d.Download(ctx, "https://x.test/1", t.TempDir())
		errs <- err
	}()
	select {
	case <-errs:
		t.Fatal("the download did not wait")
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	if err := <-errs; !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
	close(hold)
	if _, err := d.Download(context.Background(), "https://x.test/1", t.TempDir()); err != nil {
		t.Errorf("released: %v", err)
	}
}
