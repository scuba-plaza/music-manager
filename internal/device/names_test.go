package device_test

import (
	"testing"

	"music-manager/internal/device"
	"music-manager/internal/manifest"
)

const wolfgangWidth = 12

// The player's index keeps 12 characters of an artist and of an album, so what
// mm writes into the tags has to say its piece within them. These are the
// rules of the plan, with its worked examples.

func TestTheArtistTheWolfgangShows(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"short", "Fairouz", "Fairouz"},
		{"exactly the room", "Nils Frahmer", "Nils Frahmer"},
		{"a leading The stays when all fits", "The Cure", "The Cure"},
		{"a leading The goes when it must", "The Wanted Stars", "Wanted Stars"},
		{"a parenthesis goes", "Nils Frahm (Live)", "Nils Frahm"},
		{"various artists", "Various Artists", "Various"},
		{"initial and surname", "Federico Albanese", "F. Albanese"},
		{"initial and surname again", "George Orwell", "G. Orwell"},
		{"keaton", "Keaton Henson", "K. Henson"},
		{"sebastian", "Sebastian Plano", "S. Plano"},
		{"a particle belongs to the surname", "Dalal Abu Amneh", "D. Abu Amneh"},
		{"initials of every other word", "Mohamed Abdel Wahab", "M. A. Wahab"},
		{"initials of every other word, again", "Abdel Halim Hafez", "A. H. Hafez"},
		{"the surname alone when the initial does not fit", "Gustavo Santaolalla", "Santaolalla"},
		{"khairuddin", "Hussein Khairuddin", "Khairuddin"},
		{"a hyphenated particle is one word with its name", "Sayyed Salam al-Hsini", "S. al-Hsini"},
		{"without the particle when even the surname is too long", "Ludwig van Beethoven", "Beethoven"},
		{"one long word is cut in it", "Supercalifragilistic", "Supercalifra"},
		{"an apostrophe is not an initial", "'Bd al-Hlim Hafz", "B. A. Hafz"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := device.ArtistName(c.in, wolfgangWidth); got != c.want {
				t.Errorf("ArtistName(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestTheAlbumTheWolfgangShows(t *testing.T) {
	cases := []struct{ name, in, artist, want string }{
		{"short", "Hits", "Fairouz", "Hits"},
		{"exactly the room", "Songs (Live)", "Fairouz", "Songs (Live)"},
		{"a leading The goes when it must", "The Blue Hour", "Max Richter", "Blue Hour"},
		{"a trailing EP goes", "Fortanach - EP", "Federico Albanese", "Fortanach"},
		{"the artist at the start goes", "Franz Kafka - Audiobooks (Franz Kafka)", "Franz Kafka", "Audiobooks"},
		{"a word of the artist at the start goes", "Nasrallah Speeches (Hassan Nasrallah)", "Hassan Nasrallah", "Speeches"},
		{"cut after a whole word", "Somnambulant Cycles", "Phoria", "Somnambulant"},
		{"the The goes, then the cut", "The Last of Us Part II Soundtrack", "Gustavo Santaolalla", "Last of Us"},
		{"nothing is left dangling", "Caught a Black Rabbit", "Phoria", "Caught"},
		{"a word that is not small stays", "Songs from the Iraqi Resistance", "Various Artists", "Songs from"},
		{"Arabic small words dangle too", "Layali al Uns", "Asmahan", "Layali"},
		{"one long word is cut in it", "Supercalifragilistic", "Someone", "Supercalifra"},
		{"never nothing, when the album is only the artist", "Hassan Nasrallah", "Hassan Nasrallah", "Nasrallah"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := device.AlbumName(c.in, c.artist, wolfgangWidth); got != c.want {
				t.Errorf("AlbumName(%q, %q) = %q, want %q", c.in, c.artist, got, c.want)
			}
		})
	}
}

func TestTheWolfgangShowsTheLatinFormOfTrueNames(t *testing.T) {
	tr := manifest.Track{Status: manifest.Identified, Title: "وطني", Artist: "فيروز", Album: "روائع عبد الحليم"}
	artist, album := device.PlayerTags(tr, wolfgangWidth)
	if artist != "Firuz" {
		t.Errorf("artist = %q, want Firuz", artist)
	}
	if len([]rune(album)) > wolfgangWidth || album == "" {
		t.Errorf("album = %q, want something of at most %d characters", album, wolfgangWidth)
	}

	single := manifest.Track{Status: manifest.Identified, Title: "Dream of Me and You", Artist: "Max Richter"}
	if _, album := device.PlayerTags(single, wolfgangWidth); album != "Dream of Me" {
		t.Errorf("a single's album = %q, want its title shortened as an album", album)
	}
}
