package browse

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/mp3"
)

// Store is what the editor changes; *app.App is one.
type Store interface {
	Manifest() (*manifest.Manifest, error)
	Candidates(ctx context.Context, id string, src app.MetadataSource, q *app.Query) ([]app.Candidate, error)
	ApplyCandidate(ctx context.Context, id string, c app.Candidate, src app.MetadataSource) (manifest.Track, error)
	Identify(ctx context.Context, ids []string, src app.MetadataSource, opts app.IdentifyOptions) ([]app.IdentifyResult, error)
	Adopt(ids []string) ([]app.AdoptResult, error)
	Fetch(ctx context.Context, req app.AddRequest, dl app.Downloader) (*app.Draft, error)
	Finish(ctx context.Context, d *app.Draft, e app.Edit) (manifest.Track, error)
	Edit(id string, e app.Edit) (manifest.Track, error)
	EditMany(edits map[string]app.Edit) (map[string]manifest.Track, error)
	Renumber(ids []string) (map[string]manifest.Track, error)
	Restore(t manifest.Track) error
	RemoveMany(ids []string) ([]manifest.Track, error)
	SetCover(id string, image []byte) error
	SetCoverMany(ids []string, image []byte) (map[string]manifest.Track, error)
	CreatePlaylist(name string, ids ...string) error
	AddToPlaylist(name string, ids []string) (added int, err error)
	RemoveFromPlaylist(name string, ids []string) error
	MoveInPlaylist(name, id string, to int) error
	RenamePlaylist(name, to string) error
	SetPlaylistLatin(name, latin string) error
	DeletePlaylist(name string) error
	ReplacePlaylists(ps []manifest.Playlist) error
}

// Result says how a change turned out, for the screen to show.
type Result struct {
	// Message is what to tell the person; empty when nothing was changed.
	Message string `json:"message,omitempty"`
	// Reveal is a track to put the cursor on, which may have moved.
	Reveal string `json:"reveal,omitempty"`
	// Moves maps the keys of rows to the keys they have after a rename, so that
	// the rows that were open stay open and selected.
	Moves map[string]string `json:"moves,omitempty"`
	// Same says there was nothing to save: the text was left as it was.
	Same bool `json:"same,omitempty"`
}

// change is one step of what can be undone.
type change struct {
	label         string
	before, after []manifest.Track

	// A change to the playlists keeps the playlists as they were and became.
	playlists                       bool
	playlistsBefore, playlistsAfter []manifest.Playlist

	// moves are the rows of the tree that the change renamed, from the old key to the new.
	moves map[string]string
}

// maxHistory is how many changes can be undone.
const maxHistory = 200

// History says what undo and redo would do now, and what is copied.
type History struct {
	Undo string `json:"undo,omitempty"`
	Redo string `json:"redo,omitempty"`
	Clip string `json:"clip,omitempty"`
}

// Editor changes the library and remembers what it did, so that it can be
// undone. One editor serves the one person using the program.
type Editor struct {
	lib        Store
	source     app.MetadataSource // the music database to look tracks up in; nil if there is none
	downloader app.Downloader     // what downloads a link; nil if there is none

	mu       sync.Mutex
	undoes   []change
	redoes   []change
	nudgeKey string
	clip     *clipContents

	searches    map[string]search // what the searches found, by name
	searchOrder []string
	drafts      map[string]*app.Draft // downloads waiting for their details, by name
	draftOrder  []string
}

// EditorOption adds a capability to an editor.
type EditorOption func(*Editor)

// WithSource lets the editor look tracks up in a music database.
func WithSource(s app.MetadataSource) EditorOption { return func(e *Editor) { e.source = s } }

// WithDownloader lets the editor add tracks by downloading them.
func WithDownloader(d app.Downloader) EditorOption { return func(e *Editor) { e.downloader = d } }

