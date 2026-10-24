package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/device"
	"music-manager/internal/manifest"
	"music-manager/internal/render"
)

// Library is everything the interface needs from the music library. The real
// one is *app.App behind NewLibrary; tests use that too, or wrap it to make
// a call fail.
type Library interface {
	// Root is the library folder.
	Root() string
	// Tracks returns every record.
	Tracks() ([]manifest.Track, error)
	// Playlists returns every playlist, in the order they were made.
	Playlists() ([]manifest.Playlist, error)
	// Edit changes a track and returns it as saved.
	Edit(id string, e app.Edit) (manifest.Track, error)
	// Restore puts a record back exactly as it was; undo is built on it.
	Restore(t manifest.Track) error
	// PreviewTrim writes the audio around one of a track's cuts to a
	// temporary file; the caller removes it with cleanup.
	PreviewTrim(id string, edge app.Edge, window time.Duration) (path string, cleanup func(), err error)
	// PlanSync works out what writing the library folder (or target, if
	// set) would change, without changing anything.
	PlanSync(opts app.SyncOptions) (Plan, error)
	// Doctor checks the stores for missing or damaged files.
	Doctor() ([]app.Problem, error)
	// Stamp is a short text that changes whenever the library's records
	// change, however that happens: it is how changes made by another
	// program are noticed.
	Stamp() (string, error)
	// RecordSynced notes that the library folder matches the records.
	RecordSynced() error
	// Unsynced counts the tracks and playlists whose output would change in the library
	// folder; known is false until it has been synced once.
	Unsynced() (n int, known bool, err error)
	// CanAdd reports whether tracks can be downloaded; Fetch downloads one and
	// prepares it, Finish adds it to the library with the given details.
	CanAdd() bool
	Fetch(ctx context.Context, url string) (*app.Draft, error)
	Finish(ctx context.Context, d *app.Draft, e app.Edit) (manifest.Track, error)
	// Lookup reports whether a music database can be searched at all.
	Lookup() bool
	// Candidates searches the database for what a track could be (or for q,
	// if given); ApplyCandidate identifies the track as one of them.
	Candidates(ctx context.Context, id string, q *app.Query) ([]app.Candidate, error)
	ApplyCandidate(ctx context.Context, id string, c app.Candidate) (manifest.Track, error)
	// IdentifyAll takes the confident match for every track that needs work,
	// telling each as it goes.
	IdentifyAll(ctx context.Context, each func(app.IdentifyResult, manifest.Track)) ([]app.IdentifyResult, error)
	// Adopt identifies tracks by the tags their files had.
	Adopt(ids []string) ([]app.AdoptResult, error)
	// RemoveMany removes tracks, all or none; their audio stays stored until
	// a sync collects it.
	RemoveMany(ids []string) ([]manifest.Track, error)
	// EditMany applies several edits at once: all of them or none.
	EditMany(edits map[string]app.Edit) (map[string]manifest.Track, error)
	// Renumber numbers the tracks 1..N in the order given.
	Renumber(ids []string) (map[string]manifest.Track, error)
	// The playlist operations are those of the app; ReplacePlaylists puts
	// back a snapshot, which is what undo is built on.
	CreatePlaylist(name string, ids ...string) error
	AddToPlaylist(name string, ids []string) (added int, err error)
	RemoveFromPlaylist(name string, ids []string) error
	MoveInPlaylist(name, id string, to int) error
	RenamePlaylist(name, to string) error
	SetPlaylistLatin(name, latin string) error
	DeletePlaylist(name string) error
	ReplacePlaylists(ps []manifest.Playlist) error
	// SetCover sets a track's cover image (JPEG or PNG), or removes it if
	// image is empty, and returns the track as saved.
	SetCover(id string, image []byte) (manifest.Track, error)
	// SetCoverMany does the same for several tracks at once, all or none,
	// and returns them as saved.
	SetCoverMany(ids []string, image []byte) (map[string]manifest.Track, error)
	// CoverImage returns the stored cover image with the given SHA.
	CoverImage(sha string) ([]byte, error)
}

type appLibrary struct {
	*app.App
	source     app.MetadataSource
	downloader app.Downloader
}

// WithDownloader lets the library add tracks by downloading them.
func WithDownloader(d app.Downloader) LibraryOption { return func(l *appLibrary) { l.downloader = d } }

// LibraryOption adds a capability to the library built by NewLibrary.
type LibraryOption func(*appLibrary)

