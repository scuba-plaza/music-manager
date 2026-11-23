package manifest_test

import (
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
