package layout_test

import (
	"fmt"
	"path"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"music-manager/internal/layout"
	"music-manager/internal/manifest"
	"music-manager/internal/translit"
)

// rules are the ones of the one target that needs them today: ASCII names, a
// component of at most 100 bytes, 24 characters before ".mp3".
var rules = layout.Rules{Fold: translit.ToASCII, ComponentMax: 100, StemMax: 24}

func identified(artist, album, title string, n, total int) manifest.Track {
	return manifest.Track{
		ID: "abc123", Status: manifest.Identified,
		Artist: artist, Album: album, Title: title, TrackNumber: n, TrackTotal: total,
	}
}

func TestPathFor(t *testing.T) {
	cases := []struct {
		name string
		in   manifest.Track
		want string
	}{
		{"album track", identified("Max Richter", "Sleep", "Dream 3", 3, 8), "Max Richter/Sleep/03 - Dream 3.mp3"},
		{"two-digit number", identified("A", "B", "C", 12, 20), "A/B/12 - C.mp3"},
		{"wide enough for the album size", identified("A", "B", "C", 7, 120), "A/B/007 - C.mp3"},
		{"album without track number", identified("A", "B", "C", 0, 0), "A/B/C.mp3"},
		{"single", identified("Fairuz", "", "Wtni", 0, 0), "Fairuz/Wtni.mp3"},
		{"track number without album is ignored", identified("Fairuz", "", "Wtni", 4, 0), "Fairuz/Wtni.mp3"},
		{"unsafe characters", identified(`AC/DC`, `Who? What: "Now"`, `a<b>c|d*e\f`, 1, 1), "AC-DC/Who- What- -Now-/01 - a-b-c-d-e-f.mp3"},
		{"dots and spaces at the ends", identified("  Artist.. ", ". Album .", "Title ", 0, 0), "Artist/Album/Title.mp3"},
		{"leading apostrophe from romanised ayn", identified("'Bd al-Hlim", "", "Song'", 0, 0), "Bd al-Hlim/Song.mp3"},
		{"inner apostrophes stay", identified("Ma'a", "", "Don't", 0, 0), "Ma'a/Don't.mp3"},
		{"empty after cleaning", identified("???", "", "...", 0, 0), "Unknown/Unknown.mp3"},
		{"unidentified", manifest.Track{ID: "k7f2qd", Status: manifest.Unidentified, Hint: manifest.Hint{Filename: "01 - Bahlam Bek"}}, "_Inbox/k7f2qd - 01 - Bahlam Bek.mp3"},
		{"unidentified with non-ascii hint", manifest.Track{ID: "k7f2qd", Status: manifest.Unidentified, Hint: manifest.Hint{Filename: "وطني"}}, "_Inbox/k7f2qd - Wtni.mp3"},
		{"unidentified without hint", manifest.Track{ID: "k7f2qd", Status: manifest.Unidentified}, "_Inbox/k7f2qd.mp3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := layout.PathFor(c.in, rules); got != c.want {
				t.Errorf("PathFor = %q, want %q", got, c.want)
			}
		})
	}
}

func TestPathForKeepsEveryComponentShort(t *testing.T) {
	long := strings.Repeat("x", 400)
	got := layout.PathFor(identified(long, long, long, 1, 1), rules)
	for _, part := range strings.Split(got, "/") {
		if len(part) > 120 {
			t.Errorf("component of %d bytes in %q", len(part), got)
		}
	}
	if !strings.HasSuffix(got, ".mp3") {
		t.Errorf("lost the extension: %q", got)
	}
}

func TestPathForFoldsTheNamesWhenTheRulesSay(t *testing.T) {
	got := layout.PathFor(identified("Bjørn", "", "Tïtle", 0, 0), rules)
	for _, r := range got {
		if r > 127 {
			t.Errorf("non-ASCII rune %q in %q", r, got)
		}
	}
	if got := layout.PathFor(identified("فيروز", "Hits", "وطني", 3, 12), rules); got != "Firuz/Hits/03 - Wtni.mp3" {
		t.Errorf("folded path = %q", got)
	}
}

