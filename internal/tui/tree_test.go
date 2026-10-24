package tui

import (
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

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// expandAll opens every row, level by level, until nothing new appears.
func expandAll(tracks []manifest.Track) map[string]bool {
	open := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, r := range buildRowsWith(tracks, nil, "", open) {
			if !open[r.Key] {
				open[r.Key] = true
				changed = true
			}
		}
	}
	return open
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
	got := labels(buildRowsWith(sampleTracks, nil, "", expandAll(sampleTracks)))
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
	if !equal(got, want) {
		t.Errorf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestEverythingStartsCollapsedExceptWhatWasOpened(t *testing.T) {
	rows := buildRowsWith(sampleTracks, nil, "", map[string]bool{})
	if got, want := labels(rows), []string{"Needs work", "Fairuz", "phoria", "Shawn James"}; !equal(got, want) {
		t.Errorf("collapsed = %v, want %v", got, want)
	}
	var fairuz Row
	for _, r := range rows {
		if r.Label == "Fairuz" {
			fairuz = r
		}
	}
	if fairuz.Expanded || fairuz.Count != 3 || fairuz.Kind != kindArtist || fairuz.Depth != 0 {
		t.Errorf("Fairuz = %+v", fairuz)
	}

	open := buildRowsWith(sampleTracks, nil, "", map[string]bool{fairuz.Key: true})
	if got, want := labels(open), []string{"Needs work", "Fairuz", "  Jerusalem in My Heart", "  Singles", "phoria", "Shawn James"}; !equal(got, want) {
		t.Errorf("one artist open = %v, want %v", got, want)
	}
	for _, r := range open {
		if r.Label == "Fairuz" && !r.Expanded {
			t.Error("an opened artist should say so")
		}
	}
}

func TestNeedsWorkGroupListsUnidentifiedTracksAndIsHiddenWhenEmpty(t *testing.T) {
	unid := trk("u1", "", "", "", 0)
	unid.Hint = manifest.Hint{Title: "Bahlam Bek", Artist: "Abdel Halim", Filename: "01 Bahlam Bek"}
	rows := buildRowsWith([]manifest.Track{unid, trk("a", "A", "Band", "", 0)}, nil, "", map[string]bool{"needs": true})
	if rows[0].Kind != kindGroup || rows[0].Count != 1 || rows[0].Label != "Needs work" {
		t.Fatalf("first row = %+v", rows[0])
	}
	if rows[1].Kind != kindTrack || rows[1].Label != "Bahlam Bek" || rows[1].Detail != "Abdel Halim" || rows[1].Depth != 1 {
		t.Errorf("unidentified row = %+v; it should show what the file said about itself", rows[1])
	}

	for _, r := range buildRowsWith([]manifest.Track{trk("a", "A", "Band", "", 0)}, nil, "", map[string]bool{}) {
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
	for _, r := range buildRowsWith(tracks, nil, "", expandAll(tracks)) {
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
	rows := buildRowsWith(sampleTracks, nil, "shawn flow", map[string]bool{})
	if got, want := labels(rows), []string{"Shawn James", "  Shadows", "    Flow"}; !equal(got, want) {
		t.Errorf("rows = %v, want %v (a filter opens everything it finds)", got, want)
	}
	if rows[0].Count != 1 {
		t.Errorf("counts should describe the matches, got %d", rows[0].Count)
	}
	if got := labels(buildRowsWith(sampleTracks, nil, "status:unidentified", map[string]bool{})); !equal(got, []string{"Needs work", "  ?"}) {
		t.Errorf("status filter = %v", got)
	}
	if got := buildRowsWith(sampleTracks, nil, "zzz", map[string]bool{}); len(got) != 0 {
		t.Errorf("no match = %v", labels(got))
	}
	if got := len(buildRowsWith(sampleTracks, nil, "   ", map[string]bool{})); got != 4 {
		t.Errorf("a blank filter filters nothing: %d rows", got)
	}
}

func TestEveryRowHasAStableUniqueKeyAndAParent(t *testing.T) {
	expanded := expandAll(sampleTracks)
	rows := buildRowsWith(sampleTracks, nil, "", expanded)
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
	again := buildRowsWith(sampleTracks, nil, "", expanded)
	for i := range rows {
		if rows[i].Key != again[i].Key {
			t.Errorf("keys are not stable: %q vs %q", rows[i].Key, again[i].Key)
		}
	}
}

func TestAncestorKeysSaySoWhatToOpenToRevealATrack(t *testing.T) {
	expanded := map[string]bool{}
	for _, k := range ancestorKeys(sampleTracks[0]) {
		expanded[k] = true
	}
	var revealed bool
	for _, r := range buildRowsWith(sampleTracks, nil, "", expanded) {
		if r.Kind == kindTrack && r.Track.ID == "t1" {
			revealed = true
		}
	}
	if !revealed {
		t.Error("opening the ancestor keys did not reveal the track")
	}
	if got := ancestorKeys(sampleTracks[7]); len(got) != 1 || got[0] != keyNeedsWork {
		t.Errorf("an unidentified track lives under Needs work, got %v", got)
	}
}

func TestNonTrackRowsKnowWhichTracksTheyHold(t *testing.T) {
	rows := buildRowsWith(sampleTracks, nil, "", map[string]bool{artistKey("Fairuz"): true})
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
	got := labels(buildRowsWith(tracks, nil, "", expandAll(tracks)))
	want := []string{"Abba", "Singles", "Song", "فيروز", "Singles", "وطني", "Zed", "Singles", "Song"}
	for i := range got {
		got[i] = strings.TrimSpace(got[i])
	}
	for i := range want {
		want[i] = strings.TrimSpace(want[i])
	}
	if !equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
}

func TestTheTreeIsInTheOrderOfTheLatinFormsSetByHand(t *testing.T) {
	// Worked out, فيروز is "Firuz" and falls after "Fb"; set by hand it is
	// "Fairouz" and falls before it.
	fairouz := trk("t1", "وطني", "فيروز", "ألبوم", 0)
	fairouz.ArtistLatin = "Fairouz"
	other := trk("t2", "Song", "Fb", "", 0)
	got := labels(buildRowsWith([]manifest.Track{other, fairouz}, nil, "", map[string]bool{}))
	if want := []string{"فيروز", "Fb"}; !equal(got, want) { // the rows show the true name; only the place follows the form
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
	tracks := []manifest.Track{c, a, b}
	rows := labels(buildRowsWith(tracks, nil, "", expandAll(tracks)))
	for i := range rows {
		rows[i] = strings.TrimSpace(rows[i])
	}
	if want := []string{"X", "ص", "ا", "ب", "Beta", "Song"}; !equal(rows, want) {
		t.Errorf("rows = %q, want %q", rows, want)
	}
}
