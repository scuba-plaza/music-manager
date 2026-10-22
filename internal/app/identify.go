package app

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"music-manager/internal/manifest"
)

// defaultMinScore is how well a candidate must match to be taken without asking.
const defaultMinScore = 95

// IdentifyOptions tunes Identify.
type IdentifyOptions struct {
	// Yes takes the best candidate without asking, if it scores at least
	// MinScore, and skips the track otherwise.
	Yes      bool
	MinScore int // 95 if zero
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
	for _, t := range todo {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		res := IdentifyResult{TrackID: t.ID}
		q := queryFor(t)
		if q.Title == "" {
			res.Reason = "nothing to search for"
			results = append(results, res)
			continue
		}
		cands, err := src.Search(ctx, q)
		if err != nil {
			res.Err = err
			results = append(results, res)
			continue
		}
		pick, reason, err := choose(t, cands, p, opts)
		if err != nil {
			return results, err
		}
		if pick == nil {
			res.Reason = reason
			results = append(results, res)
			continue
		}
		updated, err := a.applyCandidate(ctx, t, *pick, src)
		if err != nil {
			res.Err = err
			results = append(results, res)
			continue
		}
		m.Update(updated)
		if err := a.store.Save(m); err != nil {
			return results, err
		}
		res.Applied = pick
		results = append(results, res)
	}
	return results, nil
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

// choose picks one of cands, or returns nil and why not.
func choose(t manifest.Track, cands []Candidate, p Prompter, opts IdentifyOptions) (*Candidate, string, error) {
	if len(cands) == 0 {
		return nil, "no match found", nil
	}
	if opts.Yes {
		if cands[0].Score < opts.MinScore {
			return nil, fmt.Sprintf("best match scores only %d", cands[0].Score), nil
		}
		return &cands[0], "", nil
	}
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
