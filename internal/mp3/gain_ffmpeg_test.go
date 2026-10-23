package mp3_test

import (
	"bytes"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"music-manager/internal/mp3"
	"music-manager/internal/testmp3"
)

// These tests ask ffmpeg, a decoder that is not ours, whether what the gain
// code does to a file is what it should. They are skipped where there is no
// ffmpeg.

// ffmpeg runs it with the arguments and returns what it printed on stderr.
func ffmpeg(t *testing.T, args ...string) string {
	t.Helper()
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("no ffmpeg")
	}
	cmd := exec.Command(path, append([]string{"-hide_banner", "-nostdin", "-loglevel", "info"}, args...)...)
	for _, kv := range os.Environ() { // a library path meant for other programs can break it
		if len(kv) < 16 || kv[:16] != "LD_LIBRARY_PATH=" {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run() // frames made up for a test are allowed to fail to decode
	return stderr.String()
}

func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFfmpegAcceptsTheCrcOfAnAdjustedFrameAndRefusesAWrongOne(t *testing.T) {
	audio := testmp3.WithGains(testmp3.Layout{CRC: true}, ramp(40, 2, 120))
	out, err := mp3.Adjust(audio, mp3.Shape{Steps: 5})
	if err != nil {
		t.Fatal(err)
	}
	if log := ffmpeg(t, "-err_detect", "crccheck", "-i", writeTemp(t, "a.mp3", out), "-f", "null", "-"); regexp.MustCompile(`CRC mismatch`).MatchString(log) {
		t.Errorf("ffmpeg found a wrong CRC in the adjusted frames:\n%s", log)
	}

	// The same frames with the side information changed and the CRC left as
	// it was: the check is able to tell.
	bad := bytes.Clone(out)
	for f := 0; f < len(bad)/testmp3.FrameSize; f++ {
		bad[f*testmp3.FrameSize+10] ^= 0x55
	}
	if log := ffmpeg(t, "-err_detect", "crccheck", "-i", writeTemp(t, "b.mp3", bad), "-f", "null", "-"); !regexp.MustCompile(`CRC mismatch`).MatchString(log) {
		t.Errorf("ffmpeg did not mind a wrong CRC, so the test above proves nothing:\n%s", log)
	}
}

// sine encodes seconds of a 440 Hz tone, 128 kbps stereo, with ffmpeg and
// returns the file's audio.
func sine(t *testing.T, seconds int) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sine.mp3")
	ffmpeg(t, "-f", "lavfi", "-i", "sine=frequency=440:duration="+strconv.Itoa(seconds), "-ac", "2", "-c:a", "libmp3lame", "-b:a", "128k", "-y", path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skip("ffmpeg could not encode an MP3")
	}
	audio, _, err := mp3.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return audio
}

var meanVolume = regexp.MustCompile(`mean_volume: (-?[0-9.]+) dB`)

// level is the mean volume in dB that ffmpeg measures in the audio, or in the
// part of it that filter picks, such as "atrim=0:0.5".
func level(t *testing.T, audio []byte, filter string) float64 {
	t.Helper()
	chain := "volumedetect"
	if filter != "" {
		chain = filter + "," + chain
	}
	log := ffmpeg(t, "-i", writeTemp(t, "level.mp3", audio), "-af", chain, "-f", "null", "-")
	m := meanVolume.FindStringSubmatch(log)
	if m == nil {
		t.Fatalf("no volume in the output of ffmpeg:\n%s", log)
	}
	v, _ := strconv.ParseFloat(m[1], 64)
	return v
}

func TestFourStepsOfGainAreSixDecibelsToARealDecoder(t *testing.T) {
	audio := sine(t, 2)
	base := level(t, audio, "")
	for _, steps := range []int{4, -4, 7} {
		out, err := mp3.Adjust(audio, mp3.Shape{Steps: steps})
		if err != nil {
			t.Fatal(err)
		}
		got, want := level(t, out, "")-base, float64(steps)*mp3.StepDB
		if math.Abs(got-want) > 0.3 { // the decoder rounds to a tenth of a dB, twice
			t.Errorf("%d steps changed the level by %.1f dB, want %.1f", steps, got, want)
		}
	}
}

func TestAFadeIsHeardWhereItShouldBeToARealDecoder(t *testing.T) {
	audio := sine(t, 3)
	base := level(t, audio, "")

	in, err := mp3.Adjust(audio, mp3.Shape{FadeIn: mp3.Fade{End: time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		window string
		want   float64 // relative to the unfaded sound
	}{
		{"atrim=0.45:0.55", -6}, // half way up: half the amplitude
		{"atrim=0.2:0.3", -12},  // a quarter of the way: a quarter of it
		{"atrim=1.5:2.5", 0},    // after the fade
	} {
		if got := level(t, in, c.window) - base; math.Abs(got-c.want) > 1.5 { // the ramp goes by steps of 1.5 dB
			t.Errorf("fade-in, %s: %.1f dB from the full volume, want %.0f", c.window, got, c.want)
		}
	}

	out, err := mp3.Adjust(audio, mp3.Shape{FadeOut: mp3.Fade{Start: time.Second, End: 2 * time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		window string
		want   float64
	}{
		{"atrim=0.2:0.8", 0},
		{"atrim=1.45:1.55", -6},
	} {
		if got := level(t, out, c.window) - base; math.Abs(got-c.want) > 1.5 {
			t.Errorf("fade-out, %s: %.1f dB from the full volume, want %.0f", c.window, got, c.want)
		}
	}
	// After the fade-out there is nothing to measure: the stream is silent, and
	// the decoder says -91 dB (or -inf) for that.
	if got := level(t, out, "atrim=2.2:2.8"); got > base-50 {
		t.Errorf("after the fade-out: %.1f dB, want silence", got)
	}
}
