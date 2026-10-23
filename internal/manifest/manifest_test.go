package manifest_test

import (
	"testing"
	"time"

	"music-manager/internal/manifest"
)

func TestAddAssignsUniqueIDs(t *testing.T) {
	m := &manifest.Manifest{}
	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		id := m.Add(manifest.Track{}).ID
		if len(id) != 6 || seen[id] {
			t.Fatalf("id %q is not a fresh 6-character id", id)
		}
		seen[id] = true
	}
}

func TestGetAndRemove(t *testing.T) {
	m := &manifest.Manifest{}
	a := m.Add(manifest.Track{Title: "A"})
	b := m.Add(manifest.Track{Title: "B"})

	if got, ok := m.Get(b.ID); !ok || got.Title != "B" {
		t.Errorf("Get(%q) = %+v, %v", b.ID, got, ok)
	}
	if _, ok := m.Get("nope"); ok {
		t.Error("Get of unknown id succeeded")
	}
	if !m.Remove(a.ID) || m.Remove(a.ID) {
		t.Error("Remove should succeed once, then report the id is gone")
	}
	if len(m.Tracks) != 1 || m.Tracks[0].ID != b.ID {
		t.Errorf("tracks after remove = %+v", m.Tracks)
	}
}

func TestUpdateReplacesRecordByID(t *testing.T) {
	m := &manifest.Manifest{}
	a := m.Add(manifest.Track{Title: "Old"})
	a.Title = "New"
	if !m.Update(a) {
		t.Fatal("Update reported unknown id")
	}
	if got, _ := m.Get(a.ID); got.Title != "New" {
		t.Errorf("title = %q, want New", got.Title)
	}
	if m.Update(manifest.Track{ID: "nope"}) {
		t.Error("Update of unknown id succeeded")
	}
}

func TestSearch(t *testing.T) {
	m := &manifest.Manifest{}
	m.Add(manifest.Track{Title: "فيروز", Artist: "فيروز", Status: manifest.Identified})
	m.Add(manifest.Track{Title: "Sleep", Artist: "Max Richter", Status: manifest.Identified})
	m.Add(manifest.Track{Status: manifest.Unidentified, Hint: manifest.Hint{Title: "بحلم بيك", Artist: "Abdel Halim", Album: "Rawaeaa", Filename: "01 Bahlam Bek"}})

	cases := []struct {
		query string
		want  int
	}{
		{"", 3},
		{"bahlam", 1},  // hint filename
		{"rawaeaa", 1}, // hint album
		{"بحلم", 1},    // hint title
		{"status:unidentified abdel", 1},
		{"richter", 1},     // case-insensitive
		{"فيروز", 1},       // the true name
		{"firuz", 1},       // its Latin form
		{"firuz sleep", 0}, // every word must match the same track
		{"max sleep", 1},   // words may match different fields
		{"status:unidentified", 1},
		{"status:identified richter", 1},
		{"zzz", 0},
	}
	for _, c := range cases {
		if got := len(m.Search(c.query)); got != c.want {
			t.Errorf("Search(%q) = %d tracks, want %d", c.query, got, c.want)
		}
	}
}

func TestPlayingLengthExcludesTrims(t *testing.T) {
	tr := manifest.Track{SourceLength: 100 * time.Second, TrimStart: 3 * time.Second, TrimEnd: 7 * time.Second}
	if got, want := tr.PlayingLength(), 90*time.Second; got != want {
		t.Errorf("PlayingLength = %v, want %v", got, want)
	}
	over := manifest.Track{SourceLength: 10 * time.Second, TrimStart: 8 * time.Second, TrimEnd: 8 * time.Second}
	if got := over.PlayingLength(); got != 0 {
		t.Errorf("PlayingLength with overlapping trims = %v, want 0", got)
	}
}

func TestSearchFindsAGenreAsAWordOrWithTheGenrePrefix(t *testing.T) {
	m := &manifest.Manifest{}
	m.Add(manifest.Track{Title: "Longing", Artist: "Gustavo", Genre: "Neoclassical", Status: manifest.Identified})
	m.Add(manifest.Track{Title: "Folk Song", Artist: "Max", Genre: "Anthem", Status: manifest.Identified})
	m.Add(manifest.Track{Title: "Plain", Artist: "Nobody", Status: manifest.Identified})

	cases := []struct {
		query string
		want  int
	}{
		{"neoclassical", 1}, // a plain word looks at the genre too
		{"genre:neo", 1},    // a start or any part, case-insensitive
		{"genre:ANTHEM", 1},
		{"genre:folk", 0}, // only the genre is looked at after the prefix: not the title
		{"folk", 1},       // a plain word matches the title
		{"genre:neo genre:anthem", 0},
		{"genre:neo longing", 1},
		{"genre:", 3}, // nothing asked: everything
	}
	for _, c := range cases {
		if got := len(m.Search(c.query)); got != c.want {
			t.Errorf("Search(%q) = %d tracks, want %d", c.query, got, c.want)
		}
	}
}
