package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"music-manager/internal/manifest"
)

// Info is what the downloader learned about a download.
type Info struct {
	Title       string
	Artist      string
	Uploader    string
	Album       string
	TrackNumber int
	TrackTotal  int
	// Released is when the track came out, if the source says: 1999, 1999-05 or 1999-05-17.
	Released string
}

// Downloaded is an MP3 file fetched by a Downloader.
type Downloaded struct {
	Path string
	Info Info
}

// Downloader fetches the audio at a URL as an MP3 file into dir.
type Downloader interface {
	Download(ctx context.Context, url, dir string) (Downloaded, error)
}

// DuplicateError says the audio is already in the library.
type DuplicateError struct{ ID string }

func (e *DuplicateError) Error() string {
	return fmt.Sprintf("this audio is already in the library as %s", e.ID)
}

type AddRequest struct {
	URL   string
	Cover []byte // optional JPEG or PNG
}

// Draft is a downloaded track waiting for its details. Its audio is already
// in the library's store but it has no record yet; Finish creates the record.
// A draft that is never finished leaves a blob that the next sync collects.
type Draft struct {
	URL string
	// Info is what the download said, to offer as the defaults for the details.
	Info Info
	// Length is the length of the downloaded audio.
	Length time.Duration

	staged staged
}

// Fetch downloads the audio at req.URL and prepares a Draft, without adding
// anything to the library. It fails with a *DuplicateError if the audio is
// already there.
func (a *App) Fetch(ctx context.Context, req AddRequest, dl Downloader) (*Draft, error) {
	if err := os.MkdirAll(filepath.Join(a.root, ".mm"), 0o755); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(filepath.Join(a.root, ".mm"), "download-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	d, err := dl.Download(ctx, req.URL, dir)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", req.URL, err)
	}
	s, err := a.stage(d.Path)
	if err != nil {
		return nil, fmt.Errorf("downloaded file: %w", err)
	}
	m, err := a.store.Load()
	if err != nil {
		return nil, err
	}
	if dup, ok := m.FindByAudio(s.sha); ok {
		return nil, &DuplicateError{ID: dup.ID}
	}
	if len(req.Cover) > 0 {
		if s.coverSHA, err = a.StoreCover(req.Cover); err != nil {
			return nil, err
		}
	}

	return &Draft{URL: req.URL, Info: d.Info, Length: s.length, staged: s}, nil
}

// Finish records the draft as an identified track with the details in e.
// Nothing is added if e is invalid or the audio has meanwhile been added.
func (a *App) Finish(d *Draft, e Edit) (manifest.Track, error) {
	m, err := a.store.Load()
	if err != nil {
		return manifest.Track{}, err
	}
	if err := a.usable(m, d); err != nil { // so that a draft that cannot be added says so before anything is looked up
		return manifest.Track{}, err
	}
	t, err := applyEdit(newTrack(d.staged, d.URL), e)
	if err != nil {
		return manifest.Track{}, err
	}
	if t.Status != manifest.Identified {
		return manifest.Track{}, missingFields(t)
	}
	var added manifest.Track
	err = a.store.Update(func(m *manifest.Manifest) error {
		if err := a.usable(m, d); err != nil { // and again on the records as they are now
			return err
		}
		if t.CoverSHA256 != "" && !a.covers.Has(t.CoverSHA256) { // a cover looked up on the way, if a sync has cleared it away, is left out
			t.CoverSHA256 = ""
		}
		added = m.Add(t)
		return nil
	})
	if err != nil {
		return manifest.Track{}, err
	}
	return added, nil
}

// usable says whether the draft can still be added to the records m: its audio
// is not in them, and its files are still stored, which no record held until
// now, so a sync in between may have collected them.
func (a *App) usable(m *manifest.Manifest, d *Draft) error {
	if dup, ok := m.FindByAudio(d.staged.sha); ok {
		return &DuplicateError{ID: dup.ID}
	}
	if !a.stored(d.staged) {
		return errors.New("the downloaded audio was cleared away by a sync before it was added; download it again")
	}
	return nil
}

func missingFields(t manifest.Track) error {
	var missing []string
	if t.Title == "" {
		missing = append(missing, "title")
	}
	if t.Artist == "" {
		missing = append(missing, "artist")
	}
	return fmt.Errorf("missing %s", strings.Join(missing, " and "))
}
