package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"music-manager/internal/device"
	"music-manager/internal/manifest"
	"music-manager/internal/mp3"
	"music-manager/internal/render"
)

// SyncOptions selects where the tree is rendered and what happens to files
// the manifest does not account for.
type SyncOptions struct {
	Target string // directory to render into; the library root if empty
	Prune  bool   // delete stray files instead of only reporting them
	// Device is the kind of place the tree is written to; nil means device.Generic.
	Device device.Target
	// OnProgress, if set, hears how many tracks have been prepared so far
	// while the plan is made.
	OnProgress func(done, total int)
}

// SyncPlan is the set of operations that turns a target directory into the
// tree the manifest describes.
type SyncPlan struct {
	Ops []render.Op
	// Collected is how many unused audio and cover blobs Apply deleted.
	Collected int
	// Warnings is what the device will not do well for these tracks.
	Warnings []device.Warning

	a                     *App
	target                string
	build                 map[string]func() ([]byte, error)
	keepAudio, keepCovers map[string]bool
	desired, existing     render.Tree
	sigs                  map[string]string
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
	dev := opts.Device
	if dev == nil {
		dev = device.Generic
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
	paths, covers, _ := a.renderInputs(m.Tracks, dev)
	desired := render.Tree{}
	build := map[string]func() ([]byte, error){}
	for i, t := range m.Tracks {
		t := t
		p := paths[t.ID]
		build[p] = func() ([]byte, error) { return a.renderTrack(t, dev) }
		data, err := build[p]()
		if err != nil {
			return nil, fmt.Errorf("track %s: %w", t.ID, err)
		}
		desired[p] = fingerprint(data)
		if opts.OnProgress != nil {
			opts.OnProgress(i+1, len(m.Tracks))
		}
	}
	for _, p := range m.Playlists {
		name, data := dev.Playlist(p, byID(m.Tracks), paths)
		build[name] = func() ([]byte, error) { return data, nil }
		desired[name] = fingerprint(data)
	}
	for name, t := range covers {
		data, _, err := a.coverFile(t.CoverSHA256, dev)
		if err != nil {
			return nil, fmt.Errorf("cover of track %s: %w", t.ID, err)
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
		desired: desired, existing: existing, sigs: a.signatures(m),
		Warnings: dev.Diagnose(m.Tracks),
	}, nil
}

// WithPrune returns the same plan with stray files marked for removal (or,
// with false, only reported), without scanning anything again.
func (p *SyncPlan) WithPrune(prune bool) *SyncPlan {
	q := *p
	q.Ops = render.Plan(p.desired, p.existing, prune)
	q.Collected = 0
	return &q
}

// Apply carries out the plan: it writes created and updated files and
// removes the files the plan marks for removal, along with directories left
// empty. Strays are left alone. Finally it deletes audio and cover blobs that
// no record uses any more.
func (p *SyncPlan) Apply() error { return p.ApplyWith(nil) }

// ApplyWith is Apply that reports how many of the plan's operations are done.
// After a sync into the library folder it also records what was synced, for
// Unsynced.
func (p *SyncPlan) ApplyWith(progress func(done, total int)) error {
	for i, op := range p.Ops {
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
		if progress != nil {
			progress(i+1, len(p.Ops))
		}
	}
	if err := removeEmptyDirs(p.target); err != nil {
		return err
	}
	keepAudio, keepCovers, err := p.keepNow()
	if err != nil {
		return err
	}
	audio, err := p.a.audio.GC(keepAudio)
	if err != nil {
		return err
	}
	covers, err := p.a.covers.GC(keepCovers)
	p.Collected = len(audio) + len(covers)
	if err != nil {
		return err
	}
	if sameDir(p.target, p.a.root) {
		return p.a.writeSynced(p.sigs)
	}
	return nil
}

// keepNow is the audio and the covers that must not be collected: those the
// records needed when the plan was made, and those they need now. A plan can
// wait for review, and a sync to a card takes minutes, in which another mm may
// have added a track whose files are not in the plan.
func (p *SyncPlan) keepNow() (audio, covers map[string]bool, err error) {
	m, err := p.a.store.Load()
	if err != nil {
		return nil, nil, err
	}
	audio, covers = maps.Clone(p.keepAudio), maps.Clone(p.keepCovers)
	for _, t := range m.Tracks {
		audio[t.AudioSHA256], covers[t.CoverSHA256] = true, true
	}
	return audio, covers, nil
}

func (a *App) renderTrack(t manifest.Track, dev device.Target) ([]byte, error) {
	audio, err := os.ReadFile(a.audio.Path(t.AudioSHA256))
	if err != nil {
		return nil, err
	}
	if audio, err = shaped(t, audio); err != nil {
		return nil, err
	}
	if t.TrimStart > 0 || t.TrimEnd > 0 {
		if audio, err = mp3.Trim(audio, t.TrimStart, t.TrimEnd); err != nil {
			return nil, err
		}
	}
	ts := dev.Tags(t)
	if t.Status == manifest.Identified && t.CoverSHA256 != "" {
		if ts.Cover, ts.CoverMIME, err = a.coverFile(t.CoverSHA256, dev); err != nil {
			return nil, err
		}
	}
	return mp3.Render(audio, ts)
}

func fingerprint(data []byte) render.File {
	sum := sha256.Sum256(data)
	return render.File{Size: int64(len(data)), SHA: hex.EncodeToString(sum[:])}
}

var mediaExts = map[string]bool{".mp3": true, ".jpg": true, ".jpeg": true, ".png": true, ".m3u": true, ".m3u8": true}

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
