//go:build live

package musicbrainz_test

import (
	"context"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/musicbrainz"
)

// Run with: go test -tags live ./internal/musicbrainz
// It talks to the real MusicBrainz and Cover Art Archive, and so can fail when
// the network is down or the services change.
func TestLiveSearchAndCover(t *testing.T) {
	c := musicbrainz.New("music-manager-test/0.1")
	got, err := c.Search(context.Background(), app.Query{Title: "Dream 3", Artist: "Max Richter"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 || got[0].Score < 90 || got[0].Artist != "Max Richter" || got[0].TrackNumber == 0 {
		t.Fatalf("unexpected result: %+v", got)
	}
	t.Logf("best: %+v", got[0])

	img, err := c.CoverFront(context.Background(), got[0].ReleaseID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("cover: %d bytes", len(img))
}
