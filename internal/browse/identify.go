package browse

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

// This file is everything the editor does with a music database: looking a
// track up and choosing what it is, identifying all that need it, and using
// the tags a file had.

// DiffLine is one row of the preview of what a candidate would change.
type DiffLine struct {
	Label string `json:"label"`
	Old   string `json:"old"`
	New   string `json:"new"`
	Same  bool   `json:"same"`
}

// candidateDiff compares a track as it is with a candidate it could become.
func candidateDiff(t manifest.Track, c app.Candidate) []DiffLine {
	orDash := func(s string) string {
		if s == "" {
			return "—"
		}
		return s
	}
	line := func(label, old, new string) DiffLine {
		return DiffLine{label, orDash(old), orDash(new), orDash(old) == orDash(new)}
	}
	newTrack := ""
	switch {
	case c.TrackNumber > 0 && c.TrackTotal > 0:
		newTrack = fmt.Sprintf("%d of %d", c.TrackNumber, c.TrackTotal)
	case c.TrackNumber > 0:
		newTrack = fmt.Sprint(c.TrackNumber)
	}
	return []DiffLine{
		line("Title", name(t, fTitle), c.Title),
		line("Artist", name(t, fArtist), c.Artist),
		line("Album", name(t, fAlbum), c.Album),
		line("Track", displayValue(t, fTrack), newTrack),
	}
}

// candidateText is one row of the list. What you choose by is the release, so
// the album leads; the title is only there if this recording is not the one the
// first row has (a live or shortened version).
func candidateText(c app.Candidate, firstTitle string) string {
	album := c.Album
	if album == "" {
		album = "(no album)"
	}
	s := fmt.Sprintf("%3d%%  %s", c.Score, album)
	if c.Date != "" {
		s += " · " + c.Date
	}
	if c.TrackNumber > 0 {
		s += fmt.Sprintf(" · %d/%d", c.TrackNumber, c.TrackTotal)
	}
	s += " · " + c.Artist
	if c.Title != firstTitle {
		s += " · " + c.Title
	}
	return s
}

// CandidateView is one way a track could be described, as the list shows it.
type CandidateView struct {
	Text string     `json:"text"`
	Diff []DiffLine `json:"diff"`
}

// SearchView is what a search found. Search names it, so that choosing one of
// the candidates only has to say which.
type SearchView struct {
	Search     string          `json:"search"`
	Title      string          `json:"title"` // the words searched for
	Artist     string          `json:"artist"`
	Candidates []CandidateView `json:"candidates"`
}

// search is what a search found, kept until one of it is chosen.
type search struct {
	track string
	cands []app.Candidate
}

// maxSearches is how many searches are kept; the oldest are forgotten.
const maxSearches = 32

// SearchTimeout bounds one search, in case the service never answers.
const SearchTimeout = 40 * time.Second

// ErrNoLookup is the refusal when there is no music database.
var ErrNoLookup = errors.New("Lookup is not available: no MusicBrainz client is set up")

// Lookup reports whether a music database can be searched at all.
func (e *Editor) Lookup() bool { return e.source != nil }

// Candidates searches the music database for what a track could be: for the
// words in q if there are any, else for the track itself.
func (e *Editor) Candidates(ctx context.Context, id string, q *app.Query) (SearchView, error) {
	if e.source == nil {
		return SearchView{}, ErrNoLookup
	}
	e.mu.Lock()
	s, err := e.snapshot()
	e.mu.Unlock()
	if err != nil {
		return SearchView{}, err
	}
	t, err := track(s, id)
	if err != nil {
		return SearchView{}, err
	}
	v := SearchView{Title: name(t, fTitle), Artist: name(t, fArtist), Candidates: []CandidateView{}}
	if q != nil {
		q = &app.Query{Title: strings.TrimSpace(q.Title), Artist: strings.TrimSpace(q.Artist)}
		v.Title, v.Artist = q.Title, q.Artist
	}
	cands, err := e.lib.Candidates(ctx, id, e.source, q)
	if err != nil {
		return SearchView{}, err
	}
	if len(cands) == 0 {
		return SearchView{}, fmt.Errorf("No match found for “%s – %s”.", v.Title, v.Artist)
	}
	for _, c := range cands {
		v.Candidates = append(v.Candidates, CandidateView{Text: candidateText(c, cands[0].Title), Diff: candidateDiff(t, c)})
	}
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return SearchView{}, err
	}
	v.Search = hex.EncodeToString(raw[:])
	e.mu.Lock()
	if e.searches == nil {
		e.searches = map[string]search{}
	}
	if len(e.searchOrder) >= maxSearches {
		delete(e.searches, e.searchOrder[0])
		e.searchOrder = e.searchOrder[1:]
	}
	e.searches[v.Search] = search{track: id, cands: cands}
	e.searchOrder = append(e.searchOrder, v.Search)
	e.mu.Unlock()
	return v, nil
}

