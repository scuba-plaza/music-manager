// Package manifest holds the library's single source of truth: one record
// per track, persisted as library.json.
package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

const schemaVersion = 2

type Status string

const (
	Unidentified Status = "unidentified"
	Identified   Status = "identified"
)

type Track struct {
	ID          string `json:"id"`
	AudioSHA256 string `json:"audio_sha256"`
	SourceURL   string `json:"source_url"`
	Status      Status `json:"status"`
	// Title, Artist and Album are the true names, in whatever script they are
	// written. Any other form of them is worked out when it is needed.
	Title        string        `json:"title"`
	Artist       string        `json:"artist"`
	Album        string        `json:"album"`
	TrackNumber  int           `json:"track_number"`
	TrackTotal   int           `json:"track_total"`
	SourceLength time.Duration `json:"source_length_ns"`
	TrimStart    time.Duration `json:"trim_start_ns"`
	TrimEnd      time.Duration `json:"trim_end_ns"`
	Hint         Hint          `json:"hint,omitzero"`
	// Released is when the track first came out, as precisely as is known:
	// 1999, 1999-05 or 1999-05-17.
	Released string `json:"released,omitempty"`
	// Genre is free text: Folk, Neoclassical, Anthem, whatever the library calls it.
	Genre string `json:"genre,omitempty"`
	// The Latin forms are how the true name (and the genre) reads in Latin
	// letters, where the one worked out from it is not good enough. They are
	// empty unless set by hand, and say nothing about any one device.
	TitleLatin  string `json:"title_latin,omitempty"`
	ArtistLatin string `json:"artist_latin,omitempty"`
	AlbumLatin  string `json:"album_latin,omitempty"`
	GenreLatin  string `json:"genre_latin,omitempty"`
	// GainSteps makes the track louder (or, below zero, quieter) by this many
	// steps of 1.505 dB. FadeIn and FadeOut are stretches of the original
	// audio over which the sound rises from nothing or falls to nothing.
	GainSteps     int       `json:"gain_steps,omitempty"`
	FadeIn        Span      `json:"fade_in,omitzero"`
	FadeOut       Span      `json:"fade_out,omitzero"`
	CoverSHA256   string    `json:"cover_sha256"`
	MBRecordingID string    `json:"mb_recording_id"`
	MBReleaseID   string    `json:"mb_release_id"`
	AddedAt       time.Time `json:"added_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Span is a stretch of a track's original audio; the zero value is none.
type Span struct {
	Start time.Duration `json:"start_ns"`
	End   time.Duration `json:"end_ns"`
}

// Hint carries what an ingested file said about itself. It only seeds the
// search in identify and is cleared once the track is identified.
type Hint struct {
	Title    string `json:"title,omitempty"`
	Artist   string `json:"artist,omitempty"`
	Album    string `json:"album,omitempty"`
	Track    int    `json:"track,omitempty"`
	Total    int    `json:"of,omitempty"`
	Date     string `json:"date,omitempty"`
	Filename string `json:"filename,omitempty"`
	Genre    string `json:"genre,omitempty"`
}

// PlayingLength is the length of the track once its trims are applied.
func (t Track) PlayingLength() time.Duration {
	return max(0, t.SourceLength-t.TrimStart-t.TrimEnd)
}

// Playlist is a named list of tracks, by ID, in the order they are played.
type Playlist struct {
	Name string `json:"name"`
	// NameLatin is the name in Latin letters where it is set by hand.
	NameLatin string   `json:"name_latin,omitempty"`
	Tracks    []string `json:"tracks"`
}

type Manifest struct {
	Tracks    []Track    `json:"tracks"`
	Playlists []Playlist `json:"playlists,omitempty"`
}

// Store keeps the records in one file. Reading is free; every change to the
// records goes through Update, which makes it take its turn with the changes
// of other mm (see lock.go).
type Store struct{ path string }

func NewStore(path string) *Store { return &Store{path: path} }

type file struct {
	Version   int        `json:"version"`
	Tracks    []Track    `json:"tracks"`
	Playlists []Playlist `json:"playlists,omitempty"`
}

func (s *Store) Load() (*Manifest, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Manifest{}, nil
	}
	if err != nil {
		return nil, err
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", s.path, err)
	}
	if f.Version == 1 {
		return nil, fmt.Errorf("%s is a version 1 library, which this mm no longer reads; an older build of mm can still convert it", s.path)
	}
	if f.Version != schemaVersion {
		return nil, fmt.Errorf("%s: unsupported library version %d (want %d)", s.path, f.Version, schemaVersion)
	}
	return &Manifest{Tracks: f.Tracks, Playlists: f.Playlists}, nil
}

// save writes the manifest atomically: the data goes to a temp file that is
// renamed over the target, after the previous version is copied to .bak.
func (s *Store) save(m *Manifest) error {
	data, err := json.MarshalIndent(file{Version: schemaVersion, Tracks: m.Tracks, Playlists: m.Playlists}, "", "  ")
	if err != nil {
		return err
	}
	if prev, err := os.ReadFile(s.path); err == nil {
		if err := os.WriteFile(s.path+".bak", prev, 0o644); err != nil {
			return err
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), filepath.Base(s.path)+".tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil { // on disk before the rename, or a power cut leaves an empty library.json
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}
