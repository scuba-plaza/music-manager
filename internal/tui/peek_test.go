package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// The right-hand pane shows the track under the cursor, or for anything that
// holds tracks, what is inside it, as in a file manager: the tree is for
// moving about and the pane is for seeing what is where.

// rightPane is the text of the right-hand pane, row by row.
func rightPane(m Model) []string {
	left := computeLayout(m.w, m.h).left
	var out []string
	for _, l := range strings.Split(screen(m), "\n") {
		if cells := []rune(l); len(cells) > left {
			out = append(out, string(cells[left:]))
		}
	}
	return out
}

// inPane checks that the pane has a row matching each pattern, in this order.
func inPane(t *testing.T, m Model, patterns ...string) {
	t.Helper()
	rows, at := rightPane(m), 0
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		found := false
		for ; at < len(rows); at++ {
			if re.MatchString(rows[at]) {
				found, at = true, at+1
				break
			}
		}
		if !found {
			t.Errorf("the pane has no row matching %q (after the previous ones):\n%s", p, strings.Join(rows, "\n"))
			return
		}
	}
}

func TestAnArtistRowShowsItsAlbumsAndSingles(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "down", "down", "down") // Shawn James, closed
	notInTree(t, m, "Shadows")             // not in the tree...
	inPane(t, m, `Shawn James`, `3 tracks`, `Shadows\s+3\s+0:\d\d`)

	m = keys(t, m, "up", "up", "up", "down") // Fairuz: one single
	inPane(t, m, `Fairuz`, `Singles\s+1\s+0:02`)
}

func TestAnAlbumRowListsItsTracksInOrderWithTheirNumbers(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "down", "down", "down", "enter", "down") // open the artist, then the album Shadows
	if got := selectedLabel(t, m); got != "Shadows" {
		t.Fatalf("setup: %q", got)
	}
	notInTree(t, m, "Flow") // the album is closed
	inPane(t, m, `Shadows`, `Shawn James · 3 tracks`, ` 1\s+The Wanderer\s+0:02`, ` 7\s+Flow\s+0:02`, `12\s+Along Our Way\s+0:02`)
}

func TestTheNeedsWorkGroupListsWhatIsMissing(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	inPane(t, m, `Needs work`, `not been identified`, `af\s+no title, no artist\s+0:02`)
}

func TestAPlaylistListsItsTracksInPlayOrderWithTheirArtists(t *testing.T) {
	lib, _ := playlistLib(t)
	m := newTestModel(t, lib, 120, 36)
	m = keys(t, m, "right", "down") // the playlist Study, closed
	if got := selectedLabel(t, m); got != "Study" {
		t.Fatalf("setup: %q", got)
	}
	notInTree(t, m, "Dream")
	inPane(t, m, `Study`, `2 tracks`, `Playlists/Study.m3u`, `1\s+Flow\s+Shawn James\s+0:02`, `2\s+Dream\s+Max Richter\s+0:02`)
}

func TestThePlaylistsGroupListsThePlaylists(t *testing.T) {
	lib, _ := playlistLib(t)
	m := newTestModel(t, lib, 120, 36)
	if got := selectedLabel(t, m); got != "Playlists" {
		t.Fatalf("setup: %q", got)
	}
	inPane(t, m, `Playlists`, `Sync writes`, `Study\s+2\s+0:0\d`)
}

func TestALongListEndsWithHowManyMoreThereAre(t *testing.T) {
	a := testApp(t)
	var specs []seedTrack
	for i := 1; i <= 20; i++ {
		specs = append(specs, seedTrack{title: fmt.Sprintf("Song %02d", i), artist: "Band", album: "Long", n: i, of: 20})
	}
	seed(t, a, specs...)
	m := newTestModel(t, NewLibrary(a), 120, 24)
	m = keys(t, m, "enter", "down") // open the artist, then the album
	if got := selectedLabel(t, m); got != "Long" {
		t.Fatalf("setup: %q", got)
	}
	inPane(t, m, `Long`, `Song 01`, `Song 02`)
	rows := strings.Join(rightPane(m), "\n")
	if !regexp.MustCompile(`… \d+ more`).MatchString(rows) {
		t.Errorf("no count of the tracks that do not fit:\n%s", rows)
	}
	if strings.Contains(rows, "Song 20") {
		t.Errorf("the pane shows the last track although it cannot fit:\n%s", rows)
	}
}

func TestThePaneIsNamedForWhatItShows(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	title := func() string { return strings.Split(screen(m), "\n")[1] }
	if !strings.Contains(title(), "Needs work") {
		t.Errorf("on the group: %q", title())
	}
	m = keys(t, m, "down", "down", "down")
	if !strings.Contains(title(), "Artist") {
		t.Errorf("on an artist: %q", title())
	}
	m = keys(t, m, "enter", "down")
	if !strings.Contains(title(), "Album") {
		t.Errorf("on an album: %q", title())
	}
	m = keys(t, m, "enter", "down")
	if !strings.Contains(title(), "Details") {
		t.Errorf("on a track: %q", title())
	}
}

// notInTree checks that none of the labels is a row of the tree (the pane on
// the right may well name them).
func notInTree(t *testing.T, m Model, labels ...string) {
	t.Helper()
	for _, l := range labels {
		if treeLine(m, l) >= 0 {
			t.Errorf("the tree has a row %q:\n%s", l, screen(m))
		}
	}
}
