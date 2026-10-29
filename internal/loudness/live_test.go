//go:build live

package loudness_test

import (
	"bytes"
	"errors"
	"os/exec"
	"testing"

	"music-manager/internal/loudness"
	"music-manager/internal/testmp3"
)

// Run with: go test -tags live ./internal/loudness
// It needs the real ffmpeg.
func TestLiveFFmpegMeasuresRealMP3s(t *testing.T) {
	ff := loudness.NewFFmpeg()

	if _, err := ff.Measure(testmp3.Silent(50)); !errors.Is(err, loudness.ErrUnmeasurable) {
		t.Errorf("silent MP3: err = %v, want ErrUnmeasurable", err)
	}

	var tone bytes.Buffer
	cmd := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=3",
		"-c:a", "libmp3lame", "-f", "mp3", "pipe:1")
	cmd.Stdout = &tone
	if err := cmd.Run(); err != nil {
		t.Skipf("ffmpeg cannot make a test tone: %v", err)
	}
	got, err := ff.Measure(tone.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	// A sine at ffmpeg's default level; the exact figure depends on the encoder.
	if got.LUFS < -24 || got.LUFS > -19 || got.PeakDB > -15 || got.PeakDB < -21 {
		t.Errorf("tone = %+v, want about -21.8 LUFS and a peak of about -18 dB", got)
	}
}
