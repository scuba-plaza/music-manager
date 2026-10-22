package tui

import (
	"context"
	"errors"
	"os"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

// Library is everything the interface needs from the music library. The real
// one is *app.App behind NewLibrary; tests use that too, or wrap it to make
// a call fail.
type Library interface {
	// Root is the library folder.
	Root() string
	// Tracks returns every record.
	Tracks() ([]manifest.Track, error)
	// Edit changes a track and returns it as saved.
	Edit(id string, e app.Edit) (manifest.Track, error)
	// Restore puts a record back exactly as it was; undo is built on it.
	Restore(t manifest.Track) error
	// PreviewTrim writes the audio around one of a track's cuts to a
	// temporary file; the caller removes it with cleanup.
	PreviewTrim(id string, edge app.Edge, window time.Duration) (path string, cleanup func(), err error)
	// Lookup reports whether a music database can be searched at all.
	Lookup() bool
	// Candidates searches the database for what a track could be (or for q,
	// if given); ApplyCandidate identifies the track as one of them.
	Candidates(ctx context.Context, id string, q *app.Query) ([]app.Candidate, error)
	ApplyCandidate(ctx context.Context, id string, c app.Candidate) (manifest.Track, error)
	// IdentifyAll takes the confident match for every track that needs work,
	// telling each as it goes.
	IdentifyAll(ctx context.Context, each func(app.IdentifyResult, manifest.Track)) ([]app.IdentifyResult, error)
	// Adopt identifies tracks by the tags their files had.
	Adopt(ids []string) ([]app.AdoptResult, error)
	// RemoveMany removes tracks, all or none; their audio stays stored until
	// a sync collects it.
	RemoveMany(ids []string) ([]manifest.Track, error)
	// EditMany applies several edits at once: all of them or none.
	EditMany(edits map[string]app.Edit) (map[string]manifest.Track, error)
	// Renumber numbers the tracks 1..N in the order given.
	Renumber(ids []string) (map[string]manifest.Track, error)
	// SetCover sets a track's cover image (JPEG or PNG), or removes it if
	// image is empty, and returns the track as saved.
	SetCover(id string, image []byte) (manifest.Track, error)
	// CoverImage returns the stored cover image with the given SHA.
	CoverImage(sha string) ([]byte, error)
}

type appLibrary struct {
	*app.App
	source app.MetadataSource
}

// LibraryOption adds a capability to the library built by NewLibrary.
type LibraryOption func(*appLibrary)

// WithSource lets the library look tracks up in a music database.
func WithSource(s app.MetadataSource) LibraryOption { return func(l *appLibrary) { l.source = s } }

// NewLibrary returns the Library backed by a.
func NewLibrary(a *app.App, opts ...LibraryOption) Library {
	l := appLibrary{App: a}
	for _, o := range opts {
		o(&l)
	}
	return l
}

func (l appLibrary) Lookup() bool { return l.source != nil }

func (l appLibrary) Candidates(ctx context.Context, id string, q *app.Query) ([]app.Candidate, error) {
	if l.source == nil {
		return nil, errNoLookup
	}
	return l.App.Candidates(ctx, id, l.source, q)
}

func (l appLibrary) ApplyCandidate(ctx context.Context, id string, c app.Candidate) (manifest.Track, error) {
	if l.source == nil {
		return manifest.Track{}, errNoLookup
	}
	return l.App.ApplyCandidate(ctx, id, c, l.source)
}

func (l appLibrary) IdentifyAll(ctx context.Context, each func(app.IdentifyResult, manifest.Track)) ([]app.IdentifyResult, error) {
	if l.source == nil {
		return nil, errNoLookup
	}
	return l.App.Identify(ctx, nil, l.source, nil, app.IdentifyOptions{Yes: true, OnResult: func(r app.IdentifyResult) {
		var t manifest.Track
		if r.Applied != nil { // what was saved, for the screen to show
			if m, err := l.Manifest(); err == nil {
				t, _ = m.Get(r.TrackID)
			}
		}
		each(r, t)
	}})
}

func (l appLibrary) SetCover(id string, image []byte) (manifest.Track, error) {
	if err := l.App.SetCover(id, image); err != nil {
		return manifest.Track{}, err
	}
	m, err := l.Manifest()
	if err != nil {
		return manifest.Track{}, err
	}
	t, _ := m.Get(id)
	return t, nil
}

func (l appLibrary) CoverImage(sha string) ([]byte, error) { return os.ReadFile(l.CoverPath(sha)) }

func (l appLibrary) Tracks() ([]manifest.Track, error) {
	m, err := l.Manifest()
	if err != nil {
		return nil, err
	}
	return m.Tracks, nil
}

var errNoLookup = errors.New("lookup is not available")
