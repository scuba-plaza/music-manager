package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

func TestCandidateDiffShowsWhatWouldChange(t *testing.T) {
	tr := identified()
	tr.Title, tr.OriginalTitle, tr.Artist, tr.OriginalArtist = "Flow", "Flow", "Shawn James", "Shawn James"
	tr.Album, tr.OriginalAlbum, tr.TrackNumber, tr.TrackTotal = "Live", "Live", 2, 10
	c := app.Candidate{Title: "Flow", Artist: "Shawn James", Album: "Shadows", TrackNumber: 7, TrackTotal: 12}

	got := candidateDiff(tr, c)
	want := []diffLine{
		{"Title", "Flow", "Flow", true},
		{"Artist", "Shawn James", "Shawn James", true},
		{"Album", "Live", "Shadows", false},
		{"Track", "2 of 10", "7 of 12", false},
	}
	if len(got) != len(want) {
		t.Fatalf("diff = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestCandidateDiffOfATrackNobodyIdentifiedStartsFromItsOldTags(t *testing.T) {
	tr := manifest.Track{ID: "u", Status: manifest.Unidentified, Hint: manifest.Hint{Title: "Old Title", Artist: "Old Artist", Filename: "x"}}
	c := app.Candidate{Title: "New Title", Artist: "Old Artist", Album: "An Album", TrackNumber: 3}
	got := candidateDiff(tr, c)
	if got[0].old != "Old Title" || got[0].same || !got[1].same {
		t.Errorf("= %+v", got)
	}
	if got[2].old != "—" || got[2].new != "An Album" {
		t.Errorf("an unknown old value reads —, got %+v", got[2])
	}
	if got[3].old != "—" || got[3].new != "3" {
		t.Errorf("track = %+v", got[3])
	}
}

func TestDiffLinesRead(t *testing.T) {
	if got := diffText(diffLine{"Album", "Live", "Shadows", false}); !strings.Contains(got, "Live → Shadows") {
		t.Errorf("= %q", got)
	}
	if got := diffText(diffLine{"Title", "Flow", "Flow", true}); !strings.Contains(got, "Flow") || strings.Contains(got, "→") {
		t.Errorf("= %q", got)
	}
}

// A metadata source with canned answers, for the picker.
type fakeSource struct {
	results map[string][]app.Candidate // by "title|artist"
	err     error
	queries []app.Query
}

func (f *fakeSource) Search(ctx context.Context, q app.Query) ([]app.Candidate, error) {
	f.queries = append(f.queries, q)
	if f.err != nil {
		return nil, f.err
	}
	return f.results[q.Title+"|"+q.Artist], nil
}

func (f *fakeSource) CoverFront(ctx context.Context, releaseID string) ([]byte, error) {
	return nil, nil
}

var (
	richterA = app.Candidate{RecordingID: "rec-1", ReleaseID: "rel-1", Title: "Dream 3", Artist: "Max Richter", Album: "From Sleep", TrackNumber: 1, TrackTotal: 7, Score: 100, Date: "2015-09-04"}
	richterB = app.Candidate{RecordingID: "rec-1", ReleaseID: "rel-2", Title: "Dream 3", Artist: "Max Richter", Album: "Voyager", TrackNumber: 9, TrackTotal: 36, Score: 100, Date: "2019-10-04"}
)

// pickerModel is a library with one identified track and two nobody
// identified ("Old Title" is on MusicBrainz, "Mystery" is not), looked up in
// src, with "Old Title" selected.
func pickerModel(t *testing.T, src *fakeSource) (Model, *editHook) {
	t.Helper()
	a := testApp(t)
	seed(t, a, seedTrack{"Anchor", "Band", "Album", 1, 1, 0})
	seedTagged(t, a, "01 Old Name", "Old Title", "Old Artist", "Old Album", "1/9")
	seedTagged(t, a, "02 Mystery", "Mystery", "Nobody", "", "")
	hook := &editHook{Library: NewLibrary(a, WithSource(src))}
	m := newTestModel(t, hook, 120, 36)
	return selectTrack(t, m, "Old Title"), hook
}

func asPicker(t *testing.T, m Model) *picker {
	t.Helper()
	p, ok := m.overlay.(*picker)
	if !ok {
		t.Fatalf("no picker is open:\n%s", screen(m))
	}
	return p
}

// open presses i and returns the model while the search is still under way,
// together with the search itself.
func openPicker(t *testing.T, m Model) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(press("i"))
	return next.(Model), cmd
}

func TestIOpensAPickerThatSearchesForTheTrack(t *testing.T) {
	src := &fakeSource{results: map[string][]app.Candidate{"Old Title|Old Artist": {richterA, richterB}}}
	m, _ := pickerModel(t, src)

	m, search := openPicker(t, m)
	contains(t, m, "Identify", "Searching", "Old Title")
	if search == nil {
		t.Fatal("no search was started")
	}

	m = send(t, m, search)
	contains(t, m, "From Sleep", "Voyager", "2015", "2019", "1/7", "9/36", "100%")
	if p := asPicker(t, m); p.cursor != 0 {
		t.Errorf("cursor = %d", p.cursor)
	}
	contains(t, m, "This would change", "Old Title → Dream 3", "Old Album → From Sleep")
	if len(src.queries) != 1 || src.queries[0].Title != "Old Title" || src.queries[0].Artist != "Old Artist" {
		t.Errorf("searched for %+v", src.queries)
	}
}

func TestChoosingACandidateIdentifiesTheTrack(t *testing.T) {
	src := &fakeSource{results: map[string][]app.Candidate{"Old Title|Old Artist": {richterA, richterB}}}
	m, hook := pickerModel(t, src)
	m, search := openPicker(t, m)
	m = send(t, m, search)

	m = keys(t, m, "down")
	contains(t, m, "Old Album → Voyager")
	m = keys(t, m, "enter")

	got := trackNamed(t, hook.Library, "Dream 3")
	if got.Status != manifest.Identified || got.Album != "Voyager" || got.TrackNumber != 9 || got.MBReleaseID != "rel-2" {
		t.Errorf("track = %+v", got)
	}
	if m.overlay != nil {
		t.Error("the picker should close")
	}
	contains(t, m, "Identified as")
	if r, _ := m.selectedRow(); r.Track.ID != got.ID || r.Kind != kindTrack {
		t.Errorf("the selection should follow the track out of Needs work, got %+v", r)
	}

	m = keys(t, m, "u")
	if back := trackNamed(t, hook.Library, "Old Title"); back.Status != manifest.Unidentified || back.Hint.Title != "Old Title" {
		t.Errorf("undo should put the unidentified track back with its old tags: %+v", back)
	}
}

func TestEscCancelsThePickerAndIgnoresLateAnswers(t *testing.T) {
	src := &fakeSource{results: map[string][]app.Candidate{"Old Title|Old Artist": {richterA}}}
	m, hook := pickerModel(t, src)
	m, search := openPicker(t, m)
	m = keys(t, m, "esc") // while still searching
	if m.overlay != nil {
		t.Fatal("esc should close the picker, even while searching")
	}
	m = send(t, m, search) // the answer arrives after all
	if m.overlay != nil {
		t.Error("a late answer must not bring the picker back")
	}
	if trackNamed(t, hook.Library, "Old Title").Status != manifest.Unidentified {
		t.Error("a cancelled picker changed the track")
	}
}

func TestNoMatchSaysSoAndOffersOtherWords(t *testing.T) {
	m, _ := pickerModel(t, &fakeSource{})
	m, search := openPicker(t, m)
	m = send(t, m, search)
	contains(t, m, "No match", "/")
	m = keys(t, m, "enter") // nothing to apply
	asPicker(t, m)
}

func TestASearchThatFailsShowsWhy(t *testing.T) {
	m, _ := pickerModel(t, &fakeSource{err: errors.New("musicbrainz: HTTP 503")})
	m, search := openPicker(t, m)
	m = send(t, m, search)
	contains(t, m, "HTTP 503")
}

func TestSlashSearchesWithOtherWords(t *testing.T) {
	src := &fakeSource{results: map[string][]app.Candidate{"Dream 3|Max Richter": {richterA}}}
	m, hook := pickerModel(t, src)
	m, search := openPicker(t, m)
	m = send(t, m, search)
	contains(t, m, "No match")

	m = keys(t, m, "/")
	contains(t, m, "Title", "Artist")
	m = keys(t, m, "ctrl+u")
	m = typed(t, m, "Dream 3")
	m = keys(t, m, "tab", "ctrl+u")
	m = typed(t, m, "Max Richter")
	m = keys(t, m, "enter")
	contains(t, m, "From Sleep")
	if got := src.queries[len(src.queries)-1]; got.Title != "Dream 3" || got.Artist != "Max Richter" {
		t.Errorf("searched for %+v", got)
	}
	m = keys(t, m, "enter")
	if trackNamed(t, hook.Library, "Dream 3").Status != manifest.Identified {
		t.Error("the match found with other words was not applied")
	}
}

func TestWithoutALookupServiceIDoesNothing(t *testing.T) {
	a := testApp(t)
	seedTagged(t, a, "x", "Old Title", "Old Artist", "", "")
	m := newTestModel(t, NewLibrary(a), 120, 36)
	m = selectTrack(t, m, "Old Title")
	m = keys(t, m, "i")
	if m.overlay != nil {
		t.Error("a picker opened with nothing to search")
	}
	contains(t, m, "not available")
}

func TestITakesAnyTrackButNotAGroup(t *testing.T) {
	m, _ := pickerModel(t, &fakeSource{})
	m = keys(t, m, "g") // the Needs work group
	if r, _ := m.selectedRow(); r.Kind != kindGroup {
		t.Fatalf("on %v", r.Kind)
	}
	m = keys(t, m, "i")
	if m.overlay != nil {
		t.Error("a picker opened for a group")
	}
}

func TestThePickerWorksFromTheInspectorToo(t *testing.T) {
	src := &fakeSource{results: map[string][]app.Candidate{"Old Title|Old Artist": {richterA}}}
	m, _ := pickerModel(t, src)
	m = keys(t, m, "right") // into the inspector
	m, search := openPicker(t, m)
	m = send(t, m, search)
	contains(t, m, "From Sleep")
}

func TestPluralKnowsMatches(t *testing.T) {
	for _, c := range []struct {
		n    int
		word string
		want string
	}{{1, "match", "1 match"}, {12, "match", "12 matches"}, {0, "track", "0 tracks"}, {1, "track", "1 track"}, {2, "album", "2 albums"}, {2, "box", "2 boxes"}} {
		if got := plural(c.n, c.word); got != c.want {
			t.Errorf("plural(%d, %q) = %q, want %q", c.n, c.word, got, c.want)
		}
	}
}

func TestEachCandidateRowLeadsWithTheAlbumAndKeepsItVisible(t *testing.T) {
	long := app.Candidate{Title: "Dream 3 (In the Midst of My Life)", Artist: "Max Richter", Album: "From Sleep", Date: "2015-09-04", TrackNumber: 1, TrackTotal: 7, Score: 100}
	row := candidateText(long, "Dream 3 (In the Midst of My Life)")
	for _, want := range []string{"100%", "From Sleep", "2015-09-04", "1/7", "Max Richter"} {
		if !strings.Contains(row, want) {
			t.Errorf("row %q lacks %q", row, want)
		}
	}
	if strings.Index(row, "From Sleep") > strings.Index(row, "Max Richter") {
		t.Errorf("the album is what you choose by, so it comes first: %q", row)
	}
	if strings.Contains(row, "Dream 3") {
		t.Errorf("a title shared by every candidate is noise in the row: %q", row)
	}
	other := long
	other.Title = "Dream 3 (short edit)"
	if row := candidateText(other, long.Title); !strings.Contains(row, "short edit") {
		t.Errorf("a different recording should say so: %q", row)
	}
}

func TestEscWhileApplyingSkipsTheCoverButStillIdentifies(t *testing.T) {
	src := &slowCoverSource{fakeSource: fakeSource{results: map[string][]app.Candidate{"Old Title|Old Artist": {richterA}}}}
	m, hook := pickerModel(t, &src.fakeSource)
	// The same library, but whose cover download waits until it is cancelled.
	a := appOf(t, hook.Library)
	hook.Library = NewLibrary(a, WithSource(src))
	m.lib = hook

	m, search := openPicker(t, m)
	m = send(t, m, search)
	next, apply := m.Update(press("enter")) // starts applying, which waits for the cover
	m = next.(Model)
	contains(t, m, "Applying")
	m = keys(t, m, "esc") // skip the wait
	m = send(t, m, apply)
	if got := trackNamed(t, hook.Library, "Dream 3"); got.Status != manifest.Identified || got.CoverSHA256 != "" {
		t.Errorf("track = %+v: it should be identified, without a cover", got)
	}
}

// slowCoverSource finds the track at once but only gives up its cover when
// the request is cancelled.
type slowCoverSource struct{ fakeSource }

func (s *slowCoverSource) CoverFront(ctx context.Context, releaseID string) ([]byte, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
