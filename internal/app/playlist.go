package app

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"music-manager/internal/layout"
	"music-manager/internal/manifest"
)

// CreatePlaylist adds a playlist, with the given tracks in it if there are
// any (and all of them exist: otherwise there is no playlist). Its name must
// not be empty, nor come out as the same file name as another playlist's.
func (a *App) CreatePlaylist(name string, ids ...string) error {
	m, err := a.store.Load()
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if err := checkPlaylistName(m, name); err != nil {
		return err
	}
	m.Playlists = append(m.Playlists, manifest.Playlist{Name: name, Tracks: []string{}})
	if _, err := addTracks(m, len(m.Playlists)-1, ids); err != nil {
		return err
	}
	return a.store.Save(m)
}

// checkPlaylistName says whether name can be given to a new playlist.
func checkPlaylistName(m *manifest.Manifest, name string) error {
	if name == "" {
		return errors.New("a playlist needs a name")
	}
	file := strings.ToLower(layout.PlaylistPath(name))
	for _, p := range m.Playlists {
		if strings.ToLower(layout.PlaylistPath(p.Name)) == file {
			return fmt.Errorf("there is already a playlist called %q, and %q would be the same file (%s)", p.Name, name, layout.PlaylistPath(name))
		}
	}
	return nil
}

// playlist returns the index of the playlist with this name.
func playlist(m *manifest.Manifest, name string) (int, error) {
	for i, p := range m.Playlists {
		if p.Name == name {
			return i, nil
		}
	}
	return 0, fmt.Errorf("no playlist named %q", name)
}

// AddToPlaylist appends tracks to the end of a playlist, in the order given,
// and returns how many were added. A track already in the playlist is left
// where it is. If any track is unknown nothing is added.
func (a *App) AddToPlaylist(name string, ids []string) (added int, err error) {
	m, err := a.store.Load()
	if err != nil {
		return 0, err
	}
	i, err := playlist(m, name)
	if err != nil {
		return 0, err
	}
	if added, err = addTracks(m, i, ids); err != nil {
		return 0, err
	}
	return added, a.store.Save(m)
}

// addTracks appends the tracks that are not in playlist i yet, in order, and
// says how many that was. It fails, changing nothing, if a track is unknown.
func addTracks(m *manifest.Manifest, i int, ids []string) (added int, err error) {
	for _, id := range ids {
		if _, ok := m.Get(id); !ok {
			return 0, fmt.Errorf("no track with id %q", id)
		}
	}
	have := map[string]bool{}
	for _, id := range m.Playlists[i].Tracks {
		have[id] = true
	}
	for _, id := range ids {
		if !have[id] {
			have[id] = true
			m.Playlists[i].Tracks = append(m.Playlists[i].Tracks, id)
			added++
		}
	}
	return added, nil
}

// RemoveFromPlaylist takes tracks out of a playlist, leaving the rest in
// order. If any of them is not in the playlist, nothing is removed.
func (a *App) RemoveFromPlaylist(name string, ids []string) error {
	m, err := a.store.Load()
	if err != nil {
		return err
	}
	i, err := playlist(m, name)
	if err != nil {
		return err
	}
	drop := map[string]bool{}
	for _, id := range ids {
		if !slices.Contains(m.Playlists[i].Tracks, id) {
			return fmt.Errorf("track %q is not in the playlist %q", id, name)
		}
		drop[id] = true
	}
	m.Playlists[i].Tracks = slices.DeleteFunc(m.Playlists[i].Tracks, func(id string) bool { return drop[id] })
	return a.store.Save(m)
}

// MoveInPlaylist puts a track at position to (counting from 0) in a
// playlist, shifting the tracks in between.
func (a *App) MoveInPlaylist(name, id string, to int) error {
	m, err := a.store.Load()
	if err != nil {
		return err
	}
	i, err := playlist(m, name)
	if err != nil {
		return err
	}
	tracks := m.Playlists[i].Tracks
	from := slices.Index(tracks, id)
	switch {
	case from < 0:
		return fmt.Errorf("track %q is not in the playlist %q", id, name)
	case to < 0 || to >= len(tracks):
		return fmt.Errorf("position %d is outside the playlist %q, which has %d tracks", to, name, len(tracks))
	}
	tracks = slices.Delete(tracks, from, from+1)
	m.Playlists[i].Tracks = slices.Insert(tracks, to, id)
	return a.store.Save(m)
}

// RenamePlaylist gives a playlist another name, keeping its tracks and its
// place in the list of playlists.
func (a *App) RenamePlaylist(name, to string) error {
	m, err := a.store.Load()
	if err != nil {
		return err
	}
	i, err := playlist(m, name)
	if err != nil {
		return err
	}
	to = strings.TrimSpace(to)
	// Check the new name against the others only: it may differ from the old
	// one in nothing but case.
	others := &manifest.Manifest{Playlists: slices.Delete(slices.Clone(m.Playlists), i, i+1)}
	if err := checkPlaylistName(others, to); err != nil {
		return err
	}
	m.Playlists[i].Name = to
	return a.store.Save(m)
}

// DeletePlaylist removes a playlist. The tracks stay in the library.
func (a *App) DeletePlaylist(name string) error {
	m, err := a.store.Load()
	if err != nil {
		return err
	}
	i, err := playlist(m, name)
	if err != nil {
		return err
	}
	m.Playlists = slices.Delete(m.Playlists, i, i+1)
	return a.store.Save(m)
}

// playlistFile is the M3U file of a playlist, given where each track is
// rendered. The player (a Wolfang WMP01) was found to read paths from the
// root of its card with backslashes, one per line, in plain UTF-8 with
// Windows line endings; slashes, and paths relative to the playlist, give
// an empty playlist.
func playlistFile(p manifest.Playlist, paths map[string]string) []byte {
	var b strings.Builder
	for _, id := range p.Tracks {
		if path, ok := paths[id]; ok {
			b.WriteString(`\` + strings.ReplaceAll(path, "/", `\`) + "\r\n")
		}
	}
	return []byte(b.String())
}
