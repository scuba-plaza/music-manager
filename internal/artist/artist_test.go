package artist_test

import (
	"testing"

	"music-manager/internal/artist"
)

func TestFirstIsTheFirstArtistATagNames(t *testing.T) {
	cases := map[string]string{
		"Fairuz":                     "Fairuz",
		"Fairuz feat. Ziad Rahbani":  "Fairuz",
		"Fairuz (ft. Ziad Rahbani)":  "Fairuz", // the bracket goes with the separator
		"Fairuz FEATURING Ziad":      "Fairuz",
		"Max Richter; Daniel Hope":   "Max Richter",
		"Max Richter\x00Daniel Hope": "Max Richter",
		"Simon & Garfunkel":          "Simon & Garfunkel", // a band, not two artists
		"Earth, Wind & Fire":         "Earth, Wind & Fire",
		"; only the second":          "; only the second", // nothing before the separator: keep it all
		"":                           "",
		"  Fairuz  ":                 "Fairuz",
	}
	for in, want := range cases {
		if got := artist.First(in); got != want {
			t.Errorf("First(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLooksLikeSeveralPointsOutNamesThatMayBeMoreThanOneArtist(t *testing.T) {
	several := []string{"Simon & Garfunkel", "Earth, Wind & Fire", "Fairuz و Ziad", "A x B", "A + B", "A / B", "A vs. B", "A feat. B", "A FT. B", "A; B"}
	for _, s := range several {
		if !artist.LooksLikeSeveral(s) {
			t.Errorf("LooksLikeSeveral(%q) = false", s)
		}
	}
	one := []string{"Fairuz", "Max Richter", "AC/DC", "Alexander", "Andy", "Wu-Tang Clan", "Hozier", ""}
	for _, s := range one {
		if artist.LooksLikeSeveral(s) {
			t.Errorf("LooksLikeSeveral(%q) = true", s)
		}
	}
}