// NewEditor returns the editor of a library.
func NewEditor(lib Store, opts ...EditorOption) *Editor {
	e := &Editor{lib: lib}
	for _, o := range opts {
		o(e)
	}
	return e
}

func (e *Editor) snapshot() (Snapshot, error) {
	m, err := e.lib.Manifest()
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Tracks: m.Tracks, Playlists: m.Playlists}, nil
}

// History is what undo, redo and paste would do now.
func (e *Editor) History() History {
	e.mu.Lock()
	defer e.mu.Unlock()
	var h History
	if n := len(e.undoes); n > 0 {
		h.Undo = e.undoes[n-1].label
	}
	if n := len(e.redoes); n > 0 {
		h.Redo = e.redoes[n-1].label
	}
	if e.clip != nil {
		h.Clip = e.clip.names()
	}
	return h
}

// Forget empties the history: after a sync that deleted files, what was
// removed cannot be brought back.
func (e *Editor) Forget() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.undoes, e.redoes, e.nudgeKey = nil, nil, ""
}

// record remembers a change so it can be undone, and forgets what could
// have been redone.
func (e *Editor) record(c change) {
	e.undoes = append(e.undoes, c)
	if len(e.undoes) > maxHistory {
		e.undoes = e.undoes[1:]
	}
	e.redoes = nil
	e.nudgeKey = ""
}

func track(s Snapshot, id string) (manifest.Track, error) {
	t, ok := s.Track(id)
	if !ok {
		return t, fmt.Errorf("no track with id %q", id)
	}
	return t, nil
}

// fieldByKey finds a field by the name it goes by outside this package.
func fieldByKey(key string) (fieldKind, bool) {
	for f := fTitle; f <= fPlaylistLatin; f++ {
		if f.key() == key {
			return f, true
		}
	}
	return 0, false
}

// SetField saves what was typed in a field of a track. second is the number
// of tracks, for the field of the track number.
func (e *Editor) SetField(id, field, text, second string) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	f, ok := fieldByKey(field)
	if !ok || f == fWolfgang || f == fCover || f > fLatinGenre {
		return Result{}, fmt.Errorf("there is no field %q to edit", field)
	}
	s, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	t, err := track(s, id)
	if err != nil {
		return Result{}, err
	}
	if f.isLatin() && trueName(t, f) == "" {
		return Result{}, errors.New("There is no " + strings.ToLower(f.label()) + " to write in Latin letters")
	}
	d := draft{field: f, text: text, second: second, track: t}
	edit, err := d.edit()
	if err != nil {
		return Result{}, err
	}
	if orig := newDraft(t, f); !prefilledFrom(t, f) && text == orig.text && second == orig.second {
		return Result{Same: true}, nil // nothing changed, nothing to save
	}
	after, err := e.lib.Edit(id, edit)
	if err != nil {
		return Result{}, err
	}
	e.record(change{label: "change of " + f.what(), before: []manifest.Track{t}, after: []manifest.Track{after}})
	return Result{Message: "Saved", Reveal: id}, nil
}

// PathPreview is where the file of a track would go were the text typed in a
// field saved.
func PathPreview(s Snapshot, id, field, text, second string) (string, bool) {
	t, ok := s.Track(id)
	f, fok := fieldByKey(field)
	if !ok || !fok {
		return "", false
	}
	return pathOf(previewTrack(t, draft{field: f, text: text, second: second})), true
}

// minKept is the least audio a cut may leave of a track.
const minKept = 100 * time.Millisecond

func clamp(v, lo, hi int) int { return max(lo, min(hi, v)) }

// save is the one place a change to a single track is made and recorded.
// nudge, if set, joins this change to the run of nudges before it.
func (e *Editor) save(t manifest.Track, edit app.Edit, label, nudge string) (Result, error) {
	after, err := e.lib.Edit(t.ID, edit)
	if err != nil {
		return Result{}, err
	}
	if top := len(e.undoes) - 1; nudge != "" && nudge == e.nudgeKey && top >= 0 {
		e.undoes[top].after = []manifest.Track{after} // one more step of the same run
		e.redoes = nil
	} else {
		e.record(change{label: label, before: []manifest.Track{t}, after: []manifest.Track{after}})
	}
	e.nudgeKey = nudge
	return Result{Message: "Saved", Reveal: t.ID}, nil
}

