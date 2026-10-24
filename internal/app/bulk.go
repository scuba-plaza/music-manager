package app

import (
	"fmt"
	"sort"

	"music-manager/internal/manifest"
)

// Restore puts a track record back exactly as t has it, replacing the
// current record with the same ID or adding it again if it was removed. It is
// what undo is built on. It refuses if the track's stored audio or cover is
// gone, which happens once a sync has collected the files of a removed track.
func (a *App) Restore(t manifest.Track) error {
	return a.store.Update(func(m *manifest.Manifest) error {
		if !a.audio.Has(t.AudioSHA256) {
			return fmt.Errorf("the audio of %q is no longer stored (a sync collected it)", t.ID)
		}
		if t.CoverSHA256 != "" && !a.covers.Has(t.CoverSHA256) {
			return fmt.Errorf("the cover of %q is no longer stored (a sync collected it)", t.ID)
		}
		if !m.Update(t) {
			m.Tracks = append(m.Tracks, t)
		}
		return nil
	})
}

// EditMany applies several edits in one step: either every one is applied
// and saved together, or, if any is invalid, none is. It returns the updated
// tracks by ID.
func (a *App) EditMany(edits map[string]Edit) (map[string]manifest.Track, error) {
	ids := make([]string, 0, len(edits))
	for id := range edits {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var updated map[string]manifest.Track
	err := a.store.Update(func(m *manifest.Manifest) error {
		updated = make(map[string]manifest.Track, len(edits))
		for _, id := range ids {
			t, ok := m.Get(id)
			if !ok {
				return fmt.Errorf("no track with id %q", id)
			}
			if err := a.checkCover(edits[id]); err != nil {
				return fmt.Errorf("%s: %w", id, err)
			}
			t, err := applyEdit(t, edits[id])
			if err != nil {
				return fmt.Errorf("%s: %w", id, err)
			}
			updated[id] = t
		}
		for _, t := range updated {
			m.Update(t)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Renumber numbers the given tracks 1..N in the order given and records N as
// the number of tracks on their album.
func (a *App) Renumber(ids []string) (map[string]manifest.Track, error) {
	edits := make(map[string]Edit, len(ids))
	total := len(ids)
	for i, id := range ids {
		if _, dup := edits[id]; dup {
			return nil, fmt.Errorf("track %q is listed twice", id)
		}
		n := i + 1
		edits[id] = Edit{TrackNumber: &n, TrackTotal: &total}
	}
	return a.EditMany(edits)
}
