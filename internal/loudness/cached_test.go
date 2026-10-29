package loudness_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"music-manager/internal/loudness"
)

// counting stands in for ffmpeg: it answers from a table keyed by the audio and
// counts how often it was asked.
type counting struct {
	answers map[string]loudness.Measurement
	err     error
	calls   int
}

func (c *counting) Measure(audio []byte) (loudness.Measurement, error) {
	c.calls++
	if c.err != nil {
		return loudness.Measurement{}, c.err
	}
	return c.answers[string(audio)], nil
}

func TestCachedMeasuresEachAudioOnceEvenAcrossRuns(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "loudness") // not there yet
	inner := &counting{answers: map[string]loudness.Measurement{
		"song": {LUFS: -9.5, PeakDB: -0.4},
		"calm": {LUFS: -20.1, PeakDB: -3.3},
	}}

	first := loudness.Cached(dir, inner)
	for range 3 {
		got, err := first.Measure([]byte("song"))
		if err != nil || got != (loudness.Measurement{LUFS: -9.5, PeakDB: -0.4}) {
			t.Fatalf("song = %+v, %v", got, err)
		}
	}
	if got, _ := first.Measure([]byte("calm")); got.LUFS != -20.1 {
		t.Errorf("another audio got %+v: it must not share the answer", got)
	}
	if inner.calls != 2 {
		t.Errorf("ffmpeg ran %d times, want 2 (once per distinct audio)", inner.calls)
	}

	// A later run of mm has the answers on disk and asks nobody.
	second := loudness.Cached(dir, inner)
	if got, err := second.Measure([]byte("song")); err != nil || got.LUFS != -9.5 {
		t.Errorf("after a restart = %+v, %v", got, err)
	}
	if inner.calls != 2 {
		t.Errorf("a new run measured again: %d calls", inner.calls)
	}
}

func TestCachedRemembersThatAudioIsSilent(t *testing.T) {
	dir := t.TempDir()
	inner := &counting{err: loudness.ErrUnmeasurable}
	for range 2 {
		_, err := loudness.Cached(dir, inner).Measure([]byte("hush"))
		if !errors.Is(err, loudness.ErrUnmeasurable) {
			t.Fatalf("err = %v, want ErrUnmeasurable", err)
		}
	}
	if inner.calls != 1 {
		t.Errorf("silence was measured %d times, want once", inner.calls)
	}
}

func TestCachedDoesNotRememberAFailure(t *testing.T) {
	dir := t.TempDir()
	broken := &counting{err: errors.New("ffmpeg: not found")}
	if _, err := loudness.Cached(dir, broken).Measure([]byte("song")); err == nil {
		t.Fatal("the failure was swallowed")
	}
	fixed := &counting{answers: map[string]loudness.Measurement{"song": {LUFS: -12, PeakDB: -1}}}
	got, err := loudness.Cached(dir, fixed).Measure([]byte("song"))
	if err != nil || got.LUFS != -12 {
		t.Errorf("after the tool came back = %+v, %v", got, err)
	}
}

func TestCachedMeasuresAgainWhenAnEntryIsDamaged(t *testing.T) {
	dir := t.TempDir()
	inner := &counting{answers: map[string]loudness.Measurement{"song": {LUFS: -9.5, PeakDB: -0.4}}}
	loudness.Cached(dir, inner).Measure([]byte("song"))

	entries, _ := filepath.Glob(filepath.Join(dir, "*"))
	if len(entries) != 1 {
		t.Fatalf("cache holds %v, want one entry", entries)
	}
	if err := os.WriteFile(entries[0], []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := loudness.Cached(dir, inner).Measure([]byte("song"))
	if err != nil || got.LUFS != -9.5 {
		t.Errorf("= %+v, %v", got, err)
	}
	if inner.calls != 2 {
		t.Errorf("a damaged entry was trusted: %d calls", inner.calls)
	}
}
