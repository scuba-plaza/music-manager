package app_test

import (
	"path/filepath"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/mp3"
	"music-manager/internal/testmp3"
)

// A track can be made louder or quieter, and can fade in and out. The
// loudness moves in steps of 1.505 dB, which is what the file allows without
// being decoded, so a number of dB is rounded to the nearest step.

func TestGainInDecibelsIsKeptAsWholeSteps(t *testing.T) {
	a, tr := ingested(t)
	for _, c := range []struct {
		db   float64
		want int
	}{{3, 2}, {-3, -2}, {2, 1}, {0.7, 0}, {1, 1}, {-6, -4}, {24, 16}, {-24, -16}, {0, 0}} {
		got, err := a.Edit(tr.ID, app.Edit{GainDB: ptr(c.db)})
		if err != nil {
			t.Fatalf("%v dB: %v", c.db, err)
		}
		if got.GainSteps != c.want {
			t.Errorf("%v dB: %d steps, want %d", c.db, got.GainSteps, c.want)
		}
	}
}

func TestAGainBeyondTwentyFourDecibelsIsRefused(t *testing.T) {
	a, tr := ingested(t)
	if _, err := a.Edit(tr.ID, app.Edit{GainDB: ptr(3.0)}); err != nil {
		t.Fatal(err)
	}
	for _, db := range []float64{25, -25, 40} {
		if _, err := a.Edit(tr.ID, app.Edit{GainDB: ptr(db)}); err == nil {
			t.Errorf("%v dB accepted", db)
		}
	}
	if saved := tracks(t, a)[0]; saved.GainSteps != 2 {
		t.Errorf("a refused edit changed the gain to %d steps", saved.GainSteps)
	}
}

func TestEditFades(t *testing.T) {
	a, tr := ingested(t) // about 2.612s of audio
	in, out := manifest.Span{End: 2 * time.Second}, manifest.Span{Start: 2200 * time.Millisecond, End: 2600 * time.Millisecond}
	got, err := a.Edit(tr.ID, app.Edit{FadeIn: &in, FadeOut: &out})
	if err != nil {
		t.Fatal(err)
	}
	if got.FadeIn != in || got.FadeOut != out {
		t.Errorf("fades = %+v, %+v", got.FadeIn, got.FadeOut)
	}

	// Editing something else leaves them alone; an empty span takes one away.
	if got, err = a.Edit(tr.ID, app.Edit{Genre: ptr("Folk")}); err != nil || got.FadeIn != in || got.GainSteps != 0 {
		t.Errorf("another edit changed the fades: %+v, %v", got, err)
	}
	if got, err = a.Edit(tr.ID, app.Edit{FadeIn: &manifest.Span{}}); err != nil || got.FadeIn != (manifest.Span{}) || got.FadeOut != out {
		t.Errorf("clearing the fade-in: %+v, %v", got, err)
	}
}

func TestFadesThatMakeNoSenseAreRefused(t *testing.T) {
	a, tr := ingested(t)
	good := manifest.Span{End: time.Second}
	if _, err := a.Edit(tr.ID, app.Edit{FadeIn: &good}); err != nil {
		t.Fatal(err)
	}
	for name, e := range map[string]app.Edit{
		"ends before it starts":   {FadeOut: &manifest.Span{Start: 2 * time.Second, End: time.Second}},
		"is empty":                {FadeOut: &manifest.Span{Start: time.Second, End: time.Second}},
		"starts before the track": {FadeOut: &manifest.Span{Start: -time.Second, End: time.Second}},
		"ends after the track":    {FadeOut: &manifest.Span{Start: 2 * time.Second, End: time.Hour}},
		"overlaps the fade-in":    {FadeOut: &manifest.Span{Start: 500 * time.Millisecond, End: 2 * time.Second}},
	} {
		if _, err := a.Edit(tr.ID, e); err == nil {
			t.Errorf("a fade-out that %s was accepted", name)
		}
	}
	if saved := tracks(t, a)[0]; saved.FadeOut != (manifest.Span{}) || saved.FadeIn != good {
		t.Errorf("a refused edit changed the record: %+v", saved)
	}
}

