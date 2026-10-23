package device_test

import (
	"testing"

	"music-manager/internal/device"
)

// The player's index keeps 12 characters of an artist and of an album, so what
// mm writes into the tags has to say its piece within them.

func TestPlayerNameKeepsAShortNameAndShortensALongOne(t *testing.T) {
	cases := []struct{ name, full, want string }{
		{"short", "Fairouz", "Fairouz"},
		{"exactly the room", "Songs (Live)", "Songs (Live)"},
		{"after a whole word", "Abdel Halim Hafez", "Abdel Halim"},
		{"a leading The goes when it must", "The Wanted Stars", "Wanted Stars"},
		{"a leading The stays when all fits", "The Cure", "The Cure"},
		{"a parenthesis goes first", "Yawman ma (Remix)", "Yawman ma"},
		{"one long word is cut in it", "Supercalifragilistic", "Supercalifra"},
		{"a first word that is too short to keep", "Julia Boutros", "Julia Boutro"},
		{"nothing is left dangling", "Fairouz and, Co Band", "Fairouz"},
		{"nor a word that needs the next", "Caught a Black Rabbit", "Caught"},
		{"a connector in the middle stays", "Songs of Bay Al Hikma", "Songs of Bay"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := device.PlayerName(c.full, ""); got != c.want {
				t.Errorf("PlayerName(%q) = %q, want %q", c.full, got, c.want)
			}
		})
	}
}

func TestPlayerNameLetsAnOverrideWin(t *testing.T) {
	if got := device.PlayerName("Abdel Halim Hafez", "A. H. Hafez"); got != "A. H. Hafez" {
		t.Errorf("got %q", got)
	}
	if got := device.PlayerName("Fairouz", "Fairuz Official Channel"); got != "Fairuz Official Channel" {
		t.Errorf("an override is the user's word, even when the player will cut it: got %q", got)
	}
	if got := device.PlayerName("Abdel Halim Hafez", "  "); got != "Abdel Halim" {
		t.Errorf("a blank override is none: got %q", got)
	}
}
