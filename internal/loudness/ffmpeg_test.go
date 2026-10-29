package loudness_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"music-manager/internal/loudness"
)

// stub writes an executable shell script that stands in for ffmpeg.
func stub(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffmpeg")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFFmpegFeedsTheAudioInAndReadsTheSummary(t *testing.T) {
	seen := filepath.Join(t.TempDir(), "stdin")
	fixturePath, _ := filepath.Abs("testdata/ebur128-sine.txt")
	bin := stub(t, `cat > "`+seen+`"; cat "`+fixturePath+`" >&2`)

	got, err := loudness.FFmpeg{Bin: bin}.Measure([]byte("the mp3 bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if got.LUFS != -21.8 || got.PeakDB != -18.1 {
		t.Errorf("= %+v", got)
	}
	if data, _ := os.ReadFile(seen); string(data) != "the mp3 bytes" {
		t.Errorf("ffmpeg was given %q on stdin", data)
	}
}

func TestFFmpegSaysWhatWentWrong(t *testing.T) {
	_, err := loudness.FFmpeg{Bin: stub(t, "cat >/dev/null; echo 'Invalid data found when processing input' >&2; exit 1")}.Measure([]byte("x"))
	if err == nil || !strings.Contains(err.Error(), "Invalid data found") {
		t.Errorf("err = %v, want ffmpeg's last words", err)
	}
}

func TestFFmpegMissingIsExplained(t *testing.T) {
	_, err := loudness.FFmpeg{Bin: "no-such-ffmpeg-here"}.Measure([]byte("x"))
	if err == nil || !strings.Contains(err.Error(), "no-such-ffmpeg-here not found in PATH") {
		t.Errorf("err = %v, want it to say the program is missing", err)
	}
	if !errors.Is(err, loudness.ErrNotInstalled) {
		t.Errorf("err = %v, want it to be ErrNotInstalled so that callers can stop", err)
	}
	// A failure on one file is something else.
	_, err = loudness.FFmpeg{Bin: stub(t, "cat >/dev/null; exit 1")}.Measure([]byte("x"))
	if errors.Is(err, loudness.ErrNotInstalled) {
		t.Errorf("a failing ffmpeg was taken for a missing one: %v", err)
	}
}
