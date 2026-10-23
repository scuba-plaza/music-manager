package app

import (
	"fmt"
	"os"
	"time"

	"music-manager/internal/manifest"
)

// RestoreReport says what RestoreLatin did, or with dryRun would do, with the
// Latin forms that the backup of the version 1 library holds.
type RestoreReport struct {
	// Restored are the forms put back.
	Restored []manifest.HandSetName
	// Renamed are the forms left alone because the name they were set for has
	// been changed since, Kept those because the track has a form of its own for
	// that name now, and Gone those of tracks that were removed since.
	Renamed, Kept, Gone []manifest.HandSetName
}

// RestoreLatin puts back the Latin forms that were set by hand in the version
// 1 library kept by Migrate, for a library that was migrated before version 2
// kept them. A form is put back only where the track still has the name it was
// set for and has no form of its own for it. With dryRun nothing is written.
func (a *App) RestoreLatin(dryRun bool) (*RestoreReport, error) {
	if _, err := os.Stat(a.MigrationBackup()); err != nil {
		return nil, fmt.Errorf("there is no backup of an older library, %s: mm migrate makes it, so there is nothing to restore from", a.MigrationBackup())
	}
	held, err := manifest.HandSetNames(a.MigrationBackup())
	if err != nil {
		return nil, err
	}
	m, err := a.store.Load()
	if err != nil {
		return nil, err
	}
	rep := &RestoreReport{}
	now := time.Now().UTC()
	for _, h := range held {
		t, ok := m.Get(h.ID)
		if !ok {
			rep.Gone = append(rep.Gone, h)
			continue
		}
		var name, form *string
		switch h.Field {
		case "title":
			name, form = &t.Title, &t.TitleLatin
		case "artist":
			name, form = &t.Artist, &t.ArtistLatin
		case "album":
			name, form = &t.Album, &t.AlbumLatin
		default:
			return nil, fmt.Errorf("the backup has a Latin form for %q, which is not a name", h.Field)
		}
		switch {
		case *name != h.Name:
			rep.Renamed = append(rep.Renamed, h)
		case *form != "":
			rep.Kept = append(rep.Kept, h)
		default:
			*form = h.Held
			t.UpdatedAt = now
			m.Update(t)
			rep.Restored = append(rep.Restored, h)
		}
	}
	if dryRun || len(rep.Restored) == 0 {
		return rep, nil
	}
	return rep, a.store.Save(m)
}
