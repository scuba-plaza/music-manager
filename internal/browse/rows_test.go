package browse

import (
	"slices"
	"strings"
	"testing"
	"time"

	"music-manager/internal/manifest"
)

func trk(id, title, artist, album string, n int) manifest.Track {
	status := manifest.Identified
	if title == "" || artist == "" {
		status = manifest.Unidentified
	}
	return manifest.Track{ID: id, Title: title, Artist: artist, Album: album, TrackNumber: n, Status: status, SourceLength: 3 * time.Minute}
}

func labels(rows []Row) []string {
	var out []string
	for _, r := range rows {
		out = append(out, strings.Repeat("  ", r.Depth)+r.Label)
	}
	return out
}

// topLabels lists the labels of the rows at the top of the tree.
func topLabels(rows []Row) []string {
	var out []string
	for _, r := range rows {
		if r.Depth == 0 {
			out = append(out, r.Label)
		}
	}
	return out
}

var sampleTracks = []manifest.Track{
	trk("t1", "The Wanderer", "Shawn James", "Shadows", 1),
	trk("t2", "Along Our Way", "Shawn James", "Shadows", 12),
	trk("t3", "Flow", "Shawn James", "Shadows", 7),
	trk("t4", "Current", "phoria", "Caught a Black Rabbit", 4),
	trk("t5", "Wtni", "Fairuz", "", 0),
	trk("t6", "Sanarjiou", "Fairuz", "Jerusalem in My Heart", 1),
	trk("t7", "Loose", "Fairuz", "", 0),
	trk("u1", "", "", "", 0),
}

