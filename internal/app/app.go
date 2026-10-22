// Package app holds the use cases of the music manager. It works on a
// library directory and takes its external collaborators as interfaces.
package app

import (
	"os"
	"path/filepath"

	"music-manager/internal/blobstore"
	"music-manager/internal/manifest"
)

// App is a music library rooted at a directory.
type App struct {
	root   string
	store  *manifest.Store
	audio  *blobstore.Store
	covers *blobstore.Store
}

// Open returns the library at root, creating the directory if needed.
func Open(root string) (*App, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &App{
		root:   root,
		store:  manifest.NewStore(filepath.Join(root, "library.json")),
		audio:  blobstore.New(filepath.Join(root, ".mm", "audio"), ".mp3"),
		covers: blobstore.New(filepath.Join(root, ".mm", "covers"), ".img"),
	}, nil
}

func (a *App) Manifest() (*manifest.Manifest, error) { return a.store.Load() }

// AudioPath is where the stored audio with the given SHA lives.
func (a *App) AudioPath(sha string) string { return a.audio.Path(sha) }
