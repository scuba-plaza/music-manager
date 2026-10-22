package app

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"music-manager/internal/manifest"
)

// maxChoices is how many candidates an interactive choice lists.
const maxChoices = 15

// defaultMinScore is how well a candidate must match to be taken without asking.
const defaultMinScore = 95

// IdentifyOptions tunes Identify.
type IdentifyOptions struct {
	// Yes takes the best candidate without asking, if it scores at least
	// MinScore, and skips the track otherwise.
	Yes      bool
	MinScore int // 95 if zero
	// OnResult, if set, is called with each track's result as soon as it is known.
	OnResult func(IdentifyResult)
}

// IdentifyResult says what happened to one track.
type IdentifyResult struct {
	TrackID string
	Applied *Candidate // what the track was identified as, if anything
	Reason  string     // why it was left alone
	Err     error      // a lookup that failed; other tracks carry on
}

// Identify looks up tracks in src and records what they turn out to be. With
// no ids it takes every track that is not identified yet; named tracks are
// looked up again even if identified. Each track is searched for with what
// it is known by: its record, or for new files, the old tags and file name.
func (a *App) Identify(ctx context.Context, ids []string, src MetadataSource, p Prompter, opts IdentifyOptions) ([]IdentifyResult, error) {
	if !opts.Yes && p == nil {
		return nil, errors.New("choosing a match needs a terminal; pass --yes to take confident matches automatically")
	}
	if opts.MinScore == 0 {
		opts.MinScore = defaultMinScore
	}
	m, err := a.store.Load()
	if err != nil {
		return nil, err
	}
	var todo []manifest.Track
	if len(ids) == 0 {
		for _, t := range m.Tracks {
			if t.Status != manifest.Identified {
				todo = append(todo, t)
			}
		}
	}
	for _, id := range ids {
		t, ok := m.Get(id)
		if !ok {
			return nil, fmt.Errorf("no track with id %q", id)
		}
		todo = append(todo, t)
	}

	var results []IdentifyResult
	record := func(r IdentifyResult) {
		results = append(results, r)
		if opts.OnResult != nil {
			opts.OnResult(r)
		}
	}
	for _, t := range todo {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		res := IdentifyResult{TrackID: t.ID}
		cands, err := a.search(ctx, t, src, nil)
		if errors.Is(err, errNothingToSearch) {
			res.Reason = err.Error()
			record(res)
			continue
		}
		if err != nil {
			res.Err = err
			record(res)
			continue
		}
		pick, reason, err := choose(t, cands, p, opts)
		if err != nil {
			return results, err
		}
		if pick == nil {
			res.Reason = reason
			record(res)
			continue
		}
		updated, err := a.applyCandidate(ctx, t, *pick, src)
		if err != nil {
			res.Err = err
			record(res)
			continue
		}
		m.Update(updated)
		if err := a.store.Save(m); err != nil {
			return results, err
		}
		res.Applied = pick
		record(res)
	}
	return results, nil
}

var errNothingToSearch = errors.New("nothing to search for")

// search looks t up in src, by q if given and otherwise by what t is known
// by, and returns the candidates ordered so those fitting the file's old
// album and track number come first.
func (a *App) search(ctx context.Context, t manifest.Track, src MetadataSource, q *Query) ([]Candidate, error) {
	query := queryFor(t)
	if q != nil {
		query = *q
	}
	if strings.TrimSpace(query.Title) == "" {
		return nil, errNothingToSearch
	}
	cands, err := src.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	return preferHint(cands, t.Hint), nil
}

// Candidates lists what the track with the given id could be, best first and
// at most maxChoices of them, without changing anything. q overrides the
// words searched for, which is how to find a track whose name is romanised.
func (a *App) Candidates(ctx context.Context, id string, src MetadataSource, q *Query) ([]Candidate, error) {
	m, err := a.store.Load()
	if err != nil {
		return nil, err
	}
	t, ok := m.Get(id)
	if !ok {
		return nil, fmt.Errorf("no track with id %q", id)
	}
	cands, err := a.search(ctx, t, src, q)
	if err != nil {
		return nil, err
	}
	return cands[:min(len(cands), maxChoices)], nil
}

// ApplyCandidate identifies the track with the given id as c, fetching a
// cover if it has none, and returns the updated track.
func (a *App) ApplyCandidate(ctx context.Context, id string, c Candidate, src MetadataSource) (manifest.Track, error) {
	m, err := a.store.Load()
	if err != nil {
		return manifest.Track{}, err
	}
	t, ok := m.Get(id)
	if !ok {
		return manifest.Track{}, fmt.Errorf("no track with id %q", id)
	}
	updated, err := a.applyCandidate(ctx, t, c, src)
	if err != nil {
		return manifest.Track{}, err
	}
	m.Update(updated)
	return updated, a.store.Save(m)
}

