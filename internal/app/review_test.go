package app_test

import (
	"strings"
	"testing"

	"music-manager/internal/app"
)

const sampleReview = `# Genres: proposals to review
#
# A comment, and under it the sections.

## ARTISTS -> genre

Fairouz  (10 tracks)  [high ; MusicBrainz]
    Arabic

Max Richter  (1 track)  [medium ; the old tags said Classical]
    Neoclassical

Nobody  (2 tracks)  [low ; a guess]
    -

## ALBUMS -> genre

Songs (Live)  (Fairouz, 3 tracks)  [high]
    Live

## TRACKS -> genre

k7f2qd  Fairouz · Songs (Live) · Wtni  [low ; transliteration]
    Anthem

x9y8z7  Someone · Album · Title
    Folk
`

func TestReadingAReviewFile(t *testing.T) {
	got, err := app.ParseReview(sampleReview)
	if err != nil {
		t.Fatal(err)
	}
	want := []app.ReviewEntry{
		{Section: "ARTISTS", Field: "genre", Key: "Fairouz", Value: "Arabic", Confidence: "high"},
		{Section: "ARTISTS", Field: "genre", Key: "Max Richter", Value: "Neoclassical", Confidence: "medium"},
		{Section: "ARTISTS", Field: "genre", Key: "Nobody", Value: "-", Confidence: "low"},
		{Section: "ALBUMS", Field: "genre", Key: "Songs (Live)", Value: "Live", Confidence: "high"},
		{Section: "TRACKS", Field: "genre", Key: "k7f2qd", Value: "Anthem", Confidence: "low"},
		{Section: "TRACKS", Field: "genre", Key: "x9y8z7", Value: "Folk", Confidence: "high"}, // no note: taken as sure
	}
	if len(got) != len(want) {
		t.Fatalf("%d entries, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		g := got[i]
		g.Line = 0
		if g != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, g, want[i])
		}
	}
}

func TestAnEntryWithNoValueLineIsBlank(t *testing.T) {
	got, err := app.ParseReview("## ARTISTS -> genre\n\nA  (1 track)  [high]\n\nB  (1 track)  [high]\n    Folk\n")
	if err != nil || len(got) != 2 || got[0].Value != "" || got[1].Value != "Folk" {
		t.Errorf("entries %+v, err %v", got, err)
	}
}

func TestMistakesInAReviewFileAreSaidWithTheLine(t *testing.T) {
	for name, c := range map[string]struct{ text, want string }{
		"no field":       {"## ARTISTS\n\nA  [high]\n    Folk\n", "line 1"},
		"unknown field":  {"## ARTISTS -> colour\n", "colour"},
		"unknown part":   {"## SONGS -> genre\n", "SONGS"},
		"before a part":  {"A  [high]\n    Folk\n", "line 1"},
		"value alone":    {"## ARTISTS -> genre\n    Folk\n", "line 2"},
		"unknown rating": {"## ARTISTS -> genre\nA  [certain]\n    Folk\n", "certain"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := app.ParseReview(c.text)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %v, want one that says %q", err, c.want)
			}
		})
	}
}

// reviewLib has two Fairouz tracks on one album, a third of hers on another, a
// Max Richter track and a track nobody has an opinion on.
func reviewLib(t *testing.T) (*app.App, map[string]string) {
	t.Helper()
	a := libWithTracks(t, 10, 11, 12, 13, 14)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Wtni", "Fairouz", "Songs (Live)", 1, 2)
	identify(t, a, ts[1].ID, "Habbaytak", "Fairouz", "Songs (Live)", 2, 2)
	identify(t, a, ts[2].ID, "Kifak", "Fairouz", "Studio", 1, 1)
	identify(t, a, ts[3].ID, "Sleep", "Max Richter", "Sleep", 1, 1)
	identify(t, a, ts[4].ID, "Plain", "Nobody", "", 0, 0)
	return a, map[string]string{"Wtni": ts[0].ID, "Habbaytak": ts[1].ID, "Kifak": ts[2].ID, "Sleep": ts[3].ID, "Plain": ts[4].ID}
}

func genres(t *testing.T, a *app.App) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, tr := range tracks(t, a) {
		out[tr.Title] = tr.Genre
	}
	return out
}

