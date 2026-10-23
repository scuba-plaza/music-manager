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
	"strings"
	"time"

	"music-manager/internal/device"
	"music-manager/internal/layout"
	"music-manager/internal/manifest"
)

// renderInputs works out, for tracks, where each one is rendered and which
// track supplies each cover file: the cover of an album folder comes from the
// oldest track that has a cover, and is named for the kind of picture it is;
// where the target wants it, a single has a cover file of its own, named like
// its track. Tracks not identified have no cover file. folders says, for each
// album folder, whose cover it holds.
func (a *App) renderInputs(tracks []manifest.Track, dev device.Target) (paths map[string]string, covers, folders map[string]manifest.Track) {
	paths = layout.Assign(tracks, dev.Layout())
	covers, folders = map[string]manifest.Track{}, map[string]manifest.Track{}
	for _, t := range tracks {
		if t.Status != manifest.Identified || t.CoverSHA256 == "" {
			continue
		}
		if t.Album != "" {
			dir := path.Dir(paths[t.ID])
			if cur, ok := folders[dir]; !ok || older(t, cur) {
				folders[dir] = t
			}
		} else if dev.SingleCovers() {
			covers[singleCoverPath(paths[t.ID])] = t
		}
	}
	for dir, t := range folders {
		covers[path.Join(dir, dev.CoverFile(a.coverKind(t.CoverSHA256)))] = t
	}
	return paths, covers, folders
}

// singleCoverPath is where the cover of a single goes: beside its file, under
// the same name.
func singleCoverPath(trackPath string) string {
	return strings.TrimSuffix(trackPath, path.Ext(trackPath)) + ".jpg"
}

// signatures describes, for every track, everything its rendered output
// depends on: its record, where it goes and the cover of its album folder.
// Each playlist has one too, under "playlist:" and its name: its name and
// where each of its tracks goes.
func (a *App) signatures(m *manifest.Manifest) map[string]string {
	paths, _, folders := a.renderInputs(m.Tracks, device.Generic)
	sigs := make(map[string]string, len(m.Tracks)+len(m.Playlists))
	for _, p := range m.Playlists {
		_, content := device.Generic.Playlist(p, byID(m.Tracks), paths)
		sum := sha256.Sum256(append([]byte(p.Name+"\x00"), content...))
		sigs[playlistKey+p.Name] = hex.EncodeToString(sum[:])
	}
	for _, t := range m.Tracks {
		p := paths[t.ID]
		folderCover := "" // the cover file this track's own belongs to, or comes from
		if t.Status == manifest.Identified && t.Album != "" {
			folderCover = folders[path.Dir(p)].CoverSHA256
		} else if t.Status == manifest.Identified && t.CoverSHA256 != "" && device.Generic.SingleCovers() {
			folderCover = "single" // it has a cover file beside it
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
	current := a.signatures(m)
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
	return a.writeSynced(a.signatures(m))
}

func sameDir(a, b string) bool {
	x, err1 := filepath.Abs(a)
	y, err2 := filepath.Abs(b)
	return err1 == nil && err2 == nil && filepath.Clean(x) == filepath.Clean(y)
}

// byID lists tracks by their ID.
func byID(tracks []manifest.Track) map[string]manifest.Track {
	out := make(map[string]manifest.Track, len(tracks))
	for _, t := range tracks {
		out[t.ID] = t
	}
	return out
}
