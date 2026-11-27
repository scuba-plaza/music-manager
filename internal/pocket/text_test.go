package pocket

import (
	"testing"
	"unicode/utf8"
)

func TestNamesAreCleanedToTheLettersThePlayerDraws(t *testing.T) {
	cases := []struct{ in, want string }{
		{"\u00c9xta", "\u00c9xta"},
		{"E\u0301xta", "\u00c9xta"},
		{"Bjo\u0308rk", "Bj\u00f6rk"},
		{"\u0141\u00f3d\u017a", "\u0141\u00f3d\u017a"},
		{"Wu\u2010Tang Clan", "Wu-Tang Clan"},
		{"A\u2011B \u2012 C \u2013 D \u2014 E \u2015 F \u2212 G", "A-B - C - D - E - F - G"},
		{"Don\u2019t \u2018stop\u2019", "Don't 'stop'"},
		{"\u201cLive\u201d \u201eHere\u201f", "\"Live\" \"Here\""},
		{"Wait\u2026", "Wait..."},
		{"A\u00a0B\u202fC\u2009D", "A B C D"},
		{"Soft\u00adware", "Software"},
		{"\u200fAl\u200bbum" + string(rune(0xFEFF)) + "\u200e", "Album"},
		{"\u0218tefan \u021aar\u0103", "Stefan Tar\u0103"},
		{"Song \U0001F3B5", "Song"},
		{"  Two   spaces  ", "Two spaces"},
		{"\u0623\u0639\u0632\u0651 \u0627\u0644\u0646\u0627\u0633", "\u0623\u0639\u0632 \u0627\u0644\u0646\u0627\u0633"},
		{"\u062a\u064e\u0633\u0652\u062c\u0650\u064a\u0644\u064c", "\u062a\u0633\u062c\u064a\u0644"},
		{"\u064a\u0648\u0645\u0627\u064b \u0627\u0644\u0642\u064e\u064a\u0633\u064a", "\u064a\u0648\u0645\u0627 \u0627\u0644\u0642\u064a\u0633\u064a"},
		{"\u06a9\u062a\u0627\u0628 \u06cc", "\u0643\u062a\u0627\u0628 \u064a"},
		{"\u06f1\u06f2\u06f3", "\u0661\u0662\u0663"},
		{"\u0671\u0644\u0644\u0647", "\u0627\u0644\u0644\u0647"},
		{"\u0644\u0627 \u0640 \u061f \u060c \u061b \u0662\u0660\u0662\u0665", "\u0644\u0627 \u0640 \u061f \u060c \u061b \u0662\u0660\u0662\u0665"},
	}
	for _, c := range cases {
		if got := clean(c.in); got != c.want {
			t.Errorf("clean(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEveryCharacterCleansToCharactersThePlayerDraws(t *testing.T) {
	for r := rune(0); r <= 0x2FFFF; r++ {
		if !utf8.ValidRune(r) {
			continue
		}
		for _, out := range clean("a" + string(r) + "b") {
			if !drawable(out) {
				t.Fatalf("clean(%U) holds %U, which the player cannot draw", r, out)
			}
		}
	}
}

func TestNamesOnTheCardAreClean(t *testing.T) {
	lib := sample()
	lib.Tracks[0].Title = "Wu\u2010Tang \u2018Forever\u2019"
	lib.Tracks[0].Artist = "Bjo\u0308rk\u200f"
	files := Files(lib)
	if got := record(files, "TRACKS.BIN", 24, 0).str(); got != "Wu-Tang 'Forever'" {
		t.Errorf("title = %q", got)
	}
	found := false
	for i := range 4 {
		if record(files, "ARTISTS.BIN", 24, i).str() == "Bj\u00f6rk" {
			found = true
		}
	}
	if !found {
		t.Errorf("no artist named %q", "Bj\u00f6rk")
	}
}