// Nudge moves the cut, the volume or the fade of a track a step, within what
// the track allows: more is the direction (1 or -1), big a larger step.
func (e *Editor) Nudge(id, field string, more int, big bool) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	f, ok := fieldByKey(field)
	if !ok {
		return Result{}, fmt.Errorf("there is no field %q", field)
	}
	s, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	t, err := track(s, id)
	if err != nil {
		return Result{}, err
	}
	step := 100 * time.Millisecond
	if big {
		step = time.Second
	}
	if more < 0 {
		step = -step
	}
	switch f {
	case fVolume:
		return e.nudgeVolume(t, step)
	case fFadeIn, fFadeOut:
		return e.nudgeFade(t, f, step)
	case fCutStart, fCutEnd:
	default:
		return Result{}, nil
	}
	cur, other := t.TrimStart, t.TrimEnd
	if f == fCutEnd {
		cur, other = other, cur
	}
	room := max(t.SourceLength-other-minKept, 0).Truncate(time.Millisecond) // whole milliseconds, so rounding cannot eat into minKept
	d := time.Duration(clamp(int(cur+step), 0, int(room))).Round(time.Millisecond)
	if d == cur {
		return Result{}, nil
	}
	var edit app.Edit
	if f == fCutStart {
		edit.TrimStart = &d
	} else {
		edit.TrimEnd = &d
	}
	return e.save(t, edit, "change of "+strings.ToLower(f.label()), t.ID+"\x00"+f.label())
}

// Clear takes away the cut, the volume change or the fade of a track.
func (e *Editor) Clear(id, field string) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	f, ok := fieldByKey(field)
	if !ok {
		return Result{}, fmt.Errorf("there is no field %q", field)
	}
	s, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	t, err := track(s, id)
	if err != nil {
		return Result{}, err
	}
	var edit app.Edit
	switch f {
	case fCutStart:
		edit.TrimStart = new(time.Duration)
	case fCutEnd:
		edit.TrimEnd = new(time.Duration)
	case fVolume:
		edit.GainDB = new(float64)
	case fFadeIn:
		edit.FadeIn = &manifest.Span{}
	case fFadeOut:
		edit.FadeOut = &manifest.Span{}
	default:
		return Result{}, nil
	}
	return e.save(t, edit, "change of "+strings.ToLower(f.label()), "")
}

// nudgeVolume changes the volume by a step of 1.5 dB, or four with a big
// nudge, up to the most a track may be changed.
func (e *Editor) nudgeVolume(t manifest.Track, step time.Duration) (Result, error) {
	n := 1
	if step >= time.Second || step <= -time.Second {
		n = 4
	}
	if step < 0 {
		n = -n
	}
	next := clamp(t.GainSteps+n, -app.MaxGainSteps, app.MaxGainSteps)
	if next == t.GainSteps {
		return Result{}, nil
	}
	db := float64(next) * mp3.StepDB
	return e.save(t, app.Edit{GainDB: &db}, "change of volume", t.ID+"\x00"+fVolume.label())
}

