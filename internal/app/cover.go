package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"music-manager/internal/coverimg"
	"music-manager/internal/device"
	"music-manager/internal/manifest"
)

// SetCover sets the cover image of a track, or removes it if image is empty.
// Only JPEG and PNG images are accepted, and what is kept is a small baseline
// JPEG, the kind the player opens (see coverimg).
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

// storeCover files an image in the cover store, made into a cover the player
// opens, and returns its SHA; empty input gives an empty SHA.
func (a *App) storeCover(image []byte) (string, error) {
	if len(image) == 0 {
		return "", nil
	}
	cover, err := coverimg.Normalize(image)
	if err != nil {
		return "", err
	}
	return a.covers.Put(bytes.NewReader(cover))
}

// older reports whether a was added before b, breaking ties by ID.
func older(a, b manifest.Track) bool {
	if !a.AddedAt.Equal(b.AddedAt) {
		return a.AddedAt.Before(b.AddedAt)
	}
	return a.ID < b.ID
}

// coverFile is the stored cover with the given SHA as it is written to the
// target, and the kind of image it is.
func (a *App) coverFile(sha string, dev device.Target) ([]byte, string, error) {
	data, err := os.ReadFile(a.covers.Path(sha))
	if err != nil {
		return nil, "", err
	}
	return dev.Cover(data)
}

// CoverReport says what NormalizeCovers did, or would do.
type CoverReport struct {
	Images    int   // the different covers the library uses
	Converted int   // of those, the ones not yet in the form the player opens
	Tracks    int   // tracks that use a different cover now
	Before    int64 // the converted images' size in bytes, before and after
	After     int64
}

// NormalizeCovers rewrites every cover of the library that the player may
// not open (a PNG, a progressive or oversized JPEG) into one it does, and
// points the tracks at the new image. The old images are deleted by the next
// sync, as nothing uses them any more. With dryRun nothing is changed.
func (a *App) NormalizeCovers(dryRun bool) (CoverReport, error) {
	var rep CoverReport
	m, err := a.store.Load()
	if err != nil {
		return rep, err
	}
	newSHA := map[string]string{} // each cover in use, and what it becomes
	for _, t := range m.Tracks {
		sha := t.CoverSHA256
		if _, done := newSHA[sha]; done || sha == "" {
			continue
		}
		rep.Images++
		data, err := os.ReadFile(a.covers.Path(sha))
		if err != nil {
			return rep, fmt.Errorf("cover of track %s: %w", t.ID, err)
		}
		out, err := coverimg.Normalize(data)
		if err != nil {
			return rep, fmt.Errorf("cover of track %s: %w", t.ID, err)
		}
		if bytes.Equal(out, data) {
			newSHA[sha] = sha
			continue
		}
		rep.Converted++
		rep.Before, rep.After = rep.Before+int64(len(data)), rep.After+int64(len(out))
		if dryRun {
			sum := sha256.Sum256(out)
			newSHA[sha] = hex.EncodeToString(sum[:])
		} else if newSHA[sha], err = a.covers.Put(bytes.NewReader(out)); err != nil {
			return rep, err
		}
	}
	for i, t := range m.Tracks {
		if next, ok := newSHA[t.CoverSHA256]; ok && next != t.CoverSHA256 {
			rep.Tracks++
			m.Tracks[i].CoverSHA256 = next // not an edit: the dates stay
		}
	}
	if dryRun || rep.Tracks == 0 {
		return rep, nil
	}
	if err := a.backupLibrary("before-covers"); err != nil {
		return rep, err
	}
	return rep, a.store.Save(m)
}

// backupLibrary copies library.json to .mm/library.json.<suffix>.
func (a *App) backupLibrary(suffix string) error {
	data, err := os.ReadFile(filepath.Join(a.root, "library.json"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(a.root, ".mm"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(a.root, ".mm", "library.json."+suffix), data, 0o644)
}
