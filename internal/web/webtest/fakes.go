package webtest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/testmp3"
)

var soWhatCandidates = []app.Candidate{
	{
		RecordingID: "so-what-recording", ReleaseID: "kind-of-blue-release",
		Title: "So What", Artist: "Miles Davis", Album: "Kind of Blue",
		TrackNumber: 1, TrackTotal: 5, Length: 9*time.Minute + 22*time.Second, Score: 99, Date: "1959-08-17",
	},
	{
		RecordingID: "so-what-recording", ReleaseID: "kind-of-blue-legacy-release",
		Title: "So What", Artist: "Miles Davis", Album: "Kind of Blue (Legacy Edition)",
		TrackNumber: 1, TrackTotal: 9, Length: 9*time.Minute + 22*time.Second, Score: 90, Date: "1997-10-07",
	},
}

type FakeSource struct {
	Found   []app.Candidate
	FailFor string
}

func (s FakeSource) Search(_ context.Context, q app.Query) ([]app.Candidate, error) {
	if s.FailFor != "" && q.Title == s.FailFor {
		return nil, errors.New("network down")
	}
	if s.Found != nil {
		return slices.Clone(s.Found), nil
	}
	return slices.Clone(soWhatCandidates), nil
}

func (FakeSource) CoverFront(context.Context, string) ([]byte, error) { return nil, nil }

const defaultAudioFrames = 700

type FakeDownloader struct {
	Audio []byte
	Info  app.Info

	mu    sync.Mutex
	calls []string
}

func (d *FakeDownloader) Download(_ context.Context, url, dir string) (app.Downloaded, error) {
	d.mu.Lock()
	d.calls = append(d.calls, url)
	d.mu.Unlock()
	audio := d.Audio
	if audio == nil {
		audio = testmp3.Silent(defaultAudioFrames)
	}
	path := filepath.Join(dir, "vid.mp3")
	if err := os.WriteFile(path, audio, 0o644); err != nil {
		return app.Downloaded{}, err
	}
	return app.Downloaded{Path: path, Info: d.Info}, nil
}

func (d *FakeDownloader) Calls() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.calls)
}
