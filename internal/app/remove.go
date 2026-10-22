package app

import (
	"fmt"

	"music-manager/internal/manifest"
)

// Remove deletes the record of a track. Its audio and cover stay in the
// stores until the next sync collects them, and its rendered file is a stray
// that sync --prune deletes.
func (a *App) Remove(id string) (manifest.Track, error) {
	m, err := a.store.Load()
	if err != nil {
		return manifest.Track{}, err
	}
	t, ok := m.Get(id)
	if !ok {
		return manifest.Track{}, fmt.Errorf("no track with id %q", id)
	}
	m.Remove(id)
	return t, a.store.Save(m)
}