func TestRowsGroupByArtistThenAlbumAndSortSensibly(t *testing.T) {
	got := labels(buildRows(sampleTracks, nil, ""))
	want := []string{
		"Needs work",
		"  ?",
		"Fairuz",
		"  Jerusalem in My Heart",
		"    Sanarjiou",
		"  Singles",
		"    Loose",
		"    Wtni",
		"phoria",
		"  Caught a Black Rabbit",
		"    Current",
		"Shawn James",
		"  Shadows",
		"    The Wanderer",
		"    Flow",
		"    Along Our Way",
	}
	// "?" is the placeholder label of a track nobody could name.
	if !slices.Equal(got, want) {
		t.Errorf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestNeedsWorkGroupListsUnidentifiedTracksAndIsHiddenWhenEmpty(t *testing.T) {
	unid := trk("u1", "", "", "", 0)
	unid.Hint = manifest.Hint{Title: "Bahlam Bek", Artist: "Abdel Halim", Filename: "01 Bahlam Bek"}
	rows := buildRows([]manifest.Track{unid, trk("a", "A", "Band", "", 0)}, nil, "")
	if rows[0].Kind != kindGroup || rows[0].Count != 1 || rows[0].Label != "Needs work" {
		t.Fatalf("first row = %+v", rows[0])
	}
	if rows[1].Kind != kindTrack || rows[1].Label != "Bahlam Bek" || rows[1].Detail != "Abdel Halim" || rows[1].Depth != 1 {
		t.Errorf("unidentified row = %+v; it should show what the file said about itself", rows[1])
	}

	for _, r := range buildRows([]manifest.Track{trk("a", "A", "Band", "", 0)}, nil, "") {
		if r.Kind == kindGroup {
			t.Error("Needs work should not appear when everything is identified")
		}
	}
}

func TestTrackRowsCarryTheirNumberPaddedToTheAlbum(t *testing.T) {
	tracks := []manifest.Track{
		trk("a", "One", "Band", "Short", 3),
		trk("b", "Two", "Band", "Short", 0),
		trk("c", "Three", "Band", "Long", 7),
		trk("d", "Four", "Band", "Long", 120),
	}
	nums := map[string]string{}
	for _, r := range buildRows(tracks, nil, "") {
		if r.Kind == kindTrack {
			nums[r.Label] = r.Num
		}
	}
	want := map[string]string{"One": "03", "Two": "", "Three": "007", "Four": "120"}
	for k, v := range want {
		if nums[k] != v {
			t.Errorf("number of %s = %q, want %q", k, nums[k], v)
		}
	}
}

func TestFilteringUsesTheSameWordsAsLs(t *testing.T) {
	rows := buildRows(sampleTracks, nil, "shawn flow")
	if got, want := labels(rows), []string{"Shawn James", "  Shadows", "    Flow"}; !slices.Equal(got, want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
	if rows[0].Count != 1 {
		t.Errorf("counts should describe the matches, got %d", rows[0].Count)
	}
	if got := labels(buildRows(sampleTracks, nil, "status:unidentified")); !slices.Equal(got, []string{"Needs work", "  ?"}) {
		t.Errorf("status filter = %v", got)
	}
	if got := buildRows(sampleTracks, nil, "zzz"); len(got) != 0 {
		t.Errorf("no match = %v", labels(got))
	}
	if got, all := len(buildRows(sampleTracks, nil, "   ")), len(buildRows(sampleTracks, nil, "")); got != all {
		t.Errorf("a blank filter filters nothing: %d rows of %d", got, all)
	}
}

func TestEveryRowHasAStableUniqueKeyAndAParent(t *testing.T) {
	rows := buildRows(sampleTracks, nil, "")
	seen := map[string]bool{}
	byKey := map[string]Row{}
	for _, r := range rows {
		if seen[r.Key] {
			t.Errorf("duplicate key %q", r.Key)
		}
		seen[r.Key] = true
		byKey[r.Key] = r
	}
	for _, r := range rows {
		if r.Depth > 0 {
			p, ok := byKey[r.Parent]
			if !ok || p.Depth != r.Depth-1 {
				t.Errorf("row %q: parent %q = %+v", r.Label, r.Parent, p)
			}
		}
	}
	again := buildRows(sampleTracks, nil, "")
	for i := range rows {
		if rows[i].Key != again[i].Key {
			t.Errorf("keys are not stable: %q vs %q", rows[i].Key, again[i].Key)
		}
	}
}

func TestTheAncestorKeysOfATrackAreTheRowsAboveIt(t *testing.T) {
	rows := buildRows(sampleTracks, nil, "")
	parent := map[string]string{}
	for _, r := range rows {
		parent[r.Key] = r.Parent
	}
	for _, tr := range []manifest.Track{sampleTracks[0], sampleTracks[4], sampleTracks[7]} {
		var above []string
		for k := parent[trackKey(tr.ID)]; k != ""; k = parent[k] {
			above = append([]string{k}, above...)
		}
		if got := ancestorKeys(tr); !slices.Equal(got, above) {
			t.Errorf("%s: ancestor keys %q, want the rows above it %q", tr.ID, got, above)
		}
	}
}

func TestNonTrackRowsKnowWhichTracksTheyHold(t *testing.T) {
	rows := buildRows(sampleTracks, nil, "")
	byLabel := map[string]Row{}
	for _, r := range rows {
		byLabel[r.Label] = r
	}
	// in the order they are listed: the album, then the singles by title
	if got := strings.Join(byLabel["Fairuz"].IDs, ","); got != "t6,t7,t5" {
		t.Errorf("Fairuz holds %v", byLabel["Fairuz"].IDs)
	}
	if got := byLabel["Jerusalem in My Heart"].IDs; len(got) != 1 || got[0] != "t6" {
		t.Errorf("album holds %v", got)
	}
	if got := byLabel["Needs work"].IDs; len(got) != 1 || got[0] != "u1" {
		t.Errorf("Needs work holds %v", got)
	}
	if got := byLabel["Shawn James"].IDs; len(got) != 3 {
		t.Errorf("Shawn James holds %v", got)
	}
}

func TestTheTreeShowsTheTrueNamesInTheOrderOfTheirLatinForms(t *testing.T) {
	tracks := []manifest.Track{
		trk("t1", "Song", "Zed", "", 0),
		trk("t2", "وطني", "فيروز", "", 0), // Firuz
		trk("t3", "Song", "Abba", "", 0),
	}
	got := labels(buildRows(tracks, nil, ""))
	want := []string{"Abba", "  Singles", "    Song", "فيروز", "  Singles", "    وطني", "Zed", "  Singles", "    Song"}
	if !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
}

func TestTheTreeIsInTheOrderOfTheLatinFormsSetByHand(t *testing.T) {
	// Worked out, فيروز is "Firuz" and falls after "Fb"; set by hand it is
	// "Fairouz" and falls before it.
	fairouz := trk("t1", "وطني", "فيروز", "ألبوم", 0)
	fairouz.ArtistLatin = "Fairouz"
	other := trk("t2", "Song", "Fb", "", 0)
	got := topLabels(buildRows([]manifest.Track{other, fairouz}, nil, ""))
	if want := []string{"فيروز", "Fb"}; !slices.Equal(got, want) { // the rows show the true name; only the place follows the form
		t.Errorf("rows = %q, want %q", got, want)
	}

	// Albums of one artist and the tracks of an album follow their forms too.
	a := trk("a", "ب", "X", "ص", 0)
	a.AlbumLatin = "Alpha"
	a.TitleLatin = "Zulu"
	b := trk("b", "ا", "X", "ص", 0)
	b.AlbumLatin = "Alpha"
	b.TitleLatin = "Bravo"
	c := trk("c", "Song", "X", "Beta", 0)
	rows := labels(buildRows([]manifest.Track{c, a, b}, nil, ""))
	if want := []string{"X", "  ص", "    ا", "    ب", "  Beta", "    Song"}; !slices.Equal(rows, want) {
		t.Errorf("rows = %q, want %q", rows, want)
	}
}
