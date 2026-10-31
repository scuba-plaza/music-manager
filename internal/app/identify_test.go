package app_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/testmp3"
)

type fakeSource struct {
	results   map[string][]app.Candidate // by "title|artist"
	errs      map[string]error
	covers    map[string][]byte // by release ID
	coverErr  error
	queries   []app.Query
	coverAsks []string
}

func (f *fakeSource) Search(ctx context.Context, q app.Query) ([]app.Candidate, error) {
	f.queries = append(f.queries, q)
	key := q.Title + "|" + q.Artist
	if err := f.errs[key]; err != nil {
		return nil, err
	}
	return f.results[key], nil
}

func (f *fakeSource) CoverFront(ctx context.Context, releaseID string) ([]byte, error) {
	f.coverAsks = append(f.coverAsks, releaseID)
	return f.covers[releaseID], f.coverErr
}

var dream3 = app.Candidate{
	RecordingID: "rec-1", ReleaseID: "rel-1", Title: "Dream 3", Artist: "Max Richter", Album: "From Sleep",
	TrackNumber: 1, TrackTotal: 7, Score: 100, Date: "2015-09-04",
}
var dream3Voyager = app.Candidate{
	RecordingID: "rec-1", ReleaseID: "rel-2", Title: "Dream 3", Artist: "Max Richter", Album: "Voyager",
	TrackNumber: 9, TrackTotal: 36, Score: 100, Date: "2019-10-04",
}

// untagged returns a library with a track whose old tags and file name are the hints.
func hinted(t *testing.T, filename, title, artist string) (*app.App, manifest.Track) {
	t.Helper()
	a := openLib(t)
	f := filepath.Join(t.TempDir(), filename+".mp3")
	writeFile(t, f, testmp3.WithTags(testmp3.Silent(50), title, artist, ""))
	rep, err := a.Ingest([]string{f})
	if err != nil || len(rep.Added) != 1 {
		t.Fatalf("%+v, %v", rep, err)
	}
	return a, rep.Added[0]
}

