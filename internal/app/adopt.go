package app

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"music-manager/internal/manifest"
)

// AdoptResult says what happened to one track.
type AdoptResult struct {
	TrackID string
	Adopted bool
	Reason  string // why the track was left alone
}

// Adopt identifies tracks by what the file said about itself when it was
// ingested: its old title, artist, album and track number. It is for tracks
// that no database knows. With no ids it takes every track that is not
// identified; a track without an old title and artist is left alone.
func (a *App) Adopt(ids []string) ([]AdoptResult, error) {
	m, err := a.store.Load()
	if err != nil {
		return nil, err
	}
	var todo []manifest.Track
	if len(ids) == 0 {
		for _, t := range m.Tracks {
			if t.Status != manifest.Identified {
				todo = append(todo, t)
			}
		}
	}
	for _, id := range ids {
		t, ok := m.Get(id)
		if !ok {
			return nil, fmt.Errorf("no track with id %q", id)
		}
		todo = append(todo, t)
	}

	var results []AdoptResult
	for _, t := range todo {
		res := AdoptResult{TrackID: t.ID}
		h := t.Hint
		switch {
		case t.Status == manifest.Identified:
			res.Reason = "already identified"
		case h.Title == "" || h.Artist == "":
			res.Reason = "the old tags have no title and artist"
		default:
			track := h.Track
			if track == 0 {
				if n := fileNameNumber.FindStringSubmatch(h.Filename); n != nil {
					track, _ = strconv.Atoi(n[1])
				}
			}
			updated, err := applyEdit(t, Edit{
				Title: &h.Title, Artist: &h.Artist, Album: &h.Album,
				TrackNumber: &track, TrackTotal: &h.Total, Released: &h.Date,
			})
			if err != nil {
				res.Reason = err.Error()
				break
			}
			updated.UpdatedAt = time.Now().UTC()
			m.Update(updated)
			res.Adopted = true
		}
		results = append(results, res)
	}
	return results, a.store.Save(m)
}

// fileNameNumber matches the track number a file name starts with, "07 Song".
var fileNameNumber = regexp.MustCompile(`^(\d{1,3})\b`)