// ApplyCandidate identifies the track a search was for as the candidate at
// index, fetching its cover; ctx may be cancelled to go without the cover.
func (e *Editor) ApplyCandidate(ctx context.Context, searchID string, index int) (Result, error) {
	if e.source == nil {
		return Result{}, ErrNoLookup
	}
	e.mu.Lock()
	found, ok := e.searches[searchID]
	var before manifest.Track
	var err error
	if ok && index >= 0 && index < len(found.cands) {
		var s Snapshot
		if s, err = e.snapshot(); err == nil {
			before, err = track(s, found.track)
		}
	} else {
		ok = false
	}
	e.mu.Unlock()
	if !ok {
		return Result{}, errors.New("That search is gone: search again")
	}
	if err != nil {
		return Result{}, err
	}
	// the network is not waited for with the editor locked
	after, err := e.lib.ApplyCandidate(ctx, before.ID, found.cands[index], e.source)
	if err != nil {
		return Result{}, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.record(change{label: "identification of a track", before: []manifest.Track{before}, after: []manifest.Track{after}})
	return Result{Message: "Identified as " + after.Artist + " - " + after.Title, Reveal: after.ID}, nil
}

// IdentifyProgress is how far an identification of every track that needs it has got.
type IdentifyProgress struct {
	Total      int      `json:"total"`
	Done       int      `json:"done"`
	Identified int      `json:"identified"`
	Left       int      `json:"left"`
	Failed     int      `json:"failed"`
	Failures   []string `json:"failures"`
}

// NeedWork counts the tracks that are not identified.
func (e *Editor) NeedWork() (int, error) {
	s, err := e.snapshot()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, t := range s.Tracks {
		if t.Status != manifest.Identified {
			n++
		}
	}
	return n, nil
}

// IdentifyAll takes the confident match for every track that needs work,
// telling each as it goes. All it identified is one step to undo, even if ctx
// ended it early.
func (e *Editor) IdentifyAll(ctx context.Context, each func(IdentifyProgress)) (IdentifyProgress, error) {
	p := IdentifyProgress{Failures: []string{}}
	if e.source == nil {
		return p, ErrNoLookup
	}
	need, err := e.NeedWork()
	if err != nil {
		return p, err
	}
	p.Total = need
	e.mu.Lock()
	s, err := e.snapshot()
	e.mu.Unlock()
	if err != nil {
		return p, err
	}
	var applied []string
	_, runErr := e.lib.Identify(ctx, nil, e.source, app.IdentifyOptions{OnResult: func(r app.IdentifyResult) {
		p.Done++
		switch {
		case r.Err != nil:
			p.Failed++
			p.Failures = append(p.Failures, r.Err.Error())
		case r.Applied != nil:
			p.Identified++
			applied = append(applied, r.TrackID)
		default:
			p.Left++
		}
		each(p)
	}})
	if runErr != nil && len(p.Failures) == 0 && !errors.Is(runErr, context.Canceled) {
		p.Failures = append(p.Failures, runErr.Error())
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(applied) > 0 {
		var before, after []manifest.Track
		now, err := e.snapshot()
		if err != nil {
			return p, err
		}
		for _, id := range applied {
			b, ok1 := s.Track(id)
			a, ok2 := now.Track(id)
			if ok1 && ok2 {
				before, after = append(before, b), append(after, a)
			}
		}
		e.record(change{label: "identification of " + plural(len(after), "track"), before: before, after: after})
	}
	return p, nil
}

// Adopt identifies the tracks by the tags their files had.
func (e *Editor) Adopt(ids []string) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	var todo []string
	for _, t := range s.tracksFor(ids) {
		if t.Status != manifest.Identified {
			todo = append(todo, t.ID)
		}
	}
	if len(todo) == 0 {
		return Result{}, errors.New("Select a track that needs work, or the Needs work group")
	}
	results, err := e.lib.Adopt(todo)
	if err != nil {
		return Result{}, err
	}
	adopted := 0
	for _, r := range results {
		if r.Adopted {
			adopted++
		}
	}
	if adopted == 0 {
		return Result{}, errors.New("These files have no old tags to use")
	}
	now, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	var before, after []manifest.Track
	for _, id := range todo {
		b, _ := s.Track(id)
		if a, ok := now.Track(id); ok && a.Status == manifest.Identified && b.Status != manifest.Identified {
			before, after = append(before, b), append(after, a)
		}
	}
	e.record(change{label: "use of old tags", before: before, after: after})
	text := "Used the old tags of " + plural(len(after), "track")
	if rest := len(todo) - len(after); rest > 0 {
		text += fmt.Sprintf(" · %d had none", rest)
	}
	r := Result{Message: text}
	if len(after) > 0 {
		r.Reveal = after[0].ID
	}
	return r, nil
}
