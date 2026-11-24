package webtest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/testmp3"
)

const defaultAudioFrames = 700

const DuplicateLink = "https://www.youtube.com/watch?v=duplicate"

const TooLongLink = "https://www.youtube.com/playlist?list=toolong"

func KindOfBlue() app.Listing {
	titles := []string{"So What", "Freddie Freeloader", "Blue in Green", "All Blues", "Flamenco Sketches"}
	l := app.Listing{Title: "Kind of Blue", Uploader: "Miles Davis - Topic", Cover: testmp3.JPEG}
	for i, title := range titles {
		l.Tracks = append(l.Tracks, app.ListedTrack{
			URL:    fmt.Sprintf("https://www.youtube.com/watch?v=kob%d", i+1),
			Title:  title,
			Length: 5*time.Minute + time.Duration(i)*time.Second,
		})
	}
	return l
}

type FakeDownloader struct {
	Audio []byte
	Info  app.Info
	Cover []byte

	Listing app.Listing
	Hold    chan struct{}

	mu        sync.Mutex
	calls     []string
	listCalls []string
	links     []string
}

func (d *FakeDownloader) Download(ctx context.Context, url, dir string) (app.Downloaded, error) {
	d.mu.Lock()
	d.calls = append(d.calls, url)
	at := slices.Index(d.links, url)
	if at < 0 && url != DuplicateLink {
		d.links = append(d.links, url)
		at = len(d.links) - 1
	}
	d.mu.Unlock()
	if d.Hold != nil {
		select {
		case <-d.Hold:
		case <-ctx.Done():
			return app.Downloaded{}, ctx.Err()
		}
	}
	audio := d.Audio
	if url == DuplicateLink {
		audio = testmp3.Silent(identifiedTracks[0].frames)
	}
	if audio == nil {
		audio = testmp3.Silent(defaultAudioFrames + 10*at)
	}
	path := filepath.Join(dir, "vid.mp3")
	if err := os.WriteFile(path, audio, 0o644); err != nil {
		return app.Downloaded{}, err
	}
	return app.Downloaded{Path: path, Info: d.Info, Cover: d.Cover}, nil
}

func (d *FakeDownloader) List(_ context.Context, url, _ string, limit int) (app.Listing, error) {
	d.mu.Lock()
	d.listCalls = append(d.listCalls, url)
	d.mu.Unlock()
	l := d.Listing
	if url == TooLongLink {
		l = app.Listing{Title: "Too Long", Uploader: "Someone"}
		for i := 0; i < 40; i++ {
			l.Tracks = append(l.Tracks, app.ListedTrack{
				URL:    fmt.Sprintf("https://www.youtube.com/watch?v=long%d", i+1),
				Title:  fmt.Sprintf("Track %d", i+1),
				Length: 3 * time.Minute,
			})
		}
	}
	if limit >= 0 && len(l.Tracks) > limit {
		l.Tracks = slices.Clone(l.Tracks[:limit])
	}
	return l, nil
}

func (d *FakeDownloader) ListCalls() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.listCalls)
}

func (d *FakeDownloader) Calls() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.calls)
}
