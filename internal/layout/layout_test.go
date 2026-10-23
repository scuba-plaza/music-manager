package layout_test

import (
	"fmt"
	"path"
	"strings"
	"testing"
	"time"

	"music-manager/internal/layout"
	"music-manager/internal/manifest"
	"music-manager/internal/translit"
)

// rules are the ones of the one target that needs them today: ASCII names, a
// component of at most 100 bytes, 22 characters before ".mp3".
var rules = layout.Rules{Fold: translit.ToASCII, ComponentMax: 100, StemMax: 22}

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
		{"unidentified", manifest.Track{ID: "k7f2qd", Status: manifest.Unidentified, Hint: manifest.Hint{Filename: "01 - Bahlam Bek"}}, "_Inbox/k7f2qd - 01 - Bahlam.mp3"},
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

func TestPathForOutputIsASCIIOnlyForASCIIFields(t *testing.T) {
	// Non-ASCII in the ASCII fields is a bug upstream, but the layout must still
	// never emit it, because the target player cannot show it.
	got := layout.PathFor(identified("Bjørn", "", "Tïtle", 0, 0), rules)
	for _, r := range got {
		if r > 127 {
			t.Errorf("non-ASCII rune %q in %q", r, got)
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
		if got := layout.PlaylistPath(name, rules); got != want {
			t.Errorf("PlaylistPath(%q) = %q, want %q", name, got, want)
		}
	}
}

// The player's index keeps 26 characters of a file name, the extension
// included, and cuts the rest: so names are shortened here, where they are
// whole words, instead of there.

func TestFileNamesAreShortenedToWhatThePlayerKeeps(t *testing.T) {
	cases := []struct {
		name string
		in   manifest.Track
		want string
	}{
		{"fits exactly", identified("A", "", "Exactly Twenty Two Chr", 0, 0), "A/Exactly Twenty Two Chr.mp3"},
		{"cut after a whole word", identified("A", "", "A Very Long Song Title Indeed Yes", 0, 0), "A/A Very Long Song Title.mp3"},
		{"the number is kept and leaves less room", identified("A", "B", "The Long and Winding Road", 3, 8), "A/B/03 - The Long.mp3"},
		{"a parenthesis goes before a word does", identified("A", "B", "Ya mashallah (Remix)", 8, 9), "A/B/08 - Ya mashallah.mp3"},
		{"a word too long for the room is cut in it", identified("A", "B", "Supercalifragilistic", 1, 9), "A/B/01 - Supercalifragilis.mp3"},
		{"no word boundary worth keeping", identified("A", "B", "Hi Supercalifragilistic", 1, 9), "A/B/01 - Hi Supercalifragi.mp3"},
		{"a wide number takes more room", identified("A", "B", "The Long and Winding Road", 7, 120), "A/B/007 - The Long.mp3"},
		{"no dangling punctuation", identified("A", "", "Twenty Characters Fit, Then More", 0, 0), "A/Twenty Characters Fit.mp3"},
		{"only a parenthesis is better than nothing", identified("A", "", "(Instrumental Version Of It)", 0, 0), "A/(Instrumental Version.mp3"},
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

func TestACollisionSuffixStaysWithinWhatThePlayerKeeps(t *testing.T) {
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
		"aaaaaa": "Artist/A Very Long Song Title.mp3",
		"bbbbbb": "Artist/A Very Long Song (2).mp3",
		"cccccc": "Artist/A Very Long Song (3).mp3",
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("Assign[%s] = %q, want %q", id, got[id], w)
		}
	}
}

func TestNoFileNameIsLongerThanThePlayerKeeps(t *testing.T) {
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
		if name := path.Base(p); len(name) > 26 {
			t.Errorf("%s: file name %q is %d characters, the player keeps 26", id, name, len(name))
		}
	}
}