func TestAReviewSetsTheFieldForAnArtistThenItsAlbumsThenSingleTracks(t *testing.T) {
	a, id := reviewLib(t)
	entries, err := app.ParseReview(`## ARTISTS -> genre
Fairouz  (3 tracks)  [high]
    Arabic
Max Richter  (1 track)  [medium]
    Neoclassical
## ALBUMS -> genre
Studio  (Fairouz, 1 track)  [high]
    Studio Pop
## TRACKS -> genre
` + id["Wtni"] + `  Fairouz · Songs (Live) · Wtni  [high]
    Anthem
`)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := a.ApplyReview(entries, app.ReviewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"Wtni": "Anthem", "Habbaytak": "Arabic", "Kifak": "Studio Pop", "Sleep": "Neoclassical", "Plain": ""}
	for title, g := range genres(t, a) {
		if g != want[title] {
			t.Errorf("%s: genre %q, want %q", title, g, want[title])
		}
	}
	if rep.Changed != 4 || rep.Applied != 4 {
		t.Errorf("report %+v, want 4 tracks changed by 4 entries", rep)
	}

	// Again: nothing more to do.
	if rep, err = a.ApplyReview(entries, app.ReviewOptions{}); err != nil || rep.Changed != 0 {
		t.Errorf("a second apply: %+v, %v", rep, err)
	}
}

func TestLowConfidenceAndBlankEntriesAreLeftOutUnlessAskedFor(t *testing.T) {
	a, _ := reviewLib(t)
	entries, err := app.ParseReview(`## ARTISTS -> genre
Fairouz  (3 tracks)  [low ; a guess]
    Arabic
Max Richter  (1 track)  [high]
    -
Nobody  (1 track)  [high]
`)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := a.ApplyReview(entries, app.ReviewOptions{})
	if err != nil || rep.Changed != 0 || rep.Low != 1 || rep.Blank != 2 {
		t.Errorf("report %+v, %v; want nothing changed, 1 low, 2 blank", rep, err)
	}
	if rep, err = a.ApplyReview(entries, app.ReviewOptions{IncludeLow: true}); err != nil || rep.Changed != 3 || rep.Low != 0 {
		t.Errorf("with the low ones: %+v, %v", rep, err)
	}
	if g := genres(t, a); g["Wtni"] != "Arabic" || g["Sleep"] != "" {
		t.Errorf("genres %v", g)
	}
}

func TestADryRunReportsButChangesNothing(t *testing.T) {
	a, _ := reviewLib(t)
	entries, _ := app.ParseReview("## ARTISTS -> genre\nFairouz  (3 tracks)  [high]\n    Arabic\n")
	rep, err := a.ApplyReview(entries, app.ReviewOptions{DryRun: true})
	if err != nil || rep.Changed != 3 {
		t.Errorf("report %+v, %v", rep, err)
	}
	for title, g := range genres(t, a) {
		if g != "" {
			t.Errorf("%s got %q in a dry run", title, g)
		}
	}
}

func TestEntriesThatMatchNothingAreReported(t *testing.T) {
	a, _ := reviewLib(t)
	entries, _ := app.ParseReview("## ARTISTS -> genre\nStranger  (1 track)  [high]\n    Folk\n## TRACKS -> genre\nzzzzzz  Who · What · Ever  [high]\n    Folk\n")
	rep, err := a.ApplyReview(entries, app.ReviewOptions{})
	if err != nil || rep.Changed != 0 || len(rep.Unmatched) != 2 {
		t.Fatalf("report %+v, %v", rep, err)
	}
	if rep.Unmatched[0] != "ARTISTS: Stranger" || rep.Unmatched[1] != "TRACKS: zzzzzz" {
		t.Errorf("unmatched %v", rep.Unmatched)
	}
}

func TestAReviewWithAnInvalidValueChangesNothing(t *testing.T) {
	a, _ := reviewLib(t)
	entries, _ := app.ParseReview("## ARTISTS -> genre\nFairouz  (3 tracks)  [high]\n    Arabic\n## TRACKS -> genre\n")
	entries = append(entries, app.ReviewEntry{Section: "ARTISTS", Field: "genre", Key: "Max Richter", Value: "Two\nLines", Confidence: "high"})
	if _, err := a.ApplyReview(entries, app.ReviewOptions{}); err == nil {
		t.Fatal("accepted a genre with a line break")
	}
	for title, g := range genres(t, a) {
		if g != "" {
			t.Errorf("%s got %q although the review failed", title, g)
		}
	}
}
