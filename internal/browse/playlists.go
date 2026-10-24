package browse

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

// editPlaylists runs a change to the playlists and records it as one step, with
// the playlists as they were and became.
func (e *Editor) editPlaylists(label string, call func() (status string, err error)) (Result, error) {
	before, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	status, err := call()
	if err != nil {
		return Result{}, err
	}
	after, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	if reflect.DeepEqual(before.Playlists, after.Playlists) {
		return Result{Message: status, Same: true}, nil // nothing changed, so nothing to undo
	}
	e.record(change{label: label, playlists: true, playlistsBefore: before.Playlists, playlistsAfter: after.Playlists})
	return Result{Message: status + " · u undoes it"}, nil
}

// PlaylistNew makes a playlist of the tracks.
func (e *Editor) PlaylistNew(name string, ids []string) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	name = strings.TrimSpace(name)
	return e.editPlaylists("new playlist "+name, func() (string, error) {
		return fmt.Sprintf("Made %s with %s", name, plural(len(ids), "track")), e.lib.CreatePlaylist(name, ids...)
	})
}

// PlaylistAdd adds the tracks to a playlist.
func (e *Editor) PlaylistAdd(name string, ids []string) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.editPlaylists("adding to "+name, func() (string, error) {
		added, err := e.lib.AddToPlaylist(name, ids)
		status := "Added " + plural(added, "track") + " to " + name
		if skipped := len(ids) - added; skipped > 0 {
			status += fmt.Sprintf(" (%d already there)", skipped)
		}
		return status, err
	})
}

// PlaylistTakeOut takes tracks out of a playlist; the library keeps them.
func (e *Editor) PlaylistTakeOut(name string, ids []string) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	status := "Took " + plural(len(ids), "track") + " out of " + name
	if len(ids) == 1 {
		if t, ok := s.Track(ids[0]); ok {
			_, title, _ := app.Display(t)
			status = fmt.Sprintf("Took “%s” out of %s", title, name)
		}
	}
	return e.editPlaylists("removal from "+name, func() (string, error) {
		return status, e.lib.RemoveFromPlaylist(name, ids)
	})
}

// PlaylistDelete deletes a playlist; its tracks stay in the library.
func (e *Editor) PlaylistDelete(name string) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.editPlaylists("deletion of the playlist "+name, func() (string, error) {
		return "Deleted the playlist " + name, e.lib.DeletePlaylist(name)
	})
}

// MoveInPlaylist moves a track one place up or down its playlist.
func (e *Editor) MoveInPlaylist(name, id string, dir int) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	i, ok := manifest.PlaylistNamed(s.Playlists, name)
	if !ok {
		return Result{}, errors.New("that playlist is not there any more")
	}
	p := s.Playlists[i]
	to := slices.Index(p.Tracks, id) + dir
	if to < 0 || to >= len(p.Tracks) {
		if dir > 0 {
			return Result{Message: "It is the last track already", Same: true}, nil
		}
		return Result{Message: "It is the first track already", Same: true}, nil
	}
	r, err := e.editPlaylists("move of a track in "+name, func() (string, error) {
		return "Moved", e.lib.MoveInPlaylist(name, id, to)
	})
	r.Reveal = id
	return r, err
}

// MoveInAlbum swaps the number of a track with its neighbour's, which moves it
// up or down its album.
func (e *Editor) MoveInAlbum(id string, dir int) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	t, err := track(s, id)
	if err != nil {
		return Result{}, err
	}
	if t.Status != manifest.Identified || t.Album == "" {
		return Result{}, errors.New("Only tracks of an album can be moved")
	}
	var album []manifest.Track
	for _, x := range s.Tracks {
		if x.Status == manifest.Identified && x.Artist == t.Artist && x.Album == t.Album {
			album = append(album, x)
		}
	}
	sortTracks(album)
	at := -1
	for i, x := range album {
		if x.ID == id {
			at = i
		}
	}
	i := at + dir
	if i < 0 || i >= len(album) {
		if dir > 0 {
			return Result{Message: "It is the last track already", Same: true}, nil
		}
		return Result{Message: "It is the first track already", Same: true}, nil
	}
	a, b := album[at], album[i]
	if a.TrackNumber == 0 || b.TrackNumber == 0 {
		return Result{}, errors.New("Number the album's tracks first: select the album and press n")
	}
	edits := map[string]app.Edit{a.ID: {TrackNumber: &b.TrackNumber}, b.ID: {TrackNumber: &a.TrackNumber}}
	r, err := e.saveMany(s, "move of a track", "Moved", []string{a.ID, b.ID}, func() (map[string]manifest.Track, error) { return e.lib.EditMany(edits) })
	r.Reveal = id
	return r, err
}

// Renumber numbers the tracks of the album with the given row key (or of the
// album a track row is in) 1..N in the order they are listed.
func (e *Editor) Renumber(albumKey string) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	g, ok := groupOf(s, albumKey)
	if !ok || g.kind != kindAlbum {
		return Result{}, errors.New("Select an album, or one of its tracks, to number it")
	}
	tracks := append([]manifest.Track(nil), g.tracks...)
	sortTracks(tracks)
	ids := idsOf(tracks)
	return e.saveMany(s, "numbering of an album", fmt.Sprintf("Numbered %s 1–%d", plural(len(ids), "track"), len(ids)), ids,
		func() (map[string]manifest.Track, error) { return e.lib.Renumber(ids) })
}
