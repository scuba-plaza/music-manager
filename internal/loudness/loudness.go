// Package loudness measures how loud a track is, with ffmpeg, and says what
// ReplayGain tags that calls for.
package loudness

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Measurement is how loud a stretch of audio is.
type Measurement struct {
	// LUFS is the integrated loudness (EBU R128), gated so that silence does
	// not count.
	LUFS float64
	// PeakDB is the true peak in dB relative to full scale; above 0 means the
	// reconstructed signal goes over.
	PeakDB float64
}

// ErrUnmeasurable means the audio has no sound above the silence gate, so it
// has no loudness to speak of.
var ErrUnmeasurable = errors.New("too quiet to measure")

// silenceFloor is what ffmpeg prints when nothing is above the absolute gate
// of -70 LUFS.
const silenceFloor = -70.0

var (
	integrated = regexp.MustCompile(`(?m)^\s+I:\s+(\S+)\s+LUFS\s*$`)
	truePeak   = regexp.MustCompile(`(?m)^\s+Peak:\s+(\S+)\s+dBFS\s*$`)
)

// ReferenceLUFS is the loudness ReplayGain 2.0 plays everything at.
const ReferenceLUFS = -18.0

// Tags are the ReplayGain frames of a track measured as m, by the name of their
// TXXX frame: the gain that brings it to ReferenceLUFS and its peak as an
// amplitude (1.0 is full scale).
func Tags(m Measurement) map[string]string {
	return map[string]string{
		"REPLAYGAIN_TRACK_GAIN":         fmt.Sprintf("%.2f dB", ReferenceLUFS-m.LUFS),
		"REPLAYGAIN_TRACK_PEAK":         fmt.Sprintf("%.6f", math.Pow(10, m.PeakDB/20)),
		"REPLAYGAIN_REFERENCE_LOUDNESS": fmt.Sprintf("%.2f LUFS", ReferenceLUFS),
	}
}

// Parse reads what ffmpeg printed for `-af ebur128=peak=true`. The frame lines
// it prints along the way carry running values and are ignored; the answer is
// the summary at the end.
func Parse(output string) (Measurement, error) {
	_, summary, found := cutLast(output, "Summary:")
	if !found {
		return Measurement{}, errors.New("ffmpeg printed no loudness summary")
	}
	lufs, err := number(integrated, summary, "integrated loudness")
	if err != nil {
		return Measurement{}, err
	}
	peak, err := number(truePeak, summary, "true peak")
	if err != nil {
		return Measurement{}, err
	}
	if lufs <= silenceFloor || math.IsInf(peak, -1) {
		return Measurement{}, ErrUnmeasurable
	}
	return Measurement{LUFS: lufs, PeakDB: peak}, nil
}

func number(re *regexp.Regexp, text, what string) (float64, error) {
	m := re.FindStringSubmatch(text)
	if m == nil {
		return 0, fmt.Errorf("ffmpeg's loudness summary has no %s", what)
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil || math.IsNaN(v) {
		return 0, fmt.Errorf("ffmpeg's loudness summary has an unreadable %s: %q", what, m[1])
	}
	return v, nil
}

// cutLast splits s around the last occurrence of sep.
func cutLast(s, sep string) (before, after string, found bool) {
	i := strings.LastIndex(s, sep)
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+len(sep):], true
}
