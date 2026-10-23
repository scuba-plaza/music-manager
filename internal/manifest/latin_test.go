package manifest_test

import (
	"path/filepath"
	"testing"

	"music-manager/internal/manifest"
)

// A name may have a Latin form set by hand; where it has none, the form is
// worked out from the true name.

func TestALatinFormSetByHandWinsOverTheWorkedOutOne(t *testing.T) {
	tr := manifest.Track{
		Title: "وطني", Artist: "فيروز", Album: "Hits", Genre: "طرب",
		ArtistLatin: "Fairouz", GenreLatin: "Tarab",
	}
	for _, c := range []struct{ field, got, want string }{
		{"title", tr.LatinTitle(), "Wtni"}, // worked out
		{"artist", tr.LatinArtist(), "Fairouz"},
		{"album", tr.LatinAlbum(), "Hits"},
		{"genre", tr.LatinGenre(), "Tarab"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.field, c.got, c.want)
		}
	}
}

func TestANameWithNothingInItHasNoLatinForm(t *testing.T) {
	tr := manifest.Track{Title: "Song", Artist: "Band"}
	if tr.LatinAlbum() != "" || tr.LatinGenre() != "" {
		t.Errorf("album %q, genre %q; want none", tr.LatinAlbum(), tr.LatinGenre())
	}
}

func TestPlaylistsHaveALatinNameToo(t *testing.T) {
	p := manifest.Playlist{Name: "دراسة"}
	if got := p.LatinName(); got != "Drasa" {
		t.Errorf("worked out = %q", got)
	}
	p.NameLatin = "Study"
	if got := p.LatinName(); got != "Study" {
		t.Errorf("set by hand = %q", got)
	}
}

func TestLatinFormsAreSavedAndOnlyWhenSet(t *testing.T) {
	store := manifest.NewStore(filepath.Join(t.TempDir(), "library.json"))
	m := &manifest.Manifest{
		Tracks:    []manifest.Track{{ID: "aaaaaa", Title: "وطني", TitleLatin: "Watani", Artist: "A"}},
		Playlists: []manifest.Playlist{{Name: "دراسة", NameLatin: "Study", Tracks: []string{"aaaaaa"}}},
	}
	if err := store.Save(m); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Tracks[0].TitleLatin != "Watani" || got.Playlists[0].NameLatin != "Study" {
		t.Errorf("not saved: %+v %+v", got.Tracks[0], got.Playlists[0])
	}
}

func TestSearchFindsANameByItsLatinFormSetByHandAndByTheWorkedOutOne(t *testing.T) {
	m := &manifest.Manifest{Tracks: []manifest.Track{
		{ID: "a", Status: manifest.Identified, Title: "وطني", Artist: "فيروز", ArtistLatin: "Fairouz"},
		{ID: "b", Status: manifest.Identified, Title: "Cold", Artist: "Nils Frahm"},
	}}
	for query, want := range map[string]int{"fairouz": 1, "firuz": 1, "frahm": 1, "wtni": 1, "nobody": 0} {
		if got := len(m.Search(query)); got != want {
			t.Errorf("Search(%q) found %d, want %d", query, got, want)
		}
	}
}
