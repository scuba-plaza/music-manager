package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"music-manager/internal/layout"
	"music-manager/internal/manifest"
	"music-manager/internal/mp3"
	"music-manager/internal/render"
)

// SyncOptions selects where the tree is rendered and what happens to files
// the manifest does not account for.
type SyncOptions struct {
	Target string // directory to render into; the library root if empty
	Prune  bool   // delete stray files instead of only reporting them
}

// SyncPlan is the set of operations that turns a target directory into the
// tree the manifest describes.
type SyncPlan struct {
	Ops []render.Op
	// Collected is how many unused audio and cover blobs Apply deleted.
	Collected int

	a                     *App
	target                string
	build                 map[string]func() ([]byte, error)
	keepAudio, keepCovers map[string]bool
}

// Root is the library directory.
func (a *App) Root() string { return a.root }

// PlanSync works out what rendering the manifest into the target would
// change. It reads the whole tree but writes nothing.
func (a *App) PlanSync(opts SyncOptions) (*SyncPlan, error) {
	target := opts.Target
	if target == "" {
		target = a.root
	}
	m, err := a.store.Load()
	if err != nil {
		return nil, err
	}
	keepAudio, keepCovers := map[string]bool{}, map[string]bool{}
	for _, t := range m.Tracks {
		keepAudio[t.AudioSHA256] = true
		keepCovers[t.CoverSHA256] = true
	}
	paths := layout.Assign(m.Tracks)
	desired := render.Tree{}
	build := map[string]func() ([]byte, error){}
	albumCovers := map[string]manifest.Track{} // folder -> the oldest track with a cover
	for _, t := range m.Tracks {
		t := t
		p := paths[t.ID]
		build[p] = func() ([]byte, error) { return a.renderTrack(t) }
		data, err := build[p]()
		if err != nil {
			return nil, fmt.Errorf("track %s: %w", t.ID, err)
		}
		desired[p] = fingerprint(data)

		if t.Status == manifest.Identified && t.Album != "" && t.CoverSHA256 != "" {
			dir := path.Dir(p)
			if cur, ok := albumCovers[dir]; !ok || older(t, cur) {
				albumCovers[dir] = t
			}
		}
	}
	for dir, t := range albumCovers {
		data, err := os.ReadFile(a.covers.Path(t.CoverSHA256))
		if err != nil {
			return nil, fmt.Errorf("cover of track %s: %w", t.ID, err)
		}
		name := path.Join(dir, "cover.jpg")
		if http.DetectContentType(data) == "image/png" {
			name = path.Join(dir, "cover.png")
		}
		build[name] = func() ([]byte, error) { return data, nil }
		desired[name] = fingerprint(data)
	}
	existing, err := scanTree(target, desired)
	if err != nil {
		return nil, err
	}
	return &SyncPlan{
		Ops: render.Plan(desired, existing, opts.Prune), a: a, target: target, build: build,
		keepAudio: keepAudio, keepCovers: keepCovers,
	}, nil
}

// Apply carries out the plan: it writes created and updated files and
// removes the files the plan marks for removal, along with directories left
// empty. Strays are left alone. Finally it deletes audio and cover blobs that
// no record uses any more.
func (p *SyncPlan) Apply() error {
	for _, op := range p.Ops {
		if !filepath.IsLocal(filepath.FromSlash(op.Path)) {
			return fmt.Errorf("refusing to touch %q outside the target", op.Path)
		}
		dest := filepath.Join(p.target, filepath.FromSlash(op.Path))
		switch op.Kind {
		case render.Create, render.Update:
			data, err := p.build[op.Path]()
			if err != nil {
				return fmt.Errorf("%s: %w", op.Path, err)
			}
			if err := writeAtomic(dest, data); err != nil {
				return err
			}
		case render.Remove:
			if err := os.Remove(dest); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
	}
	if err := removeEmptyDirs(p.target); err != nil {
		return err
	}
	audio, err := p.a.audio.GC(p.keepAudio)
	if err != nil {
		return err
	}
	covers, err := p.a.covers.GC(p.keepCovers)
	p.Collected = len(audio) + len(covers)
	return err
}

func (a *App) renderTrack(t manifest.Track) ([]byte, error) {
	audio, err := os.ReadFile(a.audio.Path(t.AudioSHA256))
	if err != nil {
		return nil, err
	}
	if t.TrimStart > 0 || t.TrimEnd > 0 {
		if audio, err = mp3.Trim(audio, t.TrimStart, t.TrimEnd); err != nil {
			return nil, err
		}
	}
	ts := tagSet(t)
	if t.Status == manifest.Identified && t.CoverSHA256 != "" {
		if ts.Cover, err = os.ReadFile(a.covers.Path(t.CoverSHA256)); err != nil {
			return nil, err
		}
		ts.CoverMIME = http.DetectContentType(ts.Cover)
	}
	return mp3.Render(audio, ts)
}

// tagSet is the complete tag of a track's file. Identified tracks carry the
// ASCII fields in the standard frames and the originals in custom ones.
func tagSet(t manifest.Track) mp3.TagSet {
	extra := map[string]string{"MM_ID": t.ID}
	if t.Status != manifest.Identified {
		return mp3.TagSet{Extra: extra}
	}
	for k, v := range map[string]string{
		"ORIGINAL_TITLE":      t.OriginalTitle,
		"ORIGINAL_ARTIST":     t.OriginalArtist,
		"ORIGINAL_ALBUM":      t.OriginalAlbum,
		"SOURCE_URL":          t.SourceURL,
		"MUSICBRAINZ_TRACKID": t.MBRecordingID,
		"MUSICBRAINZ_ALBUMID": t.MBReleaseID,
	} {
		if v != "" {
			extra[k] = v
		}
	}
	return mp3.TagSet{
		Title: t.Title, Artist: t.Artist, Album: t.Album,
		Track: t.TrackNumber, Total: t.TrackTotal,
		Length: t.PlayingLength(),
		Extra:  extra,
	}
}

func fingerprint(data []byte) render.File {
	sum := sha256.Sum256(data)
	return render.File{Size: int64(len(data)), SHA: hex.EncodeToString(sum[:])}
}

var mediaExts = map[string]bool{".mp3": true, ".jpg": true, ".jpeg": true, ".png": true}

// scanTree fingerprints the media files under dir, ignoring the library's own
// .mm directory. Only files that are also wanted and have the wanted size
// are hashed; for the rest the size alone already shows they differ.
func scanTree(dir string, desired render.Tree) (render.Tree, error) {
	tree := render.Tree{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && path == dir {
				return filepath.SkipAll
			}
			return err
		}
		if d.IsDir() {
			if d.Name() == ".mm" && filepath.Dir(path) == dir {
				return filepath.SkipDir
			}
			return nil
		}
		if !mediaExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		f := render.File{Size: info.Size()}
		if want, ok := desired[rel]; ok && want.Size == f.Size {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			f = fingerprint(data)
		}
		tree[rel] = f
		return nil
	})
	return tree, err
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".mm-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// removeEmptyDirs deletes directories under root that hold nothing, deepest
// first. The root itself and .mm are kept.
func removeEmptyDirs(root string) error {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != root {
			if d.Name() == ".mm" && filepath.Dir(path) == root {
				return filepath.SkipDir
			}
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, d := range dirs {
		if entries, err := os.ReadDir(d); err == nil && len(entries) == 0 {
			if err := os.Remove(d); err != nil {
				return err
			}
		}
	}
	return nil
}
