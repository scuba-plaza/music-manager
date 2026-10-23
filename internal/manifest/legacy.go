package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"music-manager/internal/translit"
)

// Version 1 of library.json kept every name twice, in Latin letters (maybe set
// by hand) under title, artist and album, and as it really is under
// original_*, and kept the 12-character names of the Wolfgang WMP01 per track.
// Version 2 keeps the true names under title, artist and album and nothing for
// any one device. This file is the only place that knows the old shape.

// legacyNames are the fields of a version 1 track that version 2 has not got,
// and the old meaning of the keys it has.
type legacyNames struct {
	Latin struct {
		Title  string `json:"title"`
		Artist string `json:"artist"`
		Album  string `json:"album"`
	}
	OriginalTitle  string `json:"original_title"`
	OriginalArtist string `json:"original_artist"`
	OriginalAlbum  string `json:"original_album"`
	PlayerArtist   string `json:"player_artist"`
	PlayerAlbum    string `json:"player_album"`
}

// HandSetName is a Latin form that a version 1 library held for a name and
// that is not what is worked out from the name now.
type HandSetName struct {
	ID    string
	Field string // title, artist or album
	Name  string // the true name
	Held  string // the Latin form the version 1 library held
	Now   string // the Latin form that is worked out from the name
}

// MigrationReport says what migrating a library changes and drops.
type MigrationReport struct {
	Tracks  int
	HandSet []HandSetName
	// PlayerArtists and PlayerAlbums are how many tracks had a name of their
	// own for the player, which version 2 does not keep.
	PlayerArtists, PlayerAlbums int
}

// Migrate turns the version 1 library at path into version 2. The old file is
// first copied, byte for byte, to backup, which must not exist yet. With
// dryRun it only reports. A library of another version is refused.
func Migrate(path, backup string, dryRun bool) (*MigrationReport, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s: there is no library to migrate", path)
	}
	if err != nil {
		return nil, err
	}
	var old struct {
		Version   int               `json:"version"`
		Tracks    []json.RawMessage `json:"tracks"`
		Playlists []Playlist        `json:"playlists"`
	}
	if err := json.Unmarshal(data, &old); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if old.Version != 1 {
		if old.Version == schemaVersion {
			return nil, fmt.Errorf("%s is already version %d: there is nothing to migrate", path, old.Version)
		}
		return nil, fmt.Errorf("%s: unsupported library version %d", path, old.Version)
	}

	rep := &MigrationReport{Tracks: len(old.Tracks)}
	m := &Manifest{Playlists: old.Playlists}
	for _, raw := range old.Tracks {
		var t Track
		var legacy legacyNames
		if err := json.Unmarshal(raw, &t); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if err := json.Unmarshal(raw, &legacy.Latin); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if err := json.Unmarshal(raw, &legacy); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		for _, n := range []struct {
			field       string
			latin, orig string
			truth       *string
		}{
			{"title", legacy.Latin.Title, legacy.OriginalTitle, &t.Title},
			{"artist", legacy.Latin.Artist, legacy.OriginalArtist, &t.Artist},
			{"album", legacy.Latin.Album, legacy.OriginalAlbum, &t.Album},
		} {
			*n.truth = n.orig
			if n.orig == "" { // recorded before the originals were kept: the name is all there is
				*n.truth = n.latin
			}
			if now := translit.ToASCII(*n.truth); n.latin != "" && now != n.latin {
				rep.HandSet = append(rep.HandSet, HandSetName{ID: t.ID, Field: n.field, Name: *n.truth, Held: n.latin, Now: now})
			}
		}
		if legacy.PlayerArtist != "" {
			rep.PlayerArtists++
		}
		if legacy.PlayerAlbum != "" {
			rep.PlayerAlbums++
		}
		m.Tracks = append(m.Tracks, t)
	}
	if dryRun {
		return rep, nil
	}

	if _, err := os.Stat(backup); err == nil {
		return nil, fmt.Errorf("%s is already there: move it away first, it is not replaced", backup)
	}
	if err := os.MkdirAll(filepath.Dir(backup), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(backup, data, 0o644); err != nil {
		return nil, err
	}
	if err := NewStore(path).Save(m); err != nil {
		os.Remove(backup)
		return nil, err
	}
	return rep, nil
}
