package tui

import (
	"strings"
	"testing"
	"time"

	"music-manager/internal/manifest"
)

func album(n int) []manifest.Track {
	var ts []manifest.Track
	for i := 1; i <= n; i++ {
		t := trk(string(rune('a'+i)), "Song "+string(rune('A'+i)), "Shawn James", "Shadows", i)
		t.OriginalArtist, t.OriginalTitle, t.OriginalAlbum = "Shawn James", t.Title, "Shadows"
		t.TrackTotal = n
		ts = append(ts, t)
	}
	return ts
}

func TestBulkBlankKeepsEverything(t *testing.T) {
	edits, changed, err := bulkPlan(album(3), bulkValues{})
	if err != nil || changed != 0 || len(edits) != 0 {
		t.Errorf("edits %v, changed %d, err %v", edits, changed, err)
	}
}

func TestBulkSetsTheSameValueOnEveryTrack(t *testing.T) {
	ts := album(3)
	edits, changed, err := bulkPlan(ts, bulkValues{artist: "Fairuz", album: "Hits", of: "10", cutEnd: "0:03"})
	if err != nil || changed != 3 {
		t.Fatalf("changed %d, err %v", changed, err)
	}
	for _, tr := range ts {
		e := edits[tr.ID]
		if e.Artist == nil || *e.Artist != "Fairuz" || e.Album == nil || *e.Album != "Hits" ||
			e.TrackTotal == nil || *e.TrackTotal != 10 || e.TrimEnd == nil || *e.TrimEnd != 3*time.Second {
			t.Errorf("%s: %+v", tr.ID, e)
		}
		if e.Title != nil || e.TrackNumber != nil || e.TrimStart != nil {
			t.Errorf("%s: fields that were left blank must not be touched: %+v", tr.ID, e)
		}
	}
}

func TestBulkDashClears(t *testing.T) {
	ts := album(2)
	edits, changed, err := bulkPlan(ts, bulkValues{album: "-", of: "-", cutStart: "-"})
	if err != nil || changed != 2 {
		t.Fatalf("changed %d, err %v", changed, err)
	}
	e := edits[ts[0].ID]
	if *e.Album != "" || *e.TrackTotal != 0 || *e.TrimStart != 0 {
		t.Errorf("= %+v", e)
	}
}

func TestBulkSkipsTracksThatAlreadyHaveTheValue(t *testing.T) {
	ts := album(3)
	ts[1].OriginalArtist, ts[1].Artist = "Fairuz", "Fairuz"
	edits, changed, err := bulkPlan(ts, bulkValues{artist: "Fairuz"})
	if err != nil || changed != 2 || len(edits) != 2 {
		t.Errorf("changed %d, edits %v, err %v", changed, edits, err)
	}
	if _, ok := edits[ts[1].ID]; ok {
		t.Error("a track that already has the artist must be left alone")
	}
}

func TestBulkRefusesWhatCannotWork(t *testing.T) {
	for name, v := range map[string]bulkValues{
		"clearing the artist": {artist: "-"},
		"a bad count":         {of: "many"},
		"a negative count":    {of: "-3"},
		"a bad time":          {cutStart: "1:99"},
		"a negative time":     {cutEnd: "-5"},
	} {
		if _, _, err := bulkPlan(album(2), v); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestBulkSummaryDescribesTheChange(t *testing.T) {
	got := bulkSummary(bulkValues{artist: "Fairuz", album: "-", of: "12", cutStart: "0:02", cutEnd: "-"})
	for _, want := range []string{"Artist → Fairuz", "Album → none (a single)", "Tracks on album → 12", "Cut start → 0:02", "Cut end → none"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q lacks %q", got, want)
		}
	}
	if got := bulkSummary(bulkValues{}); got != "" {
		t.Errorf("nothing to say about nothing: %q", got)
	}
}

func TestBulkSetsAndClearsTheReleaseDate(t *testing.T) {
	ts := album(3)
	ts[1].Released = "1999"
	edits, changed, err := bulkPlan(ts, bulkValues{released: " 1999-05 "})
	if err != nil || changed != 3 {
		t.Fatalf("changed %d, err %v", changed, err)
	}
	if e := edits[ts[0].ID]; e.Released == nil || *e.Released != "1999-05" {
		t.Errorf("= %+v", e)
	}

	edits, changed, err = bulkPlan(ts, bulkValues{released: "-"})
	if err != nil || changed != 1 { // only the track that has a date changes
		t.Fatalf("clearing: changed %d, err %v", changed, err)
	}
	if e := edits[ts[1].ID]; e.Released == nil || *e.Released != "" {
		t.Errorf("= %+v", e)
	}

	if _, _, err := bulkPlan(ts, bulkValues{released: "soon"}); err == nil || !strings.Contains(err.Error(), "1999-05-17") {
		t.Errorf("a bad date: %v", err)
	}
	if got := bulkSummary(bulkValues{released: "1999", cutEnd: "-"}); !strings.Contains(got, "Released → 1999") {
		t.Errorf("summary %q", got)
	}
	if got := bulkSummary(bulkValues{released: "-"}); !strings.Contains(got, "Released → unknown") {
		t.Errorf("summary %q", got)
	}
}
