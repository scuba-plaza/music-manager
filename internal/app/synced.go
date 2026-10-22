package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"

	"music-manager/internal/layout"
	"music-manager/internal/manifest"
)

// renderInputs works out, for tracks, where each one is rendered and which
// track supplies each album folder's cover (the oldest one that has a cover).
func renderInputs(tracks []manifest.Track) (paths map[string]string, albumCovers map[string]manifest.Track) {
	paths = layout.Assign(tracks)
	albumCovers = map[string]manifest.Track{}
	for _, t := range tracks {
		if t.Status == manifest.Identified && t.Album != "" && t.CoverSHA256 != "" {
			dir := path.Dir(paths[t.ID])
			if cur, ok := albumCovers[dir]; !ok || older(t, cur) {
				albumCovers[dir] = t
			}
		}
	}
	return paths, albumCovers
}

// signatures describes, for every track, everything its rendered output
// depends on: its record, where it goes and the cover of its album folder.
// Each playlist has one too, under "playlist:" and its name: its name and
// where each of its tracks goes.
func signatures(m *manifest.Manifest) map[string]string {
	paths, albumCovers := renderInputs(m.Tracks)
	sigs := make(map[string]string, len(m.Tracks)+len(m.Playlists))
	for _, p := range m.Playlists {
		sum := sha256.Sum256(append([]byte(p.Name+"\x00"), playlistFile(p, paths)...))
		sigs[playlistKey+p.Name] = hex.EncodeToString(sum[:])
	}
	for _, t := range m.Tracks {
		p := paths[t.ID]
		folderCover := ""
		if t.Status == manifest.Identified && t.Album != "" {
			folderCover = albumCovers[path.Dir(p)].CoverSHA256
		}
		t.AddedAt, t.UpdatedAt = time.Time{}, time.Time{} // saving a record is not changing it
		raw, _ := json.Marshal(t)
		sum := sha256.Sum256(append(raw, "\x00"+p+"\x00"+folderCover...))
		sigs[t.ID] = hex.EncodeToString(sum[:])
	}
	return sigs
}

// playlistKey starts the keys of playlists in the record of what was synced;
// track IDs never contain a colon.
const playlistKey = "playlist:"

type syncedFile struct {
	Version int               `json:"version"`
	Tracks  map[string]string `json:"tracks"`
}

func (a *App) syncedPath() string { return filepath.Join(a.root, ".mm", "synced.json") }

func (a *App) writeSynced(sigs map[string]string) error {
	data, err := json.MarshalIndent(syncedFile{Version: 1, Tracks: sigs}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(a.syncedPath()), 0o755); err != nil {
		return err
	}
	return writeAtomic(a.syncedPath(), data)
}

// Unsynced returns how many tracks and playlists would be written, changed
// or removed by a sync into the library folder, judged from the records
// alone: it reads no audio. known is false until the library has been synced once (or
// RecordSynced was called), since there is nothing to compare with.
func (a *App) Unsynced() (n int, known bool, err error) {
	data, err := os.ReadFile(a.syncedPath())
	if errors.Is(err, fs.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	var saved syncedFile
	if err := json.Unmarshal(data, &saved); err != nil {
		return 0, false, err
	}
	m, err := a.store.Load()
	if err != nil {
		return 0, false, err
	}
	current := signatures(m)
	for id, sig := range current {
		if saved.Tracks[id] != sig {
			n++
		}
	}
	for id := range saved.Tracks {
		if _, ok := current[id]; !ok {
			n++
		}
	}
	return n, true, nil
}

// RecordSynced notes that the library folder matches the records as they are
// now. A sync does this by itself; call it only after checking, with a plan
// that has nothing to do, that the folder really is up to date.
func (a *App) RecordSynced() error {
	m, err := a.store.Load()
	if err != nil {
		return err
	}
	return a.writeSynced(signatures(m))
}

func sameDir(a, b string) bool {
	x, err1 := filepath.Abs(a)
	y, err2 := filepath.Abs(b)
	return err1 == nil && err2 == nil && filepath.Clean(x) == filepath.Clean(y)
}
