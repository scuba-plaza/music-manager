package app

import (
	"fmt"
	"strconv"
	"strings"

	"music-manager/internal/manifest"
	"music-manager/internal/mp3"
	"music-manager/internal/timecode"
)

// shaped is audio, the whole original of t, with the volume and fades the
// record asks for. They are in the time of the original, so this comes
// before the cuts.
func shaped(t manifest.Track, audio []byte) ([]byte, error) {
	shape := mp3.Shape{Steps: t.GainSteps, FadeIn: mp3.Fade(t.FadeIn), FadeOut: mp3.Fade(t.FadeOut)}
	if shape == (mp3.Shape{}) {
		return audio, nil
	}
	return mp3.Adjust(audio, shape)
}

// ParseFade reads what was typed for a fade of t: nothing (or 0) for none, a
// length ("3", "0:03.5"), which counts from where the track starts for a
// fade-in and back from where it ends for a fade-out, or a stretch of the
// original ("0:02-0:05").
func ParseFade(text string, fadeIn bool, t manifest.Track) (manifest.Span, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return manifest.Span{}, nil
	}
	if from, to, ok := strings.Cut(text, "-"); ok {
		start, err := timecode.Parse(from)
		if err != nil {
			return manifest.Span{}, err
		}
		end, err := timecode.Parse(to)
		if err != nil {
			return manifest.Span{}, err
		}
		return manifest.Span{Start: start, End: end}, nil
	}
	length, err := timecode.Parse(text)
	if err != nil || length == 0 {
		return manifest.Span{}, err
	}
	if fadeIn {
		return manifest.Span{Start: t.TrimStart, End: t.TrimStart + length}, nil
	}
	end := t.SourceLength - t.TrimEnd
	return manifest.Span{Start: end - length, End: end}, nil
}

// FadeText writes a fade the way ParseFade reads it: as a length if it
// begins where the track does (a fade-in) or ends where it does (a
// fade-out), else as the stretch it covers.
func FadeText(s manifest.Span, fadeIn bool, t manifest.Track) string {
	switch {
	case s == (manifest.Span{}):
		return ""
	case fadeIn && s.Start == t.TrimStart, !fadeIn && s.End == t.SourceLength-t.TrimEnd:
		return timecode.Format(s.End - s.Start)
	}
	return timecode.Format(s.Start) + "-" + timecode.Format(s.End)
}

// ParseGain reads a number of dB such as 3, -2.5, +6 or 3dB; nothing is 0.
func ParseGain(s string) (float64, error) {
	num := strings.TrimSpace(s)
	if len(num) >= 2 && strings.EqualFold(num[len(num)-2:], "db") {
		num = strings.TrimSpace(num[:len(num)-2])
	}
	if num == "" {
		return 0, nil
	}
	db, err := strconv.ParseFloat(strings.TrimPrefix(num, "+"), 64)
	if err != nil || strings.HasPrefix(strings.TrimPrefix(num, "+"), "+") {
		return 0, fmt.Errorf("%q is not a number of dB", s)
	}
	return db, nil
}

// VolumeText writes a change of volume of this many steps in dB.
func VolumeText(steps int) string {
	if steps == 0 {
		return "0 dB"
	}
	return fmt.Sprintf("%+.1f dB", float64(steps)*mp3.StepDB)
}