func TestIdentifyTakesAConfidentTopMatch(t *testing.T) {
	a, tr := hinted(t, "01 - dream three", "01 - Dream 3", "Max Richter")
	src := &fakeSource{
		results: map[string][]app.Candidate{"Dream 3|Max Richter": {dream3, dream3Voyager}},
		covers:  map[string][]byte{"rel-1": testmp3.JPEG},
	}
	res, err := a.Identify(context.Background(), nil, src, app.IdentifyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(src.queries) != 1 || src.queries[0] != (app.Query{Title: "Dream 3", Artist: "Max Richter"}) {
		t.Errorf("queries = %+v; the track number prefix should be dropped", src.queries)
	}
	if len(res) != 1 || res[0].Applied == nil || res[0].Applied.ReleaseID != "rel-1" {
		t.Fatalf("results = %+v", res)
	}
	got := tracks(t, a)[0]
	if got.Status != manifest.Identified || got.Title != "Dream 3" || got.Artist != "Max Richter" || got.Album != "From Sleep" ||
		got.TrackNumber != 1 || got.TrackTotal != 7 || got.MBRecordingID != "rec-1" || got.MBReleaseID != "rel-1" {
		t.Errorf("track = %+v", got)
	}
	if got.Hint != (manifest.Hint{}) || got.ID != tr.ID {
		t.Errorf("hint %+v, id %s", got.Hint, got.ID)
	}
	if got.CoverSHA256 == "" {
		t.Error("cover not fetched")
	}
}

func TestIdentifySkipsWeakMatchesAndMisses(t *testing.T) {
	a, _ := hinted(t, "x", "Dream 3", "Max Richter")
	weak := dream3
	weak.Score = 80
	for name, results := range map[string][]app.Candidate{"weak": {weak}, "none": nil} {
		src := &fakeSource{results: map[string][]app.Candidate{"Dream 3|Max Richter": results}}
		res, err := a.Identify(context.Background(), nil, src, app.IdentifyOptions{})
		if err != nil || len(res) != 1 || res[0].Applied != nil || res[0].Reason == "" {
			t.Errorf("%s: %+v, %v", name, res, err)
		}
		if tracks(t, a)[0].Status != manifest.Unidentified {
			t.Errorf("%s: track was identified", name)
		}
	}
}

func TestIdentifyLeavesIdentifiedTracksAloneUnlessNamed(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Done", "Band", "", 0, 0)
	src := &fakeSource{}
	a.Identify(context.Background(), nil, src, app.IdentifyOptions{})
	if len(src.queries) != 1 || src.queries[0].Title != "b" {
		t.Errorf("queries = %+v; only the unidentified track should be searched", src.queries)
	}

	src = &fakeSource{results: map[string][]app.Candidate{"Done|Band": {dream3}}}
	res, _ := a.Identify(context.Background(), []string{ts[0].ID}, src, app.IdentifyOptions{})
	if len(src.queries) != 1 || src.queries[0] != (app.Query{Title: "Done", Artist: "Band"}) || res[0].Applied == nil {
		t.Errorf("re-identifying a named track: queries %+v, results %+v", src.queries, res)
	}
	if _, err := a.Identify(context.Background(), []string{"nope"}, src, app.IdentifyOptions{}); err == nil {
		t.Error("an unknown id was accepted")
	}
}

func TestIdentifyOneFailureDoesNotStopTheRest(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	src := &fakeSource{
		errs:    map[string]error{"a|": errors.New("network down")},
		results: map[string][]app.Candidate{"b|": {dream3}},
	}
	res, err := a.Identify(context.Background(), nil, src, app.IdentifyOptions{})
	if err != nil || len(res) != 2 {
		t.Fatalf("%+v, %v", res, err)
	}
	if res[0].Err == nil || res[1].Applied == nil {
		t.Errorf("results = %+v", res)
	}
}

func TestIdentifyCoverRules(t *testing.T) {
	t.Run("an existing cover is kept", func(t *testing.T) {
		a, tr := hinted(t, "x", "Dream 3", "Max Richter")
		a.SetCover(tr.ID, testmp3.JPEG2)
		before := tracks(t, a)[0].CoverSHA256
		src := &fakeSource{results: map[string][]app.Candidate{"Dream 3|Max Richter": {dream3}}, covers: map[string][]byte{"rel-1": testmp3.JPEG}}
		a.Identify(context.Background(), nil, src, app.IdentifyOptions{})
		if len(src.coverAsks) != 0 || tracks(t, a)[0].CoverSHA256 != before {
			t.Errorf("asked for covers %v; cover changed", src.coverAsks)
		}
	})
	t.Run("a failing cover download does not undo the identification", func(t *testing.T) {
		a, _ := hinted(t, "x", "Dream 3", "Max Richter")
		src := &fakeSource{results: map[string][]app.Candidate{"Dream 3|Max Richter": {dream3}}, coverErr: errors.New("archive down")}
		res, err := a.Identify(context.Background(), nil, src, app.IdentifyOptions{})
		if err != nil || res[0].Applied == nil || tracks(t, a)[0].Status != manifest.Identified {
			t.Errorf("%+v, %v", res, err)
		}
	})
}

func TestIdentifyKeepsTheNamesInTheirOwnScript(t *testing.T) {
	a, _ := hinted(t, "x", "سأرجع", "Fairuz")
	arabic := app.Candidate{RecordingID: "r", ReleaseID: "l", Title: "سأرجع", Artist: "فيروز", Album: "القدس في البال", Score: 100, TrackNumber: 3, TrackTotal: 9}
	src := &fakeSource{results: map[string][]app.Candidate{"سأرجع|Fairuz": {arabic}}}
	if _, err := a.Identify(context.Background(), nil, src, app.IdentifyOptions{}); err != nil {
		t.Fatal(err)
	}
	got := tracks(t, a)[0]
	if got.Artist != "فيروز" || got.Title != "سأرجع" || got.Album != "القدس في البال" {
		t.Errorf("track = %+v", got)
	}
}

func TestIdentifyNeedsSomethingToSearchFor(t *testing.T) {
	a := openLib(t)
	f := filepath.Join(t.TempDir(), "123.mp3")
	writeFile(t, f, testmp3.Silent(5))
	a.Ingest([]string{f})
	src := &fakeSource{}
	res, err := a.Identify(context.Background(), nil, src, app.IdentifyOptions{})
	if err != nil || len(res) != 1 || res[0].Applied != nil || res[0].Reason == "" {
		t.Errorf("%+v, %v", res, err)
	}
	if len(src.queries) != 0 {
		t.Errorf("searched for %+v", src.queries)
	}
}

func TestIdentifyPrefersTheReleaseTheFileWasAlreadyFiledUnder(t *testing.T) {
	live := app.Candidate{RecordingID: "r-live", ReleaseID: "rel-live", Title: "Flow", Artist: "Shawn James", Album: "Live at the Heartbreak House", TrackNumber: 2, TrackTotal: 10, Score: 100, Date: "2012"}
	shadows := app.Candidate{RecordingID: "r-live", ReleaseID: "rel-shadows", Title: "Flow", Artist: "Shawn James", Album: "Shadows", TrackNumber: 3, TrackTotal: 12, Score: 100, Date: "2014"}
	single := app.Candidate{RecordingID: "r-single", ReleaseID: "rel-single", Title: "Flow", Artist: "Shawn James", Album: "Flow", TrackNumber: 1, TrackTotal: 1, Score: 100, Date: "2010"}
	key := map[string][]app.Candidate{"Flow|Shawn James": {single, live, shadows}}

	for name, c := range map[string]struct {
		album string
		want  string
	}{
		"the same album":                    {"Shadows", "rel-shadows"},
		"case and punctuation are ignored":  {"SHADOWS!", "rel-shadows"},
		"an EP or single suffix is ignored": {"Live at the Heartbreak House - EP", "rel-live"},
		"an album nobody lists":             {"Something Else", ""}, // the source's first pick, without its release
		"no album at all":                   {"", "rel-single"},
	} {
		t.Run(name, func(t *testing.T) {
			a, _ := hinted(t, "x", "Flow", "Shawn James")
			// hinted() leaves the album empty; ingest a tagged file with one instead.
			a = openLib(t)
			ingestTagged(t, a, "x", "Flow", "Shawn James", c.album, "")
			res, err := a.Identify(context.Background(), nil, &fakeSource{results: key}, app.IdentifyOptions{})
			if err != nil || len(res) != 1 || res[0].Applied == nil || res[0].Applied.ReleaseID != c.want {
				t.Fatalf("picked %+v, want release %q (%v)", res, c.want, err)
			}
			if c.want == "" && res[0].Applied.RecordingID != "r-single" {
				t.Errorf("picked recording %q, want the source's first", res[0].Applied.RecordingID)
			}
		})
	}
}

func TestIdentifyPrefersTheCandidateWithTheSameTrackNumber(t *testing.T) {
	a := openLib(t)
	ingestTagged(t, a, "x", "Flow", "Shawn James", "", "5/12")
	first := app.Candidate{RecordingID: "r", ReleaseID: "rel-1", Title: "Flow", Artist: "Shawn James", Album: "A", TrackNumber: 2, TrackTotal: 10, Score: 100}
	second := app.Candidate{RecordingID: "r", ReleaseID: "rel-2", Title: "Flow", Artist: "Shawn James", Album: "B", TrackNumber: 5, TrackTotal: 12, Score: 100}
	src := &fakeSource{results: map[string][]app.Candidate{"Flow|Shawn James": {first, second}}}
	res, _ := a.Identify(context.Background(), nil, src, app.IdentifyOptions{})
	if res[0].Applied == nil || res[0].Applied.ReleaseID != "rel-2" {
		t.Errorf("picked %+v", res[0].Applied)
	}
}

func TestIdentifyNeverLetsTheHintOutweighTheScore(t *testing.T) {
	a := openLib(t)
	ingestTagged(t, a, "x", "Flow", "Shawn James", "Right Album", "")
	best := app.Candidate{RecordingID: "rec-best", ReleaseID: "rel-best", Title: "Flow", Artist: "Shawn James", Album: "Other", Score: 100}
	worse := app.Candidate{RecordingID: "rec-worse", ReleaseID: "rel-worse", Title: "Flow", Artist: "Shawn James", Album: "Right Album", Score: 96}
	src := &fakeSource{results: map[string][]app.Candidate{"Flow|Shawn James": {best, worse}}}
	res, _ := a.Identify(context.Background(), nil, src, app.IdentifyOptions{})
	if res[0].Applied == nil || res[0].Applied.RecordingID != "rec-best" {
		t.Errorf("picked %+v", res[0].Applied)
	}
}

func TestIdentifyKeepsTheOldAlbumWhenNoReleaseMatchesIt(t *testing.T) {
	compilation := app.Candidate{RecordingID: "rec-9", ReleaseID: "rel-9", Title: "Ahwak", Artist: "Abdel Halim Hafez", Album: "Anthology 1955-1959", TrackNumber: 1, TrackTotal: 7, Score: 100}
	src := &fakeSource{results: map[string][]app.Candidate{"Ahwak|Abdel Halim Hafez": {compilation}}}

	t.Run("automatic: the album the file was filed under stays", func(t *testing.T) {
		a := openLib(t)
		ingestTagged(t, a, "01 Ahwak", "Ahwak", "Abdel Halim Hafez", "Rawaeaa Abd El Halim 5", "1/7")
		res, err := a.Identify(context.Background(), nil, src, app.IdentifyOptions{})
		if err != nil || res[0].Applied == nil {
			t.Fatalf("%+v, %v", res, err)
		}
		got := tracks(t, a)[0]
		if got.Album != "Rawaeaa Abd El Halim 5" || got.TrackNumber != 1 || got.TrackTotal != 7 {
			t.Errorf("album %q %d/%d, want the old one", got.Album, got.TrackNumber, got.TrackTotal)
		}
		if got.MBRecordingID != "rec-9" || got.MBReleaseID != "" {
			t.Errorf("recording %q release %q: the recording is known, the release is not", got.MBRecordingID, got.MBReleaseID)
		}
	})

	t.Run("a file with no album takes the candidate's", func(t *testing.T) {
		a := openLib(t)
		ingestTagged(t, a, "01 Ahwak", "Ahwak", "Abdel Halim Hafez", "", "")
		a.Identify(context.Background(), nil, src, app.IdentifyOptions{})
		if got := tracks(t, a)[0]; got.Album != "Anthology 1955-1959" || got.MBReleaseID != "rel-9" {
			t.Errorf("track = %+v", got)
		}
	})
}

func TestCandidatesListsMatchesBestFitFirstAndCapped(t *testing.T) {
	a := openLib(t)
	tr := ingestTagged(t, a, "x", "Flow", "Shawn James", "Shadows", "")
	var cands []app.Candidate
	for i := 0; i < 30; i++ {
		c := app.Candidate{ReleaseID: fmt.Sprint("rel-", i), Title: "Flow", Artist: "Shawn James", Album: fmt.Sprint("Album ", i), Score: 100}
		if i == 20 {
			c.Album = "Shadows"
		}
		cands = append(cands, c)
	}
	src := &fakeSource{results: map[string][]app.Candidate{"Flow|Shawn James": cands}}
	got, err := a.Candidates(context.Background(), tr.ID, src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 15 || got[0].ReleaseID != "rel-20" {
		t.Errorf("%d candidates, first %q; want 15 with the Shadows release first", len(got), got[0].ReleaseID)
	}
	if tracks(t, a)[0].Status != manifest.Unidentified {
		t.Error("listing candidates must not change the track")
	}
}

func TestCandidatesWithYourOwnWords(t *testing.T) {
	a := openLib(t)
	tr := ingestTagged(t, a, "x", "Ya Roubou'a Biladi", "Fairouz", "", "")
	src := &fakeSource{results: map[string][]app.Candidate{"يا ربوع بلادي|فيروز": {dream3}}}
	got, err := a.Candidates(context.Background(), tr.ID, src, &app.Query{Title: "يا ربوع بلادي", Artist: "فيروز"})
	if err != nil || len(got) != 1 {
		t.Fatalf("%v, %v", got, err)
	}
	if len(src.queries) != 1 || src.queries[0].Artist != "فيروز" {
		t.Errorf("searched for %+v, want the given words", src.queries)
	}
}

func TestCandidatesNeedsSomethingToSearchFor(t *testing.T) {
	a := openLib(t)
	f := filepath.Join(t.TempDir(), "123.mp3")
	writeFile(t, f, testmp3.Silent(5))
	a.Ingest([]string{f})
	if _, err := a.Candidates(context.Background(), tracks(t, a)[0].ID, &fakeSource{}, nil); err == nil {
		t.Error("searched for nothing")
	}
	if _, err := a.Candidates(context.Background(), "nope", &fakeSource{}, nil); err == nil {
		t.Error("unknown id accepted")
	}
	failing := &fakeSource{errs: map[string]error{"123|": errors.New("offline")}}
	if _, err := a.Candidates(context.Background(), tracks(t, a)[0].ID, failing, &app.Query{Title: "123"}); err == nil || !strings.Contains(err.Error(), "offline") {
		t.Errorf("err = %v", err)
	}
}

func TestApplyCandidateIdentifiesTheTrackAsChosen(t *testing.T) {
	a := openLib(t)
	tr := ingestTagged(t, a, "x", "Dream 3", "Max Richter", "Whatever", "")
	src := &fakeSource{covers: map[string][]byte{"rel-2": testmp3.JPEG}}
	got, err := a.ApplyCandidate(context.Background(), tr.ID, dream3Voyager, src)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != manifest.Identified || got.Album != "Voyager" || got.TrackNumber != 9 || got.MBReleaseID != "rel-2" || got.CoverSHA256 == "" {
		t.Errorf("track = %+v", got)
	}
	if saved := tracks(t, a)[0]; saved.Album != "Voyager" {
		t.Errorf("not saved: %+v", saved)
	}
	if _, err := a.ApplyCandidate(context.Background(), "nope", dream3, src); err == nil {
		t.Error("unknown id accepted")
	}
}

func TestIdentifyReportsEachResultAsItHappens(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30)
	src := &fakeSource{results: map[string][]app.Candidate{"b|": {dream3}}}
	var seen []string
	_, err := a.Identify(context.Background(), nil, src, app.IdentifyOptions{OnResult: func(r app.IdentifyResult) {
		seen = append(seen, r.TrackID)
	}})
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, tr := range tracks(t, a) {
		want = append(want, tr.ID)
	}
	if strings.Join(seen, ",") != strings.Join(want, ",") {
		t.Errorf("callback saw %v, want every track in order %v", seen, want)
	}
}

func TestIdentifyTakesTheReleaseDateOfTheMatch(t *testing.T) {
	a, tr := hinted(t, "dream", "Dream 3", "Max Richter")
	src := &fakeSource{results: map[string][]app.Candidate{"Dream 3|Max Richter": {dream3}}}
	if _, err := a.Identify(context.Background(), nil, src, app.IdentifyOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := tracks(t, a)[0]; got.Released != "2015-09-04" {
		t.Errorf("released = %q, want the match's 2015-09-04 (track %s)", got.Released, tr.ID)
	}
}

func TestAMatchWithoutAUsableDateKeepsTheOneWeHave(t *testing.T) {
	for _, date := range []string{"", "0000", "??"} {
		a, tr := hinted(t, "dream", "Dream 3", "Max Richter")
		if _, err := a.Edit(tr.ID, app.Edit{Released: ptr("2014")}); err != nil {
			t.Fatal(err)
		}
		c := dream3
		c.Date = date
		if _, err := a.ApplyCandidate(context.Background(), tr.ID, c, &fakeSource{}); err != nil {
			t.Fatalf("date %q: %v", date, err)
		}
		if got := tracks(t, a)[0]; got.Released != "2014" || got.Title != "Dream 3" {
			t.Errorf("date %q: released = %q, title %q", date, got.Released, got.Title)
		}
	}
}

// The database is asked for the first artist of an old tag that names several:
// "Max Richter feat. Someone" finds nothing there.
func TestIdentifyAsksForTheFirstArtistOfSeveral(t *testing.T) {
	a, _ := hinted(t, "dream", "Dream 3", "Max Richter feat. Clarice Jensen")
	src := &fakeSource{results: map[string][]app.Candidate{}}
	if _, err := a.Identify(context.Background(), nil, src, app.IdentifyOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(src.queries) != 1 || src.queries[0] != (app.Query{Title: "Dream 3", Artist: "Max Richter"}) {
		t.Errorf("queries = %+v", src.queries)
	}
}

// meddlingSource does something else to the library while a track is looked
// up, as another mm would.
type meddlingSource struct {
	*fakeSource
	meanwhile func()
}

func (m *meddlingSource) Search(ctx context.Context, q app.Query) ([]app.Candidate, error) {
	if m.meanwhile != nil {
		m.meanwhile()
		m.meanwhile = nil
	}
	return m.fakeSource.Search(ctx, q)
}

func TestIdentifyKeepsWhatAnotherMmChangedWhileTheLookupWaited(t *testing.T) {
	a, _ := hinted(t, "01 - dream three", "01 - Dream 3", "Max Richter")
	f := filepath.Join(t.TempDir(), "other.mp3")
	writeFile(t, f, testmp3.Silent(30))
	rep, err := a.Ingest([]string{f})
	if err != nil || len(rep.Added) != 1 {
		t.Fatalf("%+v, %v", rep, err)
	}
	bystander := rep.Added[0]
	identify(t, a, bystander.ID, "Song", "Band", "Album", 1, 1)

	other, err := app.Open(a.Root())
	if err != nil {
		t.Fatal(err)
	}
	genre := "Folk"
	src := &meddlingSource{
		fakeSource: &fakeSource{results: map[string][]app.Candidate{"Dream 3|Max Richter": {dream3}}},
		meanwhile: func() { // the lookup takes seconds on the network
			if _, err := other.Edit(bystander.ID, app.Edit{Genre: &genre}); err != nil {
				t.Error(err)
			}
		},
	}
	if _, err := a.Identify(context.Background(), nil, src, app.IdentifyOptions{}); err != nil {
		t.Fatal(err)
	}

	for _, tr := range tracks(t, a) {
		switch tr.ID {
		case bystander.ID:
			if tr.Genre != "Folk" {
				t.Errorf("the genre set meanwhile was overwritten: %q", tr.Genre)
			}
		default:
			if tr.Status != manifest.Identified {
				t.Errorf("the track looked up was not identified: %+v", tr)
			}
		}
	}
}
