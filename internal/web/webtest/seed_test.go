package webtest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"music-manager/internal/coverimg"
	"music-manager/internal/manifest"
	"music-manager/internal/web/webtest"
)

func describe(t manifest.Track) string {
	if t.Status != manifest.Identified {
		h := t.Hint
		return fmt.Sprintf("%s | %s | %d | %s | %s", h.Artist, h.Album, h.Track, h.Title, t.Status)
	}
	return fmt.Sprintf("%s | %s | %d | %s | %s", t.Artist, t.Album, t.TrackNumber, t.Title, t.Status)
}

func TestSeedBuildsTheUILibrary(t *testing.T) {
	a, err := webtest.Seed(filepath.Join(t.TempDir(), "lib"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := a.Manifest()
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	byTitle := map[string]manifest.Track{}
	artists := map[string]bool{}
	for _, tr := range m.Tracks {
		got = append(got, describe(tr))
		if tr.Status == manifest.Identified {
			byTitle[tr.Title] = tr
			artists[tr.Artist] = true
		}
	}
	slices.Sort(got)
	want := []string{
		"Basement Tapes | Demos | 7 | Rough Mix | unidentified",
		"John Coltrane | Giant Steps | 1 | Giant Steps | identified",
		"Miles Davis | Kind of Blue | 1 | So What | identified",
		"Miles Davis | Kind of Blue | 3 | Blue in Green | identified",
		"Nina Simone |  | 0 | Feeling Good | identified",
		"فيروز | Songs | 1 | وطني | identified",
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("tracks\n got %q\nwant %q", got, want)
	}
	if len(artists) != 4 {
		t.Errorf("%d artists in the tree, want 4: %v", len(artists), artists)
	}

	if latin := byTitle["وطني"].TitleLatin; latin != "Watani" {
		t.Errorf("the Latin form of the Arabic title is %q, want Watani", latin)
	}

	for _, title := range []string{"So What", "Blue in Green"} {
		sha := byTitle[title].CoverSHA256
		if sha == "" {
			t.Errorf("%s has no cover", title)
			continue
		}
		data, err := os.ReadFile(a.CoverPath(sha))
		if err != nil {
			t.Errorf("%s: %v", title, err)
			continue
		}
		if _, err := coverimg.Validate(data); err != nil {
			t.Errorf("%s: the cover is not an image: %v", title, err)
		}
	}
	for _, title := range []string{"Giant Steps", "Feeling Good", "وطني"} {
		if sha := byTitle[title].CoverSHA256; sha != "" {
			t.Errorf("%s has the cover %s, want none", title, sha)
		}
	}

	titles := map[string]string{}
	for _, tr := range m.Tracks {
		titles[tr.ID] = describe(tr)
	}
	var playlists []string
	for _, p := range m.Playlists {
		var inside []string
		for _, id := range p.Tracks {
			inside = append(inside, titles[id])
		}
		playlists = append(playlists, fmt.Sprintf("%s: %q", p.Name, inside))
	}
	wantPlaylists := []string{
		`Late night: ["Miles Davis | Kind of Blue | 3 | Blue in Green | identified"]`,
		`A rather long playlist name that goes on and on and on for a while: ["John Coltrane | Giant Steps | 1 | Giant Steps | identified" "فيروز | Songs | 1 | وطني | identified"]`,
	}
	if !slices.Equal(playlists, wantPlaylists) {
		t.Errorf("playlists\n got %q\nwant %q", playlists, wantPlaylists)
	}

	unidentified := 0
	for _, tr := range m.Tracks {
		if tr.Status != manifest.Identified {
			unidentified++
		}
	}
	if unidentified != 1 {
		t.Errorf("%d unidentified tracks, want 1", unidentified)
	}
}
