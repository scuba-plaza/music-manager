package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"music-manager/internal/manifest"
	"music-manager/internal/timecode"
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

// Question is one thing to ask the user.
type Question struct {
	Key      string // stable name of the field: "title", "trim-start", ...
	Label    string
	Default  string // offered when the answer is left empty
	Required bool
	Validate func(answer string) error // never nil
}

// Prompter asks the user questions interactively.
type Prompter interface {
	// Ask asks for a piece of text.
	Ask(Question) (string, error)
	// Choose asks which of options is meant and returns its index.
	Choose(title string, options []string) (int, error)
}

// ErrAborted is returned by a Prompter when the user gives up.
var ErrAborted = errors.New("aborted")

// DuplicateError says the audio is already in the library.
type DuplicateError struct{ ID string }

func (e *DuplicateError) Error() string {
	return fmt.Sprintf("this audio is already in the library as %s", e.ID)
}

// AddRequest describes a track to download. Fields set in Edit are taken as
// given and never asked about.
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
		if s.coverSHA, err = a.storeCover(req.Cover); err != nil {
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
	if dup, ok := m.FindByAudio(d.staged.sha); ok {
		return manifest.Track{}, &DuplicateError{ID: dup.ID}
	}
	t, err := applyEdit(newTrack(d.staged, d.URL), e)
	if err != nil {
		return manifest.Track{}, err
	}
	if t.Status != manifest.Identified {
		return manifest.Track{}, missingFields(t)
	}
	if match := d.Match; match != nil && t.OriginalTitle == match.Title && t.OriginalArtist == match.Artist {
		t.MBRecordingID, t.MBReleaseID = match.RecordingID, match.ReleaseID
		if t.CoverSHA256 == "" && d.source != nil {
			if img, err := d.source.CoverFront(ctx, match.ReleaseID); err == nil && len(img) > 0 {
				t.CoverSHA256, _ = a.storeCover(img)
			}
		}
	}
	t = m.Add(t)
	return t, a.store.Save(m)
}

// Add downloads the audio at req.URL into the library as an identified
// track. Fields not given in req.Edit are asked with p, offering what the
// download reported as the default; with a nil p they simply take that
// default. Nothing is added if anything fails.
func (a *App) Add(ctx context.Context, req AddRequest, dl Downloader, p Prompter) (manifest.Track, error) {
	d, err := a.Fetch(ctx, req, dl)
	if err != nil {
		return manifest.Track{}, err
	}
	e, err := collect(req.Edit, d.Info, p)
	if err != nil {
		return manifest.Track{}, err
	}
	return a.Finish(ctx, d, e)
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
	if t.OriginalTitle == "" {
		missing = append(missing, "title")
	}
	if t.OriginalArtist == "" {
		missing = append(missing, "artist")
	}
	return fmt.Errorf("missing %s (pass --%s, or run in a terminal to be asked)",
		strings.Join(missing, " and "), strings.Join(missing, " and --"))
}

// collect completes e by asking p, or by taking the defaults if p is nil.
func collect(given Edit, info Info, p Prompter) (Edit, error) {
	e := given
	ask := func(q Question) (string, error) {
		if p == nil {
			return q.Default, nil
		}
		return p.Ask(q)
	}
	// name fills one of title/artist/album.
	name := func(key, label, def string, required bool, dst **string) error {
		if *dst != nil {
			return nil
		}
		v, err := ask(Question{Key: key, Label: label, Default: def, Required: required, Validate: nonEmptyIf(required)})
		if err != nil {
			return err
		}
		v = strings.TrimSpace(v)
		*dst = &v
		return nil
	}
	artist := info.Artist
	if artist == "" {
		artist = strings.TrimSuffix(info.Uploader, " - Topic")
	}
	if err := name("title", "Title", info.Title, true, &e.Title); err != nil {
		return e, err
	}
	if err := name("artist", "Artist", artist, true, &e.Artist); err != nil {
		return e, err
	}
	if err := name("album", "Album (empty for a single)", info.Album, false, &e.Album); err != nil {
		return e, err
	}

	number := func(key, label, def string, dst **int) error {
		if *dst != nil {
			return nil
		}
		v, err := ask(Question{Key: key, Label: label, Default: def, Validate: validCount})
		if err != nil {
			return err
		}
		if v = strings.TrimSpace(v); v != "" {
			n, _ := strconv.Atoi(v)
			*dst = &n
		}
		return nil
	}
	if err := number("track", "Track number", positive(info.TrackNumber), &e.TrackNumber); err != nil {
		return e, err
	}
	if err := number("of", "Tracks on the album", positive(info.TrackTotal), &e.TrackTotal); err != nil {
		return e, err
	}
	if e.Released == nil {
		v, err := ask(Question{Key: "released", Label: "Released (1999, 1999-05 or 1999-05-17, empty if unknown)", Default: info.Released, Validate: validDate})
		if err != nil {
			return e, err
		}
		v = strings.TrimSpace(v)
		e.Released = &v
	}
	if e.Genre == nil {
		v, err := ask(Question{Key: "genre", Label: "Genre (empty for none)"})
		if err != nil {
			return e, err
		}
		v = strings.TrimSpace(v)
		e.Genre = &v
	}

	trim := func(key, label string, dst **time.Duration) error {
		if *dst != nil {
			return nil
		}
		v, err := ask(Question{Key: key, Label: label, Validate: validTimecode})
		if err != nil {
			return err
		}
		if v = strings.TrimSpace(v); v != "" {
			d, _ := timecode.Parse(v)
			*dst = &d
		}
		return nil
	}
	if err := trim("trim-start", "Cut off the start (e.g. 0:12, empty for none)", &e.TrimStart); err != nil {
		return e, err
	}
	return e, trim("trim-end", "Cut off the end (e.g. 0:05, empty for none)", &e.TrimEnd)
}

// positive formats n, or gives "" if it is not known.
func positive(n int) string {
	if n > 0 {
		return strconv.Itoa(n)
	}
	return ""
}

func nonEmptyIf(required bool) func(string) error {
	return func(s string) error {
		if required && strings.TrimSpace(s) == "" {
			return errors.New("required")
		}
		return nil
	}
}

func validCount(s string) error {
	if s = strings.TrimSpace(s); s == "" {
		return nil
	}
	if n, err := strconv.Atoi(s); err != nil || n < 0 {
		return errors.New("enter a whole number, or leave empty")
	}
	return nil
}

func validTimecode(s string) error {
	if s = strings.TrimSpace(s); s == "" {
		return nil
	}
	_, err := timecode.Parse(s)
	return err
}

func validDate(s string) error {
	_, err := NormalizeDate(s)
	return err
}