func TestPathForKeepsTheTrueNamesWithoutAFold(t *testing.T) {
	got := layout.PathFor(identified("فيروز", "روائع", "وطني", 3, 12), layout.Rules{})
	if want := "فيروز/روائع/03 - وطني.mp3"; got != want {
		t.Errorf("PathFor = %q, want %q", got, want)
	}
}

func TestPathForNeverCutsACharacterInHalf(t *testing.T) {
	long := strings.Repeat("ع", 200) // 400 bytes
	for _, r := range []layout.Rules{{}, {ComponentMax: 101}} {
		got := layout.PathFor(identified(long, long, long, 1, 1), r)
		limit := 255
		if r.ComponentMax > 0 {
			limit = r.ComponentMax
		}
		for _, part := range strings.Split(got, "/") {
			if !utf8.ValidString(part) {
				t.Errorf("%+v: %q is cut inside a character", r, part)
			}
			if len(part) > limit+len(".mp3")+len("01 - ") {
				t.Errorf("%+v: component of %d bytes", r, len(part))
			}
		}
	}
}

func TestAssignGivesCollidingTracksDistinctStablePaths(t *testing.T) {
	mk := func(id string, added int) manifest.Track {
		tr := identified("A", "B", "Same", 1, 2)
		tr.ID = id
		tr.AddedAt = time.Unix(int64(added), 0)
		return tr
	}
	older, newer, third := mk("zzzzzz", 1), mk("aaaaaa", 2), mk("mmmmmm", 3)
	other := identified("A", "B", "Different", 2, 2)
	other.ID = "qqqqqq"

	want := map[string]string{
		"zzzzzz": "A/B/01 - Same.mp3", // the oldest keeps the plain name
		"aaaaaa": "A/B/01 - Same (2).mp3",
		"mmmmmm": "A/B/01 - Same (3).mp3",
		"qqqqqq": "A/B/02 - Different.mp3",
	}
	for _, order := range [][]manifest.Track{{older, newer, third, other}, {other, third, newer, older}} {
		got := layout.Assign(order, rules)
		for id, w := range want {
			if got[id] != w {
				t.Errorf("Assign[%s] = %q, want %q", id, got[id], w)
			}
		}
	}
}

func TestAssignTreatsPathsAsCaseInsensitive(t *testing.T) {
	a, b := identified("Artist", "", "SONG", 0, 0), identified("artist", "", "song", 0, 0)
	a.ID, b.ID = "aaaaaa", "bbbbbb"
	a.AddedAt, b.AddedAt = time.Unix(1, 0), time.Unix(2, 0)
	got := layout.Assign([]manifest.Track{a, b}, rules)
	if strings.EqualFold(got["aaaaaa"], got["bbbbbb"]) {
		t.Errorf("paths collide on a case-insensitive filesystem: %v", got)
	}
}

func TestPlaylistPath(t *testing.T) {
	cases := map[string]string{
		"Study":            "Playlists/Study.m3u",
		"  Study ":         "Playlists/Study.m3u",
		"Road trip: 2024?": "Playlists/Road trip- 2024-.m3u",
		"Café":             "Playlists/Cafe.m3u",
		"???":              "Playlists/Unknown.m3u",
	}
	for name, want := range cases {
		if got := layout.PlaylistPath(name, ".m3u", rules); got != want {
			t.Errorf("PlaylistPath(%q) = %q, want %q", name, got, want)
		}
	}
}

// The player shows the first 24 characters of a name: a file name is cut
// there, so that what is shown is the end of the name and not the start of the
// extension. The number of a track counts, as it is part of the name.

