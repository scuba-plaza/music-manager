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
	return a.Fetch(context.Background(), app.AddRequest{URL: link, Source: webtest.FakeSource{Found: webtest.SoWhat()}}, &webtest.FakeDownloader{})
}

func TestFakeDownloaderBringsNewAudioForAnyOrdinaryLink(t *testing.T) {
	d, err := fetch(t, "https://www.youtube.com/watch?v=abc")
	if err != nil {
		t.Fatal(err)
	}
	if d.Match == nil || d.Match.Title != "So What" {
		t.Errorf("match = %+v, want the fake source's first candidate", d.Match)
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

func TestAFakeSourceWithNothingToFindFindsNothing(t *testing.T) {
	got, err := webtest.FakeSource{}.Search(context.Background(), app.Query{Title: "So What"})
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("Search = %#v, %v, want an empty list", got, err)
	}
}

func TestSoWhatIsTwoCandidatesAndACallerCannotChangeThem(t *testing.T) {
	first := webtest.SoWhat()
	if len(first) != 2 || first[0].Title != "So What" || first[0].Score <= first[1].Score {
		t.Fatalf("%+v", first)
	}
	first[0].Title = "changed"
	if webtest.SoWhat()[0].Title != "So What" {
		t.Error("a caller changed the candidates everyone gets")
	}
}
