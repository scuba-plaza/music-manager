package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"music-manager/internal/cli"
	"music-manager/internal/loudness"
	"music-manager/internal/mp3"
)

type fixedLoudness loudness.Measurement

func (f fixedLoudness) Measure([]byte) (loudness.Measurement, error) {
	return loudness.Measurement(f), nil
}

// -18 - (-14) = -4. The meter comes in through Deps like the downloader does, so
// the program decides which one is real.
func TestSyncWritesReplayGainWhenTheProgramHasAMeter(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	id := ingestOne(t, lib)
	run(t, lib, "edit", id, "--title", "Song", "--artist", "Band")

	deps := cli.Deps{Loudness: fixedLoudness{LUFS: -14, PeakDB: -1}}
	if code, _, errs := runWith(t, deps, lib, "sync"); code != 0 {
		t.Fatalf("sync: code %d, err %q", code, errs)
	}
	data, err := os.ReadFile(filepath.Join(lib, "Band", "Song.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	if got := mp3.ReadTagSet(data).Extra["REPLAYGAIN_TRACK_GAIN"]; got != "-4.00 dB" {
		t.Errorf("REPLAYGAIN_TRACK_GAIN = %q, want -4.00 dB", got)
	}
}
