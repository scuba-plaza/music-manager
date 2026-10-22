package tui

import (
	"context"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

func TestProgressBar(t *testing.T) {
	cases := []struct {
		done, total, width int
		want               string
	}{
		{0, 10, 10, "░░░░░░░░░░"},
		{5, 10, 10, "█████░░░░░"},
		{10, 10, 10, "██████████"},
		{1, 3, 6, "██░░░░"},
		{0, 0, 4, "░░░░"},
		{3, 2, 4, "████"},
	}
	for _, c := range cases {
		if got := ansiStrip(progressBar(c.done, c.total, c.width)); got != c.want {
			t.Errorf("progressBar(%d, %d, %d) = %q, want %q", c.done, c.total, c.width, got, c.want)
		}
	}
}

func TestIAsksThenIdentifiesWhatItConfidentlyCan(t *testing.T) {
	src := &fakeSource{results: map[string][]app.Candidate{"Old Title|Old Artist": {richterA}}}
	m, hook := pickerModel(t, src)
	m = keys(t, m, "I")
	contains(t, m, "Identify 2 tracks", "95", "y identify")
	if m.overlay == nil {
		t.Fatal("no confirmation")
	}
	m = keys(t, m, "esc")
	if len(src.queries) != 0 {
		t.Error("searched before being told to")
	}

	m = keys(t, m, "I", "y")
	contains(t, m, "Identified 1", "Left for you 1", "Failed 0")
	if got := trackNamed(t, hook.Library, "Dream 3"); got.Status != manifest.Identified {
		t.Errorf("track = %+v", got)
	}
	if got := trackNamed(t, hook.Library, "Mystery"); got.Status != manifest.Unidentified {
		t.Error("a track without a match must be left alone")
	}
	contains(t, m, "Max Richter") // the tree already shows the new artist

	m = keys(t, m, "enter")
	if m.overlay != nil {
		t.Error("enter should close the summary")
	}

	m = keys(t, m, "u")
	if got := trackNamed(t, hook.Library, "Old Title"); got.Status != manifest.Unidentified {
		t.Errorf("one undo should put everything back: %+v", got)
	}
}

func TestIdentifyAllReportsFailures(t *testing.T) {
	m, _ := pickerModel(t, &fakeSource{err: context.DeadlineExceeded})
	m = keys(t, m, "I", "y")
	contains(t, m, "Failed 2", "deadline")
}

// A source that waits until the search is cancelled.
type waitingSource struct{ fakeSource }

func (w *waitingSource) Search(ctx context.Context, q app.Query) ([]app.Candidate, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestEscStopsIdentifyingAndKeepsWhatIsDone(t *testing.T) {
	src := &waitingSource{}
	a := testApp(t)
	seedTagged(t, a, "x", "Old Title", "Old Artist", "", "")
	hook := &editHook{Library: NewLibrary(a, WithSource(src))}
	m := newTestModel(t, hook, 120, 36)

	m = keys(t, m, "I", "y")
	contains(t, m, "Identifying")
	m = keys(t, m, "esc")
	contains(t, m, "Stopped")
	m = keys(t, m, "enter")
	if m.overlay != nil {
		t.Error("the summary should close")
	}
	if trackNamed(t, hook.Library, "Old Title").Status != manifest.Unidentified {
		t.Error("a stopped run changed a track")
	}
}

func TestIdentifyAllWithNothingToDo(t *testing.T) {
	a := testApp(t)
	seed(t, a, seedTrack{"Anchor", "Band", "Album", 1, 1, 0})
	m := newTestModel(t, NewLibrary(a, WithSource(&fakeSource{})), 120, 36)
	m = keys(t, m, "I")
	if m.overlay != nil {
		t.Error("a dialog opened with nothing to identify")
	}
	contains(t, m, "Nothing needs work")
}

func TestIdentifyAllNeedsALookupService(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "I")
	if m.overlay != nil {
		t.Error("dialog opened")
	}
	contains(t, m, "not available")
}

func TestOUsesTheOldTagsOfTheSelectedTrack(t *testing.T) {
	m, hook := pickerModel(t, &fakeSource{})
	m = keys(t, m, "o")
	got := trackNamed(t, hook.Library, "Old Title")
	if got.Status != manifest.Identified || got.Artist != "Old Artist" || got.Album != "Old Album" || got.TrackNumber != 1 || got.TrackTotal != 9 {
		t.Errorf("track = %+v", got)
	}
	contains(t, m, "old tags")
	if trackNamed(t, hook.Library, "Mystery").Status != manifest.Unidentified {
		t.Error("another track was adopted too")
	}
	m = keys(t, m, "u")
	if trackNamed(t, hook.Library, "Old Title").Status != manifest.Unidentified {
		t.Error("undo did not put the track back")
	}
}

func TestOOnTheGroupAdoptsEverythingInIt(t *testing.T) {
	m, hook := pickerModel(t, &fakeSource{})
	m = keys(t, m, "g", "o")
	for _, title := range []string{"Old Title", "Mystery"} {
		if trackNamed(t, hook.Library, title).Status != manifest.Identified {
			t.Errorf("%s was not adopted", title)
		}
	}
	contains(t, m, "2 tracks")
}

func TestOTellsWhenThereAreNoTagsToUse(t *testing.T) {
	a := testApp(t)
	seed(t, a, seedTrack{"", "", "", 0, 0, 0}) // a file with no tags at all
	m := newTestModel(t, NewLibrary(a), 120, 36)
	m = keys(t, m, "g", "o")
	contains(t, m, "no old tags")
}