// loudTrack ingests a track of 200 granules (2.6 s) that all have the gain 150
// and calls it Song by Band.
func loudTrack(t *testing.T) (*app.App, manifest.Track) {
	t.Helper()
	a := openLib(t)
	gains := make([][]int, 200)
	for i := range gains {
		gains[i] = []int{150, 150}
	}
	f := filepath.Join(t.TempDir(), "loud.mp3")
	writeFile(t, f, testmp3.WithGains(testmp3.Layout{}, gains))
	rep, err := a.Ingest([]string{f})
	if err != nil || len(rep.Added) != 1 {
		t.Fatalf("ingest: %+v, %v", rep, err)
	}
	tr, err := a.Edit(rep.Added[0].ID, app.Edit{Title: ptr("Song"), Artist: ptr("Band")})
	if err != nil {
		t.Fatal(err)
	}
	return a, tr
}

// gainsIn are the gains of the first channel of every granule of an MP3 file.
func gainsIn(t *testing.T, file []byte) []int {
	t.Helper()
	audio, _, err := mp3.Parse(file)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := mp3.GlobalGains(audio)
	if err != nil {
		t.Fatal(err)
	}
	var out []int
	for _, r := range rows {
		out = append(out, r[0])
	}
	return out
}

func TestSyncWritesTheLoudnessInTheTimeOfTheOriginalAndThenCuts(t *testing.T) {
	a, tr := loudTrack(t)
	if _, err := a.Edit(tr.ID, app.Edit{
		TrimStart: ptr(time.Second), GainDB: ptr(3.0), FadeIn: &manifest.Span{End: 2 * time.Second},
	}); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, app.SyncOptions{})

	got := gainsIn(t, readFile(t, a.Root(), "Band/Song.mp3"))
	// The cut keeps what starts at 1 s, which is half way up the fade: 4 steps
	// down, and 2 steps up for the 3 dB. The fade ends at 2 s, after which
	// there is only the gain.
	if got[0] != 148 || got[1] != 148 {
		t.Errorf("the first granules: %d and %d, want 148 (150 + 2 - 4)", got[0], got[1])
	}
	if last := got[len(got)-1]; last != 152 {
		t.Errorf("the last granule: %d, want 152 (150 + 2)", last)
	}
}

func TestChangingTheLoudnessMakesTheTrackNeedASync(t *testing.T) {
	a, tr := loudTrack(t)
	mustSync(t, a, app.SyncOptions{})
	if n, _ := unsynced(t, a); n != 0 {
		t.Fatalf("%d to sync after syncing", n)
	}
	if _, err := a.Edit(tr.ID, app.Edit{GainDB: ptr(-3.0)}); err != nil {
		t.Fatal(err)
	}
	if n, _ := unsynced(t, a); n != 1 {
		t.Errorf("%d to sync after a change of volume, want 1", n)
	}
	if ops := mustSync(t, a, app.SyncOptions{}); len(ops) != 1 {
		t.Errorf("the change should rewrite the file: %v", ops)
	}
}