// nudgeFade makes a fade longer or shorter by step, from where the track
// starts for a fade-in and back from where it ends for a fade-out, or from
// the fade's own edge if it has one. A fade that shrinks to nothing is gone.
func (e *Editor) nudgeFade(t manifest.Track, f fieldKind, step time.Duration) (Result, error) {
	span := t.FadeIn
	if f == fFadeOut {
		span = t.FadeOut
	}
	length := span.End - span.Start
	next := clamp(int(length+step), 0, int(max(t.PlayingLength(), 0)))
	d := time.Duration(next).Round(time.Millisecond)
	if d == length {
		return Result{}, nil
	}
	var edit app.Edit
	switch {
	case d == 0 && f == fFadeIn:
		edit.FadeIn = &manifest.Span{}
	case d == 0:
		edit.FadeOut = &manifest.Span{}
	case f == fFadeIn:
		start := t.TrimStart
		if length > 0 {
			start = span.Start
		}
		edit.FadeIn = &manifest.Span{Start: start, End: start + d}
	default:
		end := t.SourceLength - t.TrimEnd
		if length > 0 {
			end = span.End
		}
		edit.FadeOut = &manifest.Span{Start: end - d, End: end}
	}
	return e.save(t, edit, "change of "+strings.ToLower(f.label()), t.ID+"\x00"+f.label())
}

// saveMany runs a change to the tracks with the given ids and records it as
// one step.
func (e *Editor) saveMany(s Snapshot, label, status string, ids []string, call func() (map[string]manifest.Track, error)) (Result, error) {
	got, err := call()
	if err != nil {
		return Result{}, err
	}
	var before, after []manifest.Track
	for _, id := range ids {
		if t, ok := got[id]; ok {
			b, _ := s.Track(id)
			before, after = append(before, b), append(after, t)
		}
	}
	e.record(change{label: label, before: before, after: after})
	r := Result{Message: status}
	if len(after) > 0 {
		r.Reveal = after[0].ID
	}
	return r, nil
}

// SetCover gives the tracks the image (a JPEG or PNG), or takes their covers
// away if there is none.
func (e *Editor) SetCover(ids []string, image []byte) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	return e.setCover(s, ids, image, "cover")
}

// setCover is SetCover for ids that are known; noun says whose cover it is.
func (e *Editor) setCover(s Snapshot, ids []string, image []byte, noun string) (Result, error) {
	var todo []string
	for _, id := range ids {
		t, err := track(s, id)
		if err != nil {
			return Result{}, err
		}
		if len(image) > 0 || t.CoverSHA256 != "" {
			todo = append(todo, id)
		}
	}
	if len(todo) == 0 {
		return Result{Same: true}, nil // no cover to remove
	}
	label := "change of " + noun
	if len(image) == 0 {
		label = "removal of the " + noun
	}
	if len(todo) == 1 {
		t, _ := s.Track(todo[0])
		if err := e.lib.SetCover(t.ID, image); err != nil {
			return Result{}, err
		}
		m, err := e.lib.Manifest()
		if err != nil {
			return Result{}, err
		}
		after, _ := m.Get(t.ID)
		e.record(change{label: label, before: []manifest.Track{t}, after: []manifest.Track{after}})
		return Result{Message: "Saved", Reveal: t.ID}, nil
	}
	return e.saveMany(s, label, "Changed "+plural(len(todo), "track"), todo, func() (map[string]manifest.Track, error) {
		return e.lib.SetCoverMany(todo, image)
	})
}

// Remove takes tracks out of the library (their audio stays stored until a
// sync collects it); undo brings them back, in their playlists.
func (e *Editor) Remove(ids []string) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	before, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	got, err := e.lib.RemoveMany(ids)
	if err != nil {
		return Result{}, err
	}
	c := change{label: "removal of " + plural(len(got), "track"), before: got}
	after, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	if !reflect.DeepEqual(before.Playlists, after.Playlists) { // the tracks were in playlists, which undo must put them back in
		c.playlists, c.playlistsBefore, c.playlistsAfter = true, before.Playlists, after.Playlists
	}
	e.record(c)
	return Result{Message: "Removed " + plural(len(got), "track") + " · u undoes it"}, nil
}

// idsMissingFrom lists the ids of the tracks in a that are not in b.
func idsMissingFrom(a, b []manifest.Track) []string {
	have := make(map[string]bool, len(b))
	for _, t := range b {
		have[t.ID] = true
	}
	var ids []string
	for _, t := range a {
		if !have[t.ID] {
			ids = append(ids, t.ID)
		}
	}
	return ids
}

