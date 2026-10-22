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

// RemoveMany removes several tracks in one step: either all of them or, if
// any id is unknown or listed twice, none. It returns the removed tracks in
// the order given.
func (a *App) RemoveMany(ids []string) ([]manifest.Track, error) {
	m, err := a.store.Load()
	if err != nil {
		return nil, err
	}
	removed := make([]manifest.Track, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		t, ok := m.Get(id)
		if !ok {
			return nil, fmt.Errorf("no track with id %q", id)
		}
		if seen[id] {
			return nil, fmt.Errorf("track %q is listed twice", id)
		}
		seen[id] = true
		removed = append(removed, t)
	}
	for _, id := range ids {
		m.Remove(id)
	}
	return removed, a.store.Save(m)
}