func TestFileNamesAreCutToWhatThePlayerShows(t *testing.T) {
	cases := []struct {
		name string
		in   manifest.Track
		want string
	}{
		{"fits exactly", identified("A", "", "Exactly Twenty Four Char", 0, 0), "A/Exactly Twenty Four Char.mp3"},
		{"cut at 24, whatever is there", identified("A", "", "A Very Long Song Title Indeed Yes", 0, 0), "A/A Very Long Song Title I.mp3"},
		{"the number is kept and leaves less room", identified("A", "B", "Munajat Amir al-Mumineen", 1, 12), "A/B/01 - Munajat Amir al-Mum.mp3"},
		{"a wide number takes more room", identified("A", "B", "The Long and Winding Road", 7, 120), "A/B/007 - The Long and Windi.mp3"},
		{"a space at the cut is not kept", identified("A", "", "Twenty Three Chars Okay More", 0, 0), "A/Twenty Three Chars Okay.mp3"},
		{"a dot at the cut is not kept", identified("A", "", "Twenty Three Chars Okay. More", 0, 0), "A/Twenty Three Chars Okay.mp3"},
		{"the folders keep their length", identified("An Artist With A Rather Long Name", "An Album With A Rather Long Name", "S", 0, 0), "An Artist With A Rather Long Name/An Album With A Rather Long Name/S.mp3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := layout.PathFor(c.in, rules); got != c.want {
				t.Errorf("PathFor = %q, want %q", got, c.want)
			}
		})
	}
}

func TestACollisionSuffixStaysWithinWhatThePlayerShows(t *testing.T) {
	mk := func(id string, added int, title string) manifest.Track {
		tr := identified("Artist", "", title, 0, 0)
		tr.ID = id
		tr.AddedAt = time.Unix(int64(added), 0)
		return tr
	}
	got := layout.Assign([]manifest.Track{
		mk("aaaaaa", 1, "A Very Long Song Title Indeed Yes"),
		mk("bbbbbb", 2, "A Very Long Song Title Indeed No"),
		mk("cccccc", 3, "A Very Long Song Title Indeed Maybe"),
	}, rules)
	want := map[string]string{
		"aaaaaa": "Artist/A Very Long Song Title I.mp3",
		"bbbbbb": "Artist/A Very Long Song Tit (2).mp3",
		"cccccc": "Artist/A Very Long Song Tit (3).mp3",
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("Assign[%s] = %q, want %q", id, got[id], w)
		}
	}
}

func TestNoFileNameIsLongerThanThePlayerShows(t *testing.T) {
	title := "Qasidat al-Hubb wa al-Hanin ila al-Watan al-Ghali (Live in Beirut)"
	var tracks []manifest.Track
	for i := 0; i < 12; i++ { // twelve that all shorten to the same name
		tr := identified("Artist", "", fmt.Sprintf("%s %d", title, i), 0, 0)
		tr.ID = fmt.Sprintf("id%04d", i)
		tr.AddedAt = time.Unix(int64(i), 0)
		tracks = append(tracks, tr)
	}
	tracks = append(tracks,
		identified("Artist", "Album", title, 12, 120),
		manifest.Track{ID: "k7f2qd", Status: manifest.Unidentified, Hint: manifest.Hint{Filename: title}},
	)
	for id, p := range layout.Assign(tracks, rules) {
		if name := path.Base(p); utf8.RuneCountInString(strings.TrimSuffix(name, ".mp3")) > 24 {
			t.Errorf("%s: file name %q is longer than the 24 characters the player shows", id, name)
		}
	}
}

// A name is the same name whether its accent is typed as one character or as a
// letter and a combining mark; a filesystem that treats them alike (macOS
// does) must not find two files where there is one.

func TestPathForComposesTheNames(t *testing.T) {
	got := layout.PathFor(identified("Beyoncé", "", "Café", 0, 0), layout.Rules{})
	if want := "Beyoncé/Café.mp3"; got != want {
		t.Errorf("PathFor = %q, want %q", got, want)
	}
}

func TestAssignFindsTwoSpellingsOfOneNameCollide(t *testing.T) {
	a, b := identified("Artist", "", "Café", 0, 0), identified("Artist", "", "Café", 0, 0)
	a.ID, b.ID = "aaaaaa", "bbbbbb"
	a.AddedAt, b.AddedAt = time.Unix(1, 0), time.Unix(2, 0)
	got := layout.Assign([]manifest.Track{a, b}, layout.Rules{})
	if want := "Artist/Café (2).mp3"; got["bbbbbb"] != want {
		t.Errorf("Assign[bbbbbb] = %q, want %q", got["bbbbbb"], want)
	}
}