// target is what undoing or redoing a change puts back, and gone the ids of
// the tracks that must vanish for that (those the change added, when
// undoing; those it removed, when redoing).
func (c change) target(undone bool) (tracks []manifest.Track, gone []string) {
	if undone {
		return c.before, idsMissingFrom(c.after, c.before)
	}
	return c.after, idsMissingFrom(c.before, c.after)
}

func (c change) playlistTarget(undone bool) []manifest.Playlist {
	if undone {
		return c.playlistsBefore
	}
	return c.playlistsAfter
}

// Undo takes back the last change.
func (e *Editor) Undo() (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.undoes) == 0 {
		return Result{Message: "Nothing to undo"}, nil
	}
	c := e.undoes[len(e.undoes)-1]
	r, err := e.restore(c, true)
	if err != nil {
		return Result{}, err
	}
	e.undoes = e.undoes[:len(e.undoes)-1]
	e.redoes = append(e.redoes, c)
	e.nudgeKey = ""
	return r, nil
}

// Redo does again what Undo took back.
func (e *Editor) Redo() (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.redoes) == 0 {
		return Result{Message: "Nothing to redo"}, nil
	}
	c := e.redoes[len(e.redoes)-1]
	r, err := e.restore(c, false)
	if err != nil {
		return Result{}, err
	}
	e.redoes = e.redoes[:len(e.redoes)-1]
	e.undoes = append(e.undoes, c)
	e.nudgeKey = ""
	return r, nil
}

func (e *Editor) restore(c change, undone bool) (Result, error) {
	tracks, gone := c.target(undone)
	for _, t := range tracks {
		if err := e.lib.Restore(t); err != nil {
			return Result{}, err
		}
	}
	if len(gone) > 0 {
		if _, err := e.lib.RemoveMany(gone); err != nil {
			return Result{}, err
		}
	}
	if c.playlists {
		if err := e.lib.ReplacePlaylists(c.playlistTarget(undone)); err != nil {
			return Result{}, err
		}
	}
	verb := "Redid"
	if undone {
		verb = "Undid"
	}
	r := Result{Message: verb + ": " + c.label}
	r.Moves = c.moves
	if undone {
		r.Moves = invert(c.moves)
	}
	if len(tracks) > 0 {
		r.Reveal = tracks[0].ID
	}
	return r, nil
}

// previewWindow is how much of the kept audio beside a cut is played to check it.
const previewWindow = 8 * time.Second

// Hearable says which end of a track, and how much of it, is played to check
// the field by ear: the cuts, the volume and the fades. A fade is heard whole,
// and a little beyond. ok is false for any other field.
func Hearable(t manifest.Track, field string) (edge app.Edge, window time.Duration, ok bool) {
	f, found := fieldByKey(field)
	if !found {
		return 0, 0, false
	}
	window = previewWindow
	switch f {
	case fCutStart, fVolume:
		return app.EdgeStart, window, true
	case fCutEnd:
		return app.EdgeEnd, window, true
	case fFadeIn:
		return app.EdgeStart, max(window, t.FadeIn.End-t.FadeIn.Start+2*time.Second), true
	case fFadeOut:
		return app.EdgeEnd, max(window, t.FadeOut.End-t.FadeOut.Start+2*time.Second), true
	}
	return 0, 0, false
}

// invert turns moves of rows round: from the new key to the old.
func invert(moves map[string]string) map[string]string {
	if len(moves) == 0 {
		return nil
	}
	out := make(map[string]string, len(moves))
	for from, to := range moves {
		out[to] = from
	}
	return out
}

// noteMoves says that the change just recorded renamed rows of the tree, so that
// undoing and redoing it can say so.
func (e *Editor) noteMoves(moves map[string]string) {
	if top := len(e.undoes) - 1; top >= 0 && len(moves) > 0 {
		e.undoes[top].moves = moves
	}
}
