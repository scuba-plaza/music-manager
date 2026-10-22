package app

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"music-manager/internal/manifest"
)

// SetCover sets the cover image of a track, or removes it if image is empty.
// Only JPEG and PNG images are accepted.
func (a *App) SetCover(id string, image []byte) error {
	m, err := a.store.Load()
	if err != nil {
		return err
	}
	t, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("no track with id %q", id)
	}
	sha, err := a.storeCover(image)
	if err != nil {
		return err
	}
	t.CoverSHA256 = sha
	t.UpdatedAt = time.Now().UTC()
	m.Update(t)
	return a.store.Save(m)
}

// storeCover files an image in the cover store and returns its SHA; empty
// input gives an empty SHA.
func (a *App) storeCover(image []byte) (string, error) {
	if len(image) == 0 {
		return "", nil
	}
	if kind := http.DetectContentType(image); kind != "image/jpeg" && kind != "image/png" {
		return "", fmt.Errorf("cover must be a JPEG or PNG image, got %s", kind)
	}
	return a.covers.Put(bytes.NewReader(image))
}

// older reports whether a was added before b, breaking ties by ID.
func older(a, b manifest.Track) bool {
	if !a.AddedAt.Equal(b.AddedAt) {
		return a.AddedAt.Before(b.AddedAt)
	}
	return a.ID < b.ID
}
