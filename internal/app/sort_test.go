package app_test

import (
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// sortSample is four tracks that come in a different order for every key.
func sortSample() []manifest.Track {
	return []manifest.Track{
		{ID: "a", Status: manifest.Identified, AsciiArtist: "Beta", AsciiTitle: "Zulu", AsciiAlbum: "Alpha", TrackNumber: 2,
			SourceLength: 200 * time.Second, Released: "2001", AddedAt: day("2026-01-03"), UpdatedAt: day("2026-03-01")},
		{ID: "b", Status: manifest.Identified, AsciiArtist: "alpha", AsciiTitle: "yankee", AsciiAlbum: "Gamma", TrackNumber: 1,
			SourceLength: 100 * time.Second, Released: "1999-05", AddedAt: day("2026-01-01"), UpdatedAt: day("2026-05-01")},
		{ID: "c", Status: manifest.Identified, AsciiArtist: "Gamma", AsciiTitle: "Xray",
			SourceLength: 300 * time.Second, AddedAt: day("2026-01-02")},
		{ID: "d", Status: manifest.Unidentified, Hint: manifest.Hint{Title: "Whiskey", Artist: "Delta", Album: "Beta Album", Track: 3},
			SourceLength: 150 * time.Second, AddedAt: day("2025-12-31"), UpdatedAt: day("2026-02-01")},
	}
}

func ids(ts []manifest.Track) string {
	var out []string
	for _, t := range ts {
		out = append(out, t.ID)
	}
	return strings.Join(out, "")
}

func TestSortingTracksByEachField(t *testing.T) {
	cases := []struct {
		key     app.SortKey
		reverse bool
		want    string
		why     string
	}{
		{app.SortArtist, false, "badc", "alpha, Beta, Delta (from the old tags), Gamma, ignoring case"},
		{app.SortArtist, true, "cdab", "the same backwards"},
		{app.SortTitle, false, "dcba", "Whiskey, Xray, yankee, Zulu, ignoring case"},
		{app.SortAlbum, false, "adbc", "Alpha, Beta Album, Gamma; a track with no album last"},
		{app.SortAlbum, true, "bdac", "backwards, a track with no album still last"},
		{app.SortTrack, false, "badc", "1, 2, 3 (d's comes from its old tags), then no number"},
		{app.SortLength, false, "bdac", "shortest first"},
		{app.SortLength, true, "cadb", "longest first"},
		{app.SortReleased, false, "abdc", "newest first; unknown last, those ordered by artist"},
		{app.SortReleased, true, "badc", "oldest first; unknown still last"},
		{app.SortAdded, false, "acbd", "most recently added first"},
		{app.SortAdded, true, "dbca", "oldest first"},
		{app.SortEdited, false, "badc", "most recently edited first; never edited last"},
		{app.SortEdited, true, "dabc", "least recently edited first; never edited still last"},
	}
	for _, c := range cases {
		in := sortSample()
		got := ids(app.SortTracks(in, c.key, c.reverse))
		if got != c.want {
			t.Errorf("by %s (reverse %v): %s, want %s (%s)", c.key, c.reverse, got, c.want, c.why)
		}
		if ids(in) != "abcd" {
			t.Errorf("sorting by %s changed the slice it was given: %s", c.key, ids(in))
		}
	}
}

func TestTracksThatTieAreOrderedByArtistAlbumTrackAndTitle(t *testing.T) {
	ts := []manifest.Track{
		{ID: "1", Status: manifest.Identified, AsciiArtist: "B", AsciiAlbum: "X", TrackNumber: 2, AsciiTitle: "t", Released: "2000"},
		{ID: "2", Status: manifest.Identified, AsciiArtist: "B", AsciiAlbum: "X", TrackNumber: 1, AsciiTitle: "t", Released: "2000"},
		{ID: "3", Status: manifest.Identified, AsciiArtist: "A", AsciiAlbum: "Z", TrackNumber: 9, AsciiTitle: "t", Released: "2000"},
		{ID: "4", Status: manifest.Identified, AsciiArtist: "A", AsciiAlbum: "Z", TrackNumber: 9, AsciiTitle: "s", Released: "2000"},
	}
	if got := ids(app.SortTracks(ts, app.SortReleased, false)); got != "4321" {
		t.Errorf("ties came out as %s, want 4321", got)
	}
	if got := ids(app.SortTracks(ts, app.SortReleased, true)); got != "4321" {
		t.Errorf("reversing a tie must not reverse the tie-break, got %s", got)
	}
}

func TestSortKeysHaveNamesAndANaturalDirection(t *testing.T) {
	for name, want := range map[string]app.SortKey{
		"artist": app.SortArtist, "album": app.SortAlbum, "title": app.SortTitle, "track": app.SortTrack,
		"length": app.SortLength, "released": app.SortReleased, "added": app.SortAdded, "edited": app.SortEdited,
	} {
		got, err := app.ParseSortKey(name)
		if err != nil || got != want {
			t.Errorf("ParseSortKey(%q) = %q, %v", name, got, err)
		}
		if got.Label() == "" {
			t.Errorf("%q has no label", name)
		}
	}
	if _, err := app.ParseSortKey("colour"); err == nil || !strings.Contains(err.Error(), "released") {
		t.Errorf("an unknown key should say which exist, got %v", err)
	}
	for _, k := range []app.SortKey{app.SortReleased, app.SortAdded, app.SortEdited} {
		if !k.IsDate() {
			t.Errorf("%s is a date", k)
		}
	}
	for _, k := range []app.SortKey{app.SortArtist, app.SortAlbum, app.SortTitle, app.SortTrack, app.SortLength} {
		if k.IsDate() {
			t.Errorf("%s is not a date", k)
		}
	}
	if len(app.SortKeys) != 8 {
		t.Errorf("%d keys listed, want 8", len(app.SortKeys))
	}
}

func TestSortValueIsWhatTheListShowsBesideATrack(t *testing.T) {
	tr := sortSample()[1]
	for key, want := range map[app.SortKey]string{
		app.SortReleased: "1999-05",
		app.SortAdded:    "2026-01-01",
		app.SortEdited:   "2026-05-01",
		app.SortLength:   "1:40",
		app.SortAlbum:    "Gamma",
		app.SortArtist:   "alpha",
		app.SortTitle:    "yankee",
		app.SortTrack:    "1",
	} {
		if got := app.SortValue(tr, key); got != want {
			t.Errorf("SortValue(%s) = %q, want %q", key, got, want)
		}
	}
	if got := app.SortValue(sortSample()[2], app.SortReleased); got != "" {
		t.Errorf("an unknown release date reads %q, want nothing", got)
	}
}

func init() { time.Local = time.UTC } // dates are shown in local time; the tests do not depend on where they run
