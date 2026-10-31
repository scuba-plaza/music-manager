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

// AddRequest describes a track to download.
type AddRequest struct {
	URL   string
	Edit  Edit
	Cover []byte // optional JPEG or PNG
	// Source, if set, is searched for the track; a confident match replaces
	// what the download reported as the defaults.
	Source MetadataSource
	// OnWarning, if set, hears about problems that do not stop the add.
	OnWarning func(error)
}

// Draft is a downloaded track waiting for its details. Its audio is already
// in the library's store but it has no record yet; Finish creates the record.
// A draft that is never finished leaves a blob that the next sync collects.
type Draft struct {
	URL string
	// Info is what to offer as the defaults for the details: the
	// MusicBrainz match if there is one, otherwise what the download said.
	Info Info
	// Downloaded is what the download itself reported, for when the match is
	// declined.
	Downloaded Info
	// Match is a confident MusicBrainz match, or nil.
	Match *Candidate
	// Length is the length of the downloaded audio.
	Length time.Duration

	staged staged
	source MetadataSource
}

// Fetch downloads the audio at req.URL and prepares a Draft, without adding
// anything to the library. It fails with a *DuplicateError if the audio is
// already there. Fields set in req.Edit only steer the MusicBrainz search.
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

	draft := &Draft{URL: req.URL, Info: d.Info, Downloaded: d.Info, Length: s.length, staged: s, source: req.Source}
	if req.Source != nil && (req.Edit.Title == nil || req.Edit.Artist == nil) {
		if draft.Match = a.lookup(ctx, req, d.Info); draft.Match != nil {
			draft.Info = infoFrom(*draft.Match)
		}
	}
	return draft, nil
}

// Finish records the draft as an identified track with the details in e.
// Nothing is added if e is invalid or the audio has meanwhile been added.
func (a *App) Finish(ctx context.Context, d *Draft, e Edit) (manifest.Track, error) {
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
	if match := d.Match; match != nil && t.Title == match.Title && t.Artist == match.Artist {
		t.MBRecordingID, t.MBReleaseID = match.RecordingID, match.ReleaseID
		if t.CoverSHA256 == "" && d.source != nil {
			if img, err := d.source.CoverFront(ctx, match.ReleaseID); err == nil && len(img) > 0 {
				t.CoverSHA256, _ = a.StoreCover(img)
			}
		}
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

// Add downloads the audio at req.URL into the library as an identified
// track. Nothing is added if anything fails.
func (a *App) Add(ctx context.Context, req AddRequest, dl Downloader) (manifest.Track, error) {
	d, err := a.Fetch(ctx, req, dl)
	if err != nil {
		return manifest.Track{}, err
	}
	return a.Finish(ctx, d, withDefaults(req.Edit, d.Info))
}

// lookup searches req.Source for what the download is, returning the best
// match if it is a confident one. Failures only produce a warning.
func (a *App) lookup(ctx context.Context, req AddRequest, info Info) *Candidate {
	q := Query{Title: info.Title, Artist: info.Artist}
	if q.Artist == "" {
		q.Artist = strings.TrimSuffix(info.Uploader, " - Topic")
	}
	if req.Edit.Title != nil {
		q.Title = *req.Edit.Title
	}
	if req.Edit.Artist != nil {
		q.Artist = *req.Edit.Artist
	}
	cands, err := req.Source.Search(ctx, q)
	if err != nil {
		if req.OnWarning != nil {
			req.OnWarning(fmt.Errorf("lookup failed, using what the download said: %w", err))
		}
		return nil
	}
	if len(cands) == 0 || cands[0].Score < defaultMinScore {
		return nil
	}
	return &cands[0]
}

func infoFrom(c Candidate) Info {
	return Info{
		Title: c.Title, Artist: c.Artist, Album: c.Album,
		TrackNumber: c.TrackNumber, TrackTotal: c.TrackTotal, Released: leadingDate(c.Date),
	}
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

func withDefaults(given Edit, info Info) Edit {
	e := given
	artist := info.Artist
	if artist == "" {
		artist = strings.TrimSuffix(info.Uploader, " - Topic")
	}
	text := func(dst **string, v string) {
		if *dst == nil {
			v = strings.TrimSpace(v)
			*dst = &v
		}
	}
	count := func(dst **int, n int) {
		if *dst == nil && n > 0 {
			*dst = &n
		}
	}
	text(&e.Title, info.Title)
	text(&e.Artist, artist)
	text(&e.Album, info.Album)
	count(&e.TrackNumber, info.TrackNumber)
	count(&e.TrackTotal, info.TrackTotal)
	text(&e.Released, info.Released)
	text(&e.Genre, "")
	return e
}