// queryFor is what to search for to find t.
func queryFor(t manifest.Track) Query {
	if t.Status == manifest.Identified {
		return Query{Title: t.OriginalTitle, Artist: t.OriginalArtist}
	}
	title := t.Hint.Title
	if title == "" {
		title = t.Hint.Filename
	}
	title = strings.TrimSpace(title)
	if onlyDigits.MatchString(title) {
		title = "" // a bare number says nothing about the track
	}
	return Query{Title: leadingTrackNumber.ReplaceAllString(title, ""), Artist: t.Hint.Artist}
}

var onlyDigits = regexp.MustCompile(`^\d+$`)

// leadingTrackNumber matches the "01 - " or "07. " that file names start with.
var leadingTrackNumber = regexp.MustCompile(`^\d+\s*[-._)\]]*\s+`)

// preferHint reorders cands so that among equally good matches, those that
// fit what the file said about its album and track number come first. A
// better score always wins: the hint only decides between ties, such as the
// many releases a single recording appears on.
func preferHint(cands []Candidate, h manifest.Hint) []Candidate {
	fit := func(c Candidate) int {
		n := 0
		if h.Album != "" && albumKey(c.Album) == albumKey(h.Album) {
			n += 2
		}
		if h.Track > 0 && c.TrackNumber == h.Track && (h.Total == 0 || c.TrackTotal == h.Total) {
			n++
		}
		return n
	}
	out := append([]Candidate(nil), cands...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return fit(out[i]) > fit(out[j])
	})
	return out
}

// albumKey reduces an album title to what identifies it: lower-case words
// without punctuation or a trailing "EP" / "Single".
func albumKey(s string) string {
	words := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	for len(words) > 1 && (words[len(words)-1] == "ep" || words[len(words)-1] == "single") {
		words = words[:len(words)-1]
	}
	return strings.Join(words, " ")
}

// choose picks one of cands, or returns nil and why not.
func choose(t manifest.Track, cands []Candidate, p Prompter, opts IdentifyOptions) (*Candidate, string, error) {
	if len(cands) == 0 {
		return nil, "no match found", nil
	}
	if opts.Yes {
		if cands[0].Score < opts.MinScore {
			return nil, fmt.Sprintf("best match scores only %d", cands[0].Score), nil
		}
		best := cands[0]
		if h := t.Hint; h.Album != "" && albumKey(best.Album) != albumKey(h.Album) {
			// No release of this recording is the album the file was filed
			// under. Keep that album, so its tracks stay together, and take
			// only the recording from the database.
			best.Album, best.TrackNumber, best.TrackTotal, best.ReleaseID = h.Album, h.Track, h.Total, ""
		}
		return &best, "", nil
	}
	cands = cands[:min(len(cands), maxChoices)]
	options := make([]string, 0, len(cands)+1)
	for _, c := range cands {
		options = append(options, describe(c))
	}
	options = append(options, "None of these, skip this track")
	artist, title, _ := Display(t)
	i, err := p.Choose(fmt.Sprintf("Which is %q?", strings.TrimLeft(artist+" - "+title, " -")), options)
	if err != nil {
		return nil, "", err
	}
	if i < 0 || i >= len(cands) {
		return nil, "skipped", nil
	}
	return &cands[i], "", nil
}

func describe(c Candidate) string {
	s := fmt.Sprintf("%s - %s", c.Artist, c.Title)
	if c.Album != "" {
		s += " · " + c.Album
		if c.Date != "" {
			s += " (" + c.Date + ")"
		}
	}
	if c.TrackNumber > 0 {
		s += fmt.Sprintf(" · %d/%d", c.TrackNumber, c.TrackTotal)
	}
	return fmt.Sprintf("%s · %d%%", s, c.Score)
}

// applyCandidate returns t described as c, with a cover from src if it had none.
func (a *App) applyCandidate(ctx context.Context, t manifest.Track, c Candidate, src MetadataSource) (manifest.Track, error) {
	derive := ""
	e := Edit{
		Title: &c.Title, Artist: &c.Artist, Album: &c.Album,
		TitleASCII: &derive, ArtistASCII: &derive, AlbumASCII: &derive, // follow the new text
		TrackNumber: &c.TrackNumber, TrackTotal: &c.TrackTotal,
	}
	if c.ArtistASCII != "" {
		e.ArtistASCII = &c.ArtistASCII
	}
	t, err := applyEdit(t, e)
	if err != nil {
		return t, err
	}
	t.MBRecordingID, t.MBReleaseID = c.RecordingID, c.ReleaseID
	if t.CoverSHA256 == "" {
		// A missing cover is not worth failing the identification over.
		if img, err := src.CoverFront(ctx, c.ReleaseID); err == nil && len(img) > 0 {
			if sha, err := a.storeCover(img); err == nil {
				t.CoverSHA256 = sha
			}
		}
	}
	return t, nil
}

// Display returns the artist, title and album to show for t: its own when
// identified, what the ingested file said about itself otherwise.
func Display(t manifest.Track) (artist, title, album string) {
	if t.Status == manifest.Identified {
		return t.Artist, t.Title, t.Album
	}
	title = t.Hint.Title
	if title == "" {
		title = t.Hint.Filename
	}
	return t.Hint.Artist, title, t.Hint.Album
}
