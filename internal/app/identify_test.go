package app_test

import (
	"context"
	"errors"
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

func TestIdentifyYesTakesAConfidentTopMatch(t *testing.T) {
	a, tr := hinted(t, "01 - dream three", "01 - Dream 3", "Max Richter")
	src := &fakeSource{
		results: map[string][]app.Candidate{"Dream 3|Max Richter": {dream3, dream3Voyager}},
		covers:  map[string][]byte{"rel-1": testmp3.JPEG},
	}
	res, err := a.Identify(context.Background(), nil, src, nil, app.IdentifyOptions{Yes: true})
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

func TestIdentifyYesSkipsWeakMatchesAndMisses(t *testing.T) {
	a, _ := hinted(t, "x", "Dream 3", "Max Richter")
	weak := dream3
	weak.Score = 80
	for name, results := range map[string][]app.Candidate{"weak": {weak}, "none": nil} {
		src := &fakeSource{results: map[string][]app.Candidate{"Dream 3|Max Richter": results}}
		res, err := a.Identify(context.Background(), nil, src, nil, app.IdentifyOptions{Yes: true})
		if err != nil || len(res) != 1 || res[0].Applied != nil || res[0].Reason == "" {
			t.Errorf("%s: %+v, %v", name, res, err)
		}
		if tracks(t, a)[0].Status != manifest.Unidentified {
			t.Errorf("%s: track was identified", name)
		}
	}
}

func TestIdentifyAsksWhichCandidateAndLetsYouSkip(t *testing.T) {
	a, _ := hinted(t, "x", "Dream 3", "Max Richter")
	src := &fakeSource{results: map[string][]app.Candidate{"Dream 3|Max Richter": {dream3, dream3Voyager}}}

	p := &scriptedPrompter{choice: 1} // the second candidate
	res, err := a.Identify(context.Background(), nil, src, p, app.IdentifyOptions{})
	if err != nil || res[0].Applied == nil || res[0].Applied.ReleaseID != "rel-2" {
		t.Fatalf("%+v, %v", res, err)
	}
	if len(p.options) != 3 { // two candidates and the way out
		t.Errorf("options = %q", p.options)
	}
	for _, want := range []string{"Max Richter", "Dream 3", "From Sleep", "2015", "1/7"} {
		if !strings.Contains(p.options[0], want) {
			t.Errorf("option %q lacks %q", p.options[0], want)
		}
	}
	if got := tracks(t, a)[0]; got.Album != "Voyager" || got.TrackNumber != 9 {
		t.Errorf("track = %+v", got)
	}

	// Choosing the last option skips.
	a2, _ := hinted(t, "y", "Dream 3", "Max Richter")
	p2 := &scriptedPrompter{choice: 2}
	res, err = a2.Identify(context.Background(), nil, src, p2, app.IdentifyOptions{})
	if err != nil || res[0].Applied != nil || tracks(t, a2)[0].Status != manifest.Unidentified {
		t.Errorf("skip: %+v, %v", res, err)
	}
	// Giving up stops everything.
	if _, err := a2.Identify(context.Background(), nil, src, &scriptedPrompter{abort: true}, app.IdentifyOptions{}); !errors.Is(err, app.ErrAborted) {
		t.Errorf("err = %v", err)
	}
}

func TestIdentifyWithoutAnswersAvailableIsAnError(t *testing.T) {
	a, _ := hinted(t, "x", "Dream 3", "Max Richter")
	src := &fakeSource{}
	if _, err := a.Identify(context.Background(), nil, src, nil, app.IdentifyOptions{}); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Errorf("err = %v, want a hint about --yes", err)
	}
	if len(src.queries) != 0 {
		t.Error("searched before failing")
	}
}

func TestIdentifyLeavesIdentifiedTracksAloneUnlessNamed(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Done", "Band", "", 0, 0)
	src := &fakeSource{}
	a.Identify(context.Background(), nil, src, nil, app.IdentifyOptions{Yes: true})
	if len(src.queries) != 1 || src.queries[0].Title != "b" {
		t.Errorf("queries = %+v; only the unidentified track should be searched", src.queries)
	}

	src = &fakeSource{results: map[string][]app.Candidate{"Done|Band": {dream3}}}
	res, _ := a.Identify(context.Background(), []string{ts[0].ID}, src, nil, app.IdentifyOptions{Yes: true})
	if len(src.queries) != 1 || src.queries[0] != (app.Query{Title: "Done", Artist: "Band"}) || res[0].Applied == nil {
		t.Errorf("re-identifying a named track: queries %+v, results %+v", src.queries, res)
	}
	if _, err := a.Identify(context.Background(), []string{"nope"}, src, nil, app.IdentifyOptions{Yes: true}); err == nil {
		t.Error("an unknown id was accepted")
	}
}

func TestIdentifyOneFailureDoesNotStopTheRest(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	src := &fakeSource{
		errs:    map[string]error{"a|": errors.New("network down")},
		results: map[string][]app.Candidate{"b|": {dream3}},
	}
	res, err := a.Identify(context.Background(), nil, src, nil, app.IdentifyOptions{Yes: true})
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
		a.Identify(context.Background(), nil, src, nil, app.IdentifyOptions{Yes: true})
		if len(src.coverAsks) != 0 || tracks(t, a)[0].CoverSHA256 != before {
			t.Errorf("asked for covers %v; cover changed", src.coverAsks)
		}
	})
	t.Run("a failing cover download does not undo the identification", func(t *testing.T) {
		a, _ := hinted(t, "x", "Dream 3", "Max Richter")
		src := &fakeSource{results: map[string][]app.Candidate{"Dream 3|Max Richter": {dream3}}, coverErr: errors.New("archive down")}
		res, err := a.Identify(context.Background(), nil, src, nil, app.IdentifyOptions{Yes: true})
		if err != nil || res[0].Applied == nil || tracks(t, a)[0].Status != manifest.Identified {
			t.Errorf("%+v, %v", res, err)
		}
	})
}

func TestIdentifyKeepsTheOriginalScriptAndUsesTheLatinAlias(t *testing.T) {
	a, _ := hinted(t, "x", "سأرجع", "Fairuz")
	arabic := app.Candidate{RecordingID: "r", ReleaseID: "l", Title: "سأرجع", Artist: "فيروز", ArtistASCII: "Fairuz", Album: "القدس في البال", Score: 100, TrackNumber: 3, TrackTotal: 9}
	src := &fakeSource{results: map[string][]app.Candidate{"سأرجع|Fairuz": {arabic}}}
	if _, err := a.Identify(context.Background(), nil, src, nil, app.IdentifyOptions{Yes: true}); err != nil {
		t.Fatal(err)
	}
	got := tracks(t, a)[0]
	if got.Artist != "Fairuz" || got.OriginalArtist != "فيروز" || got.OriginalTitle != "سأرجع" || got.Title == got.OriginalTitle || got.OriginalAlbum != "القدس في البال" {
		t.Errorf("track = %+v", got)
	}
}

func TestIdentifyNeedsSomethingToSearchFor(t *testing.T) {
	a := openLib(t)
	f := filepath.Join(t.TempDir(), "123.mp3")
	writeFile(t, f, testmp3.Silent(5))
	a.Ingest([]string{f})
	src := &fakeSource{}
	res, err := a.Identify(context.Background(), nil, src, nil, app.IdentifyOptions{Yes: true})
	if err != nil || len(res) != 1 || res[0].Applied != nil || res[0].Reason == "" {
		t.Errorf("%+v, %v", res, err)
	}
	if len(src.queries) != 0 {
		t.Errorf("searched for %+v", src.queries)
	}
}