func TestThePreviewOfACutSoundsAsTheSyncedFileWillWithItsLoudness(t *testing.T) {
	a, tr := loudTrack(t)
	if _, err := a.Edit(tr.ID, app.Edit{GainDB: ptr(-6.0), FadeOut: &manifest.Span{Start: 2 * time.Second, End: 2500 * time.Millisecond}}); err != nil {
		t.Fatal(err)
	}
	path, cleanup, err := a.PreviewTrim(tr.ID, app.EdgeEnd, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	got := gainsIn(t, readFile(t, filepath.Dir(path), filepath.Base(path)))
	if got[0] != 146 { // 150 - 4 steps for 6 dB, before the fade-out begins at 2 s
		t.Errorf("the first granule of the preview: %d, want 146", got[0])
	}
	if last := got[len(got)-1]; last != 0 {
		t.Errorf("the last granule of the preview: %d, want silence (0), as the fade-out has ended", last)
	}
}

// A fade is typed as how long it lasts, counted from the edge of the cut it is
// at (a fade-in from where the track now starts, a fade-out back from where it
// now ends), or as the stretch of the original it covers.

func TestAFadeIsTypedAsALengthOrAsAStretch(t *testing.T) {
	tr := manifest.Track{SourceLength: 10 * time.Second, TrimStart: 500 * time.Millisecond, TrimEnd: time.Second}
	s := func(a, b float64) manifest.Span {
		return manifest.Span{Start: time.Duration(a * float64(time.Second)), End: time.Duration(b * float64(time.Second))}
	}
	for _, c := range []struct {
		text   string
		fadeIn bool
		want   manifest.Span
	}{
		{"3", true, s(0.5, 3.5)},        // 3 s from the start of what is kept
		{"0:03.5", true, s(0.5, 4)},     // times as in the cuts
		{"3", false, s(6, 9)},           // 3 s back from the end of what is kept
		{"2-5", true, s(2, 5)},          // a stretch of the original
		{"0:02 - 0:05", false, s(2, 5)}, // with spaces
		{"", true, manifest.Span{}},     // none
		{"0", false, manifest.Span{}},
	} {
		got, err := app.ParseFade(c.text, c.fadeIn, tr)
		if err != nil || got != c.want {
			t.Errorf("ParseFade(%q, in=%v) = %+v, %v; want %+v", c.text, c.fadeIn, got, err, c.want)
		}
	}
	for _, bad := range []string{"soon", "3-", "5-2x", "-3", "1:2:3:4", "3 4"} {
		if got, err := app.ParseFade(bad, true, tr); err == nil {
			t.Errorf("ParseFade(%q) = %+v, want an error", bad, got)
		}
	}
}

func TestAFadeIsShownTheWayItCanBeTypedAgain(t *testing.T) {
	tr := manifest.Track{SourceLength: 10 * time.Second, TrimStart: 500 * time.Millisecond, TrimEnd: time.Second}
	for _, c := range []struct {
		span   manifest.Span
		fadeIn bool
		want   string
	}{
		{manifest.Span{}, true, ""},
		{manifest.Span{Start: 500 * time.Millisecond, End: 3500 * time.Millisecond}, true, "0:03"},
		{manifest.Span{Start: 6 * time.Second, End: 9 * time.Second}, false, "0:03"},
		{manifest.Span{Start: 2 * time.Second, End: 5500 * time.Millisecond}, true, "0:02-0:05.5"},
		{manifest.Span{Start: 6 * time.Second, End: 8 * time.Second}, false, "0:06-0:08"}, // ends before the cut does
	} {
		text := app.FadeText(c.span, c.fadeIn, tr)
		if text != c.want {
			t.Errorf("FadeText(%+v, in=%v) = %q, want %q", c.span, c.fadeIn, text, c.want)
		}
		if back, err := app.ParseFade(text, c.fadeIn, tr); err != nil || back != c.span {
			t.Errorf("%q reads back as %+v, %v; want %+v", text, back, err, c.span)
		}
	}
}

func TestAVolumeIsTypedAsANumberOfDecibels(t *testing.T) {
	for in, want := range map[string]float64{
		"3": 3, "-2.5": -2.5, "+6": 6, "3dB": 3, "3 dB": 3, "-3 DB": -3, " 1.5 ": 1.5, "0": 0, "": 0,
	} {
		if got, err := app.ParseGain(in); err != nil || got != want {
			t.Errorf("ParseGain(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"loud", "3 dBs", "--3", "1,5"} {
		if got, err := app.ParseGain(bad); err == nil {
			t.Errorf("ParseGain(%q) = %v, want an error", bad, got)
		}
	}
}

func TestAVolumeIsWrittenAsDecibelsAndSteps(t *testing.T) {
	for steps, want := range map[int]string{0: "0 dB", 2: "+3.0 dB", -2: "-3.0 dB", 1: "+1.5 dB", -16: "-24.1 dB"} {
		if got := app.VolumeText(steps); got != want {
			t.Errorf("VolumeText(%d) = %q, want %q", steps, got, want)
		}
	}
}
