package webtest_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"music-manager/internal/app"
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
