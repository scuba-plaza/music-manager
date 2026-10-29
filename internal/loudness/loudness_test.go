package loudness_test

import (
	"errors"
	"os"
	"reflect"
	"testing"

	"music-manager/internal/loudness"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// The fixtures are what ffmpeg 8.1 printed for `-af ebur128=peak=true -f null -`
// on a 3 s sine and on 2 s of silence. Every frame line in them also carries an
// "I: -70.0 LUFS" in the middle; only the summary at the end is the answer.
func TestParseReadsTheSummaryOfARealRun(t *testing.T) {
	got, err := loudness.Parse(fixture(t, "ebur128-sine.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := loudness.Measurement{LUFS: -21.8, PeakDB: -18.1}
	if got != want {
		t.Errorf("= %+v, want %+v", got, want)
	}
}

func TestParseSaysSilenceCannotBeMeasured(t *testing.T) {
	_, err := loudness.Parse(fixture(t, "ebur128-silence.txt"))
	if !errors.Is(err, loudness.ErrUnmeasurable) {
		t.Errorf("err = %v, want ErrUnmeasurable", err)
	}
}

// ReplayGain 2.0 plays everything at -18 LUFS, so the gain is what is missing
// from there; the peak is an amplitude, 1.0 being full scale. The numbers were
// worked out by hand (-18 - LUFS, and 10^(dB/20)), not by this package.
func TestTagsAimAtMinus18LUFS(t *testing.T) {
	for _, c := range []struct {
		name string
		m    loudness.Measurement
		want map[string]string
	}{
		{"a quiet track is lifted", loudness.Measurement{LUFS: -21.8, PeakDB: -18.1}, map[string]string{
			"REPLAYGAIN_TRACK_GAIN":         "3.80 dB",
			"REPLAYGAIN_TRACK_PEAK":         "0.124451",
			"REPLAYGAIN_REFERENCE_LOUDNESS": "-18.00 LUFS",
		}},
		{"a loud track is pulled down, its peak over full scale", loudness.Measurement{LUFS: -7.4, PeakDB: 2.2}, map[string]string{
			"REPLAYGAIN_TRACK_GAIN":         "-10.60 dB",
			"REPLAYGAIN_TRACK_PEAK":         "1.288250",
			"REPLAYGAIN_REFERENCE_LOUDNESS": "-18.00 LUFS",
		}},
	} {
		if got := loudness.Tags(c.m); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestParseRefusesOutputWithoutASummary(t *testing.T) {
	for name, out := range map[string]string{
		"empty":        "",
		"only frames":  "[Parsed_ebur128_0 @ 0x1] t: 0.1  TARGET:-23 LUFS    M:-120.7 S:-120.7     I: -70.0 LUFS\n",
		"ffmpeg error": "Error opening input file x.mp3.\n",
	} {
		_, err := loudness.Parse(out)
		if err == nil || errors.Is(err, loudness.ErrUnmeasurable) {
			t.Errorf("%s: err = %v, want a parse error", name, err)
		}
	}
}
