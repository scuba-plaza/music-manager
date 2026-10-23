package app

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	"music-manager/internal/coverimg"
	"music-manager/internal/device"
	"music-manager/internal/manifest"
)

// SetCover sets the cover image of a track, or removes it if image is empty.
// Only JPEG and PNG images are accepted, and what is kept is the file just as
// it came: what a device needs of it is made when it is written (see device).
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

// storeCover files an image in the cover store, as it is, and returns its SHA;
// empty input gives an empty SHA, and an image that is not a JPEG or a PNG is
// refused.
func (a *App) storeCover(image []byte) (string, error) {
	if len(image) == 0 {
		return "", nil
	}
	if _, err := coverimg.Validate(image); err != nil {
		return "", err
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

// coverFile is the stored cover with the given SHA as it is written to the
// target, and the kind of image it is.
func (a *App) coverFile(sha string, dev device.Target) ([]byte, string, error) {
	data, err := os.ReadFile(a.covers.Path(sha))
	if err != nil {
		return nil, "", err
	}
	return dev.Cover(data)
}

// coverKind says what kind of image the stored cover with this SHA is, from its
// first bytes: "image/jpeg" or "image/png", or "" if it cannot be read.
func (a *App) coverKind(sha string) string {
	f, err := os.Open(a.covers.Path(sha))
	if err != nil {
		return ""
	}
	defer f.Close()
	head := make([]byte, 8)
	n, _ := io.ReadFull(f, head)
	return coverimg.Sniff(head[:n])
}
