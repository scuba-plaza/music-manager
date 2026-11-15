package webtest

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"music-manager/internal/app"
	"music-manager/internal/testmp3"
)

const defaultAudioFrames = 700

const DuplicateLink = "https://www.youtube.com/watch?v=duplicate"

type FakeDownloader struct {
	Audio []byte
	Info  app.Info
	Cover []byte

	mu    sync.Mutex
	calls []string
}

func (d *FakeDownloader) Download(_ context.Context, url, dir string) (app.Downloaded, error) {
	d.mu.Lock()
	d.calls = append(d.calls, url)
	d.mu.Unlock()
	audio := d.Audio
	if url == DuplicateLink {
		audio = testmp3.Silent(identifiedTracks[0].frames)
	}
	if audio == nil {
		audio = testmp3.Silent(defaultAudioFrames)
	}
	path := filepath.Join(dir, "vid.mp3")
	if err := os.WriteFile(path, audio, 0o644); err != nil {
		return app.Downloaded{}, err
	}
	return app.Downloaded{Path: path, Info: d.Info, Cover: d.Cover}, nil
}

func (d *FakeDownloader) Calls() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.calls)
}
