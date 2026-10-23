package manifest

import (
	"crypto/rand"
	"encoding/base32"
	"slices"
	"strings"
)

var idEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

func (m *Manifest) newID() string {
	for {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			panic(err)
		}
		id := idEncoding.EncodeToString(b)[:6]
		if _, taken := m.Get(id); !taken {
			return id
		}
	}
}

// Add stores t under a fresh ID and returns the stored record.
func (m *Manifest) Add(t Track) Track {
	t.ID = m.newID()
	m.Tracks = append(m.Tracks, t)
	return t
}

func (m *Manifest) Get(id string) (Track, bool) {
	for _, t := range m.Tracks {
		if t.ID == id {
			return t, true
		}
	}
	return Track{}, false
}

// Update replaces the record with t's ID, reporting whether it exists.
func (m *Manifest) Update(t Track) bool {
	for i := range m.Tracks {
		if m.Tracks[i].ID == t.ID {
			m.Tracks[i] = t
			return true
		}
	}
	return false
}

// Remove deletes the track with this ID, and takes it out of every playlist.
func (m *Manifest) Remove(id string) bool {
	for i, t := range m.Tracks {
		if t.ID == id {
			m.Tracks = append(m.Tracks[:i], m.Tracks[i+1:]...)
			for j := range m.Playlists {
				m.Playlists[j].Tracks = slices.DeleteFunc(m.Playlists[j].Tracks, func(s string) bool { return s == id })
			}
			return true
		}
	}
	return false
}

// Search returns the tracks matching every word of query. A word matches if
// it appears (case-insensitively) in any ASCII, original, genre or hint text
// field; a "status:<s>" word filters by status instead, and a "genre:<g>"
// word by genre alone. An empty query matches all.
func (m *Manifest) Search(query string) []Track {
	var out []Track
	for _, t := range m.Tracks {
		if matches(t, strings.Fields(strings.ToLower(query))) {
			out = append(out, t)
		}
	}
	return out
}

func matches(t Track, words []string) bool {
	hay := strings.ToLower(strings.Join([]string{
		t.AsciiTitle, t.OriginalTitle, t.AsciiArtist, t.OriginalArtist, t.AsciiAlbum, t.OriginalAlbum,
		t.Genre, t.Hint.Title, t.Hint.Artist, t.Hint.Album, t.Hint.Filename,
	}, "\n"))
	for _, w := range words {
		if s, ok := strings.CutPrefix(w, "status:"); ok {
			if string(t.Status) != s {
				return false
			}
		} else if g, ok := strings.CutPrefix(w, "genre:"); ok {
			if !strings.Contains(strings.ToLower(t.Genre), g) {
				return false
			}
		} else if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}

// FindByAudio returns the track that uses the audio with this SHA.
func (m *Manifest) FindByAudio(sha string) (Track, bool) {
	for _, t := range m.Tracks {
		if t.AudioSHA256 == sha {
			return t, true
		}
	}
	return Track{}, false
}
