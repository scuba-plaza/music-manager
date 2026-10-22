package mp3

import (
	"errors"
	"time"
)

// Trim returns the part of audio, as returned by Parse, that remains after
// cutting start off the beginning and end off the end. The cut is lossless
// and falls on frame boundaries: a frame is kept if its midpoint lies in the
// kept range, so each cut is off by at most half a frame (about 13ms). The
// result shares memory with audio.
func Trim(audio []byte, start, end time.Duration) ([]byte, error) {
	if start < 0 || end < 0 {
		return nil, errors.New("mp3: trims cannot be negative")
	}
	_, total, err := Parse(audio)
	if err != nil {
		return nil, err
	}
	var elapsed time.Duration
	from, to := -1, 0
	for pos := 0; pos < len(audio); {
		f, ok := frameAt(audio[pos:])
		if !ok || f.size > len(audio)-pos {
			break
		}
		if !f.info {
			d := time.Duration(f.samples) * time.Second / time.Duration(f.rate)
			if mid := elapsed + d/2; mid >= start && mid < total-end {
				if from < 0 {
					from = pos
				}
				to = pos + f.size
			}
			elapsed += d
		}
		pos += f.size
	}
	if from < 0 {
		return nil, errors.New("mp3: trims leave no audio")
	}
	return audio[from:to], nil
}
