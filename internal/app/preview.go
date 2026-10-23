package app

import (
	"errors"
	"fmt"
	"os"
	"time"

	"music-manager/internal/mp3"
)

// Edge names one of the two cuts of a track.
type Edge int

const (
	EdgeStart Edge = iota // the cut at the start of the track
	EdgeEnd               // the cut at the end of the track
)

// PreviewTrim writes the audio around one of a track's cuts to a temporary
// MP3 so it can be played: with EdgeStart, the first window of what is kept;
// with EdgeEnd, the last window. It is cut by the same code that sync uses,
// so it sounds exactly as the synced file will. The caller must call cleanup
// when done with the file.
func (a *App) PreviewTrim(id string, edge Edge, window time.Duration) (path string, cleanup func(), err error) {
	if window <= 0 {
		return "", nil, errors.New("the preview window must be longer than zero")
	}
	m, err := a.store.Load()
	if err != nil {
		return "", nil, err
	}
	t, ok := m.Get(id)
	if !ok {
		return "", nil, fmt.Errorf("no track with id %q", id)
	}
	audio, err := os.ReadFile(a.audio.Path(t.AudioSHA256))
	if err != nil {
		return "", nil, err
	}
	if audio, err = shaped(t, audio); err != nil {
		return "", nil, err
	}

	start, end := t.TrimStart, t.TrimEnd
	switch edge {
	case EdgeStart:
		end = max(end, t.SourceLength-start-window)
	case EdgeEnd:
		start = max(start, t.SourceLength-end-window)
	}
	clip, err := mp3.Trim(audio, start, end)
	if err != nil {
		return "", nil, err
	}

	f, err := os.CreateTemp("", "mm-preview-*.mp3")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { os.Remove(f.Name()) }
	if _, err := f.Write(clip); err != nil {
		f.Close()
		cleanup()
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return f.Name(), cleanup, nil
}
