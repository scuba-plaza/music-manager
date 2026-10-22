package tui

import (
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

type appLibrary struct{ *app.App }

// NewLibrary returns the Library backed by a.
func NewLibrary(a *app.App) Library { return appLibrary{a} }

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
