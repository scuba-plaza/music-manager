package app_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/device"
	"music-manager/internal/loudness"
	"music-manager/internal/mp3"
	"music-manager/internal/testmp3"
)

// frames stands in for ffmpeg. The test tracks are runs of silent frames, so the
// number of frames in the audio it is given says which track it is.
type frames struct {
	answers   map[int]loudness.Measurement
	otherwise loudness.Measurement // for a length that is not in answers
	err       error                // for every track
	errs      map[int]error        // for the track of that many frames
	calls     int
}

func (f *frames) Measure(audio []byte) (loudness.Measurement, error) {
	f.calls++
	if f.err != nil {
		return loudness.Measurement{}, f.err
	}
	if err, ok := f.errs[len(audio)/testmp3.FrameSize]; ok {
		return loudness.Measurement{}, err
	}
	if m, ok := f.answers[len(audio)/testmp3.FrameSize]; ok {
		return m, nil
	}
	return f.otherwise, nil
}

// twoTracks is a loud and a calm track, identified, with a stand-in meter set.
func twoTracks(t *testing.T) (*app.App, *frames) {
	t.Helper()
	a := libWithTracks(t, 100, 60)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Loud", "Band", "", 0, 0)
	identify(t, a, ts[1].ID, "Calm", "Band", "", 0, 0)
	meter := &frames{answers: map[int]loudness.Measurement{
		100: {LUFS: -10, PeakDB: -1},
		60:  {LUFS: -20, PeakDB: -3},
	}}
	a.UseLoudness(meter)
	return a, meter
}

func replayGainTags(t *testing.T, a *app.App, rel string) []string {
	t.Helper()
	var names []string
	for name := range mp3.ReadTagSet(readFile(t, a.Root(), rel)).Extra {
		if strings.HasPrefix(name, "REPLAYGAIN_") {
			names = append(names, name)
		}
	}
	return names
}

// A track with nothing audible has no loudness: it is synced without tags, and
// that is nothing to warn about.
func TestASilentTrackIsSyncedWithoutTagsAndWithoutFuss(t *testing.T) {
	a, meter := twoTracks(t)
	meter.errs = map[int]error{60: loudness.ErrUnmeasurable}
	plan := mustPlan(t, a, app.SyncOptions{})
	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	if got := replayGainTags(t, a, "Band/Calm.mp3"); len(got) != 0 {
		t.Errorf("the silent track carries %v", got)
	}
	if got := replayGainTags(t, a, "Band/Loud.mp3"); len(got) != 3 {
		t.Errorf("the other track carries %v, want its 3 tags", got)
	}
	if len(plan.Warnings) != 0 {
		t.Errorf("warnings = %v, want none", plan.Warnings)
	}
}

// One file ffmpeg cannot read must not stop the other 191 from being synced; it
// is reported, by track, in the plan.
func TestATrackThatCannotBeMeasuredIsSyncedWithoutTagsAndReported(t *testing.T) {
	a, meter := twoTracks(t)
	meter.errs = map[int]error{60: errors.New("ffmpeg: Invalid data found when processing input")}
	plan := mustPlan(t, a, app.SyncOptions{})
	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	if got := replayGainTags(t, a, "Band/Calm.mp3"); len(got) != 0 {
		t.Errorf("the unreadable track carries %v", got)
	}
	if got := replayGainTags(t, a, "Band/Loud.mp3"); len(got) != 3 {
		t.Errorf("the other track carries %v", got)
	}
	calm := tracks(t, a)[1].ID
	if len(plan.Warnings) != 1 || plan.Warnings[0].TrackID != calm ||
		!strings.Contains(plan.Warnings[0].Message, "Invalid data found") {
		t.Errorf("warnings = %+v, want one for track %s with ffmpeg's words", plan.Warnings, calm)
	}
}

// Without ffmpeg every track would lose its tags at once, so the sync stops
// before it writes anything.
func TestSyncStopsWhenTheMeterIsNotInstalled(t *testing.T) {
	a, meter := twoTracks(t)
	meter.err = loudness.ErrNotInstalled
	if _, err := a.PlanSync(app.SyncOptions{}); !errors.Is(err, loudness.ErrNotInstalled) {
		t.Errorf("err = %v, want it to be ErrNotInstalled", err)
	}
}

