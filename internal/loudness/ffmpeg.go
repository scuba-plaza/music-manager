package loudness

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// FFmpeg measures with the ffmpeg command-line tool.
type FFmpeg struct {
	Bin string // the executable; "ffmpeg" by default
}

func NewFFmpeg() FFmpeg { return FFmpeg{Bin: "ffmpeg"} }

// ErrNotInstalled means the program that measures is not there at all, which
// no track will get past, unlike a file it cannot read.
var ErrNotInstalled = errors.New("not installed")

type notInstalled struct{ bin string }

func (e notInstalled) Error() string {
	return e.bin + " not found in PATH: install it to measure loudness"
}

func (e notInstalled) Is(target error) bool { return target == ErrNotInstalled }

// Measure runs the audio through ffmpeg's EBU R128 meter. The file goes in on
// standard input, so nothing is written to disk.
func (f FFmpeg) Measure(audio []byte) (Measurement, error) {
	bin, err := exec.LookPath(f.Bin)
	if err != nil {
		return Measurement{}, notInstalled{f.Bin}
	}
	cmd := exec.Command(bin, "-hide_banner", "-nostats", "-i", "pipe:0",
		"-af", "ebur128=peak=true", "-f", "null", "-")
	var stderr bytes.Buffer
	cmd.Stdin, cmd.Stderr = bytes.NewReader(audio), &stderr
	if err := cmd.Run(); err != nil {
		if msg := lastLine(stderr.String()); msg != "" {
			return Measurement{}, fmt.Errorf("%s: %s", f.Bin, msg)
		}
		return Measurement{}, fmt.Errorf("%s: %w", f.Bin, err)
	}
	return Parse(stderr.String())
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