// WithSource lets the library look tracks up in a music database.
func WithSource(s app.MetadataSource) LibraryOption { return func(l *appLibrary) { l.source = s } }

// NewLibrary returns the Library backed by a.
func NewLibrary(a *app.App, opts ...LibraryOption) Library {
	l := appLibrary{App: a}
	for _, o := range opts {
		o(&l)
	}
	return l
}

func (l appLibrary) Lookup() bool { return l.source != nil }

func (l appLibrary) Stamp() (string, error) {
	info, err := os.Stat(filepath.Join(l.Root(), "library.json"))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d", info.ModTime().UnixNano(), info.Size()), nil
}

func (l appLibrary) PlanSync(opts app.SyncOptions) (Plan, error) {
	p, err := l.App.PlanSync(opts)
	if err != nil {
		return nil, err
	}
	return appPlan{p}, nil
}

func (l appLibrary) CanAdd() bool { return l.downloader != nil }

func (l appLibrary) Fetch(ctx context.Context, url string) (*app.Draft, error) {
	if l.downloader == nil {
		return nil, errors.New("downloading is not available")
	}
	return l.App.Fetch(ctx, app.AddRequest{URL: url, Source: l.source}, l.downloader)
}

func (l appLibrary) Finish(ctx context.Context, d *app.Draft, e app.Edit) (manifest.Track, error) {
	return l.App.Finish(ctx, d, e)
}

func (l appLibrary) Candidates(ctx context.Context, id string, q *app.Query) ([]app.Candidate, error) {
	if l.source == nil {
		return nil, errNoLookup
	}
	return l.App.Candidates(ctx, id, l.source, q)
}

func (l appLibrary) ApplyCandidate(ctx context.Context, id string, c app.Candidate) (manifest.Track, error) {
	if l.source == nil {
		return manifest.Track{}, errNoLookup
	}
	return l.App.ApplyCandidate(ctx, id, c, l.source)
}

func (l appLibrary) IdentifyAll(ctx context.Context, each func(app.IdentifyResult, manifest.Track)) ([]app.IdentifyResult, error) {
	if l.source == nil {
		return nil, errNoLookup
	}
	return l.App.Identify(ctx, nil, l.source, nil, app.IdentifyOptions{Yes: true, OnResult: func(r app.IdentifyResult) {
		var t manifest.Track
		if r.Applied != nil { // what was saved, for the screen to show
			if m, err := l.Manifest(); err == nil {
				t, _ = m.Get(r.TrackID)
			}
		}
		each(r, t)
	}})
}

func (l appLibrary) SetCover(id string, image []byte) (manifest.Track, error) {
	if err := l.App.SetCover(id, image); err != nil {
		return manifest.Track{}, err
	}
	m, err := l.Manifest()
	if err != nil {
		return manifest.Track{}, err
	}
	t, _ := m.Get(id)
	return t, nil
}

func (l appLibrary) CoverImage(sha string) ([]byte, error) { return os.ReadFile(l.CoverPath(sha)) }

func (l appLibrary) Tracks() ([]manifest.Track, error) {
	m, err := l.Manifest()
	if err != nil {
		return nil, err
	}
	return m.Tracks, nil
}

func (l appLibrary) Playlists() ([]manifest.Playlist, error) {
	m, err := l.Manifest()
	if err != nil {
		return nil, err
	}
	return m.Playlists, nil
}

var errNoLookup = errors.New("lookup is not available")

// Plan is what a sync would do, ready to be carried out.
type Plan interface {
	Ops() []render.Op
	// Warnings is what the device the plan is for will not do well.
	Warnings() []device.Warning
	// WithPrune is the same plan with stray files marked for deletion or not.
	WithPrune(prune bool) Plan
	// Apply carries the plan out, reporting how far it has got.
	Apply(progress func(done, total int)) error
	// Collected is how many unused audio and cover files Apply deleted.
	Collected() int
}

type appPlan struct{ p *app.SyncPlan }

func (a appPlan) Ops() []render.Op                           { return a.p.Ops }
func (a appPlan) Warnings() []device.Warning                 { return a.p.Warnings }
func (a appPlan) WithPrune(prune bool) Plan                  { return appPlan{a.p.WithPrune(prune)} }
func (a appPlan) Collected() int                             { return a.p.Collected }
func (a appPlan) Apply(progress func(done, total int)) error { return a.p.ApplyWith(progress) }