// A sync renders every track twice, to plan and to apply, and runs again and again;
// ffmpeg should only hear a track when its sound is new. A cut changes the sound:
// -15 LUFS is what the stand-in says for any other length, and -18 - (-15) = -3.
func TestEachTrackIsMeasuredOnlyWhenItsSoundIsNew(t *testing.T) {
	a, meter := twoTracks(t)
	meter.otherwise = loudness.Measurement{LUFS: -15, PeakDB: -2}
	mustSync(t, a, app.SyncOptions{})
	if meter.calls != 2 {
		t.Fatalf("after the first sync the meter ran %d times, want 2", meter.calls)
	}
	if ops := mustSync(t, a, app.SyncOptions{}); len(ops) != 0 || meter.calls != 2 {
		t.Fatalf("a second sync wants %v and ran the meter %d times", ops, meter.calls)
	}

	start := time.Second
	if _, err := a.Edit(tracks(t, a)[0].ID, app.Edit{TrimStart: &start}); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, app.SyncOptions{})
	if meter.calls != 3 {
		t.Errorf("after one cut the meter ran %d times in all, want 3", meter.calls)
	}
	if got := mp3.ReadTagSet(readFile(t, a.Root(), "Band/Loud.mp3")).Extra["REPLAYGAIN_TRACK_GAIN"]; got != "-3.00 dB" {
		t.Errorf("the cut track's gain = %q, want -3.00 dB", got)
	}
	if got := mp3.ReadTagSet(readFile(t, a.Root(), "Band/Calm.mp3")).Extra["REPLAYGAIN_TRACK_GAIN"]; got != "2.00 dB" {
		t.Errorf("the other track's gain changed to %q", got)
	}
}

// Turning the tags on rewrites every file, and the "to sync" count says so.
func TestUnsyncedNoticesThatReplayGainWasTurnedOn(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	mustSync(t, a, app.SyncOptions{})
	if n, _ := unsynced(t, a); n != 0 {
		t.Fatalf("before = %d", n)
	}
	a.UseLoudness(&frames{answers: map[int]loudness.Measurement{10: {LUFS: -12, PeakDB: -1}, 20: {LUFS: -14, PeakDB: -1}}})
	if n, _ := unsynced(t, a); n != 2 {
		t.Errorf("with the meter on = %d, want both tracks waiting", n)
	}
	mustSync(t, a, app.SyncOptions{})
	if n, _ := unsynced(t, a); n != 0 {
		t.Errorf("after syncing = %d", n)
	}
}

// The Wolfgang shows no volume setting and its tags are measured to the byte:
// it gets what it always did, and nobody measures anything for it.
func TestWolfgangSyncCarriesNoReplayGain(t *testing.T) {
	a, meter := twoTracks(t)
	card := t.TempDir()
	mustSync(t, a, app.SyncOptions{Target: card, Device: device.Wolfgang})

	files, _ := filepath.Glob(filepath.Join(card, "*", "*.mp3"))
	if len(files) != 2 {
		t.Fatalf("card holds %v, want 2 tracks", files)
	}
	for _, f := range files {
		data, _ := os.ReadFile(f)
		for name := range mp3.ReadTagSet(data).Extra {
			if strings.HasPrefix(name, "REPLAYGAIN_") {
				t.Errorf("%s carries %s", filepath.Base(f), name)
			}
		}
	}
	if meter.calls != 0 {
		t.Errorf("the meter ran %d times for the Wolfgang", meter.calls)
	}
}

// -18 - (-10) = -8 and -18 - (-20) = 2; 10^(-1/20) and 10^(-3/20), by hand.
func TestSyncWritesReplayGainTagsFromTheMeasuredLoudness(t *testing.T) {
	a, _ := twoTracks(t)
	mustSync(t, a, app.SyncOptions{})

	loud := mp3.ReadTagSet(readFile(t, a.Root(), "Band/Loud.mp3")).Extra
	if loud["REPLAYGAIN_TRACK_GAIN"] != "-8.00 dB" || loud["REPLAYGAIN_TRACK_PEAK"] != "0.891251" ||
		loud["REPLAYGAIN_REFERENCE_LOUDNESS"] != "-18.00 LUFS" {
		t.Errorf("loud track: %v", loud)
	}
	calm := mp3.ReadTagSet(readFile(t, a.Root(), "Band/Calm.mp3")).Extra
	if calm["REPLAYGAIN_TRACK_GAIN"] != "2.00 dB" || calm["REPLAYGAIN_TRACK_PEAK"] != "0.707946" {
		t.Errorf("calm track: %v", calm)
	}
	if loud["MM_ID"] == "" {
		t.Errorf("the other tags are gone: %v", loud)
	}
}
