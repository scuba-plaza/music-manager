package tui

import (
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
}

type appLibrary struct{ *app.App }

// NewLibrary returns the Library backed by a.
func NewLibrary(a *app.App) Library { return appLibrary{a} }

func (l appLibrary) Tracks() ([]manifest.Track, error) {
	m, err := l.Manifest()
	if err != nil {
		return nil, err
	}
	return m.Tracks, nil
}
