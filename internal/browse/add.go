package browse

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"music-manager/internal/app"
	"music-manager/internal/device"
	"music-manager/internal/layout"
	"music-manager/internal/manifest"
)

// This file is adding a track from a link: the download and what it found, the
// form of details, and putting the track in the library.

// AddForm is the texts of the form that a downloaded track is added with.
type AddForm struct {
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	Track    string `json:"track"`
	Of       string `json:"of"`
	Released string `json:"released"`
	Genre    string `json:"genre"`
	CutStart string `json:"cutStart"`
	CutEnd   string `json:"cutEnd"`
}

// addDefaults is what to offer for a downloaded track: what the MusicBrainz
// match says, if there is one and it is wanted, and otherwise what the
// download itself reported (taking the uploader of a "- Topic" channel for
// the artist).
func addDefaults(info app.Info) AddForm {
	artist := info.Artist
	if artist == "" {
		artist = strings.TrimSuffix(info.Uploader, " - Topic")
	}
	return AddForm{
		Title: info.Title, Artist: artist, Album: info.Album,
		Track: numberText(info.TrackNumber), Of: numberText(info.TrackTotal), Released: info.Released,
	}
}

// Edit turns the form into the details of the track.
func (f AddForm) Edit() (app.Edit, error) {
	var e app.Edit
	parts := []draft{
		{field: fTitle, text: f.Title},
		{field: fArtist, text: f.Artist},
		{field: fAlbum, text: f.Album},
		{field: fTrack, text: f.Track, second: f.Of},
		{field: fReleased, text: f.Released},
		{field: fGenre, text: f.Genre},
		{field: fCutStart, text: f.CutStart},
		{field: fCutEnd, text: f.CutEnd},
	}
	for _, p := range parts {
		part, err := p.edit()
		if err != nil {
			return e, err
		}
		e = mergeEdits(e, part)
	}
	return e, nil
}

// mergeEdits is a with every field b sets.
func mergeEdits(a, b app.Edit) app.Edit {
	if b.Title != nil {
		a.Title = b.Title
	}
	if b.Artist != nil {
		a.Artist = b.Artist
	}
	if b.Album != nil {
		a.Album = b.Album
	}
	if b.TrackNumber != nil {
		a.TrackNumber = b.TrackNumber
	}
	if b.TrackTotal != nil {
		a.TrackTotal = b.TrackTotal
	}
	if b.Released != nil {
		a.Released = b.Released
	}
	if b.Genre != nil {
		a.Genre = b.Genre
	}
	if b.TrimStart != nil {
		a.TrimStart = b.TrimStart
	}
	if b.TrimEnd != nil {
		a.TrimEnd = b.TrimEnd
	}
	return a
}

// AddPath is where the track would be written, or what is missing to say.
func AddPath(f AddForm) string {
	title, artist := strings.TrimSpace(f.Title), strings.TrimSpace(f.Artist)
	if title == "" || artist == "" {
		return "fill in a title and an artist"
	}
	n, _ := count(f.Track)
	of, _ := count(f.Of)
	return layout.PathFor(manifest.Track{
		ID: "new", Status: manifest.Identified,
		Title: title, Artist: artist, Album: strings.TrimSpace(f.Album),
		TrackNumber: n, TrackTotal: of,
	}, device.Generic.Layout())
}

// MatchView is the recording a download was matched with on MusicBrainz.
type MatchView struct {
	Artist string `json:"artist"`
	Title  string `json:"title"`
	Album  string `json:"album"`
}

// DraftView is a downloaded track, waiting for its details.
type DraftView struct {
	Draft string     `json:"draft"`
	Match *MatchView `json:"match,omitempty"`
	// Matched is the form to start from if the match is used, and Downloaded
	// the form from what the download said; they are the same without a match.
	Matched    AddForm `json:"matched"`
	Downloaded AddForm `json:"downloaded"`
}

// DuplicateView is the track that a download turned out to be already.
type DuplicateView struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

// Fetched is what a download came to: a draft to add, or the track that is there already.
type Fetched struct {
	Draft     *DraftView     `json:"draft,omitempty"`
	Duplicate *DuplicateView `json:"duplicate,omitempty"`
}

// maxDrafts is how many downloads wait to be added; the oldest are forgotten.
const maxDrafts = 8

// CanAdd reports whether tracks can be downloaded.
func (e *Editor) CanAdd() bool { return e.downloader != nil }

// CleanLink strips the spaces and line breaks that come with a paste.
func CleanLink(s string) string { return strings.Join(strings.Fields(s), "") }

// Fetch downloads the audio at the link and prepares it to be added. A track
// that is in the library already comes back as a Duplicate, not an error.
func (e *Editor) Fetch(ctx context.Context, link string) (Fetched, error) {
	if e.downloader == nil {
		return Fetched{}, errors.New("Adding from YouTube is not available: yt-dlp is not set up")
	}
	link = CleanLink(link)
	if link == "" {
		return Fetched{}, errors.New("Paste a link first")
	}
	d, err := e.lib.Fetch(ctx, app.AddRequest{URL: link, Source: e.source}, e.downloader)
	var dup *app.DuplicateError
	if errors.As(err, &dup) {
		s, serr := e.snapshot()
		if serr != nil {
			return Fetched{}, serr
		}
		t, _ := s.Track(dup.ID)
		artist, title, _ := app.Display(t)
		return Fetched{Duplicate: &DuplicateView{ID: dup.ID, Title: title, Artist: artist}}, nil
	}
	if err != nil {
		return Fetched{}, err
	}
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return Fetched{}, err
	}
	v := DraftView{Draft: hex.EncodeToString(raw[:]), Matched: addDefaults(d.Info), Downloaded: addDefaults(d.Downloaded)}
	if d.Match != nil {
		v.Match = &MatchView{Artist: d.Match.Artist, Title: d.Match.Title, Album: d.Match.Album}
	} else {
		v.Matched = v.Downloaded
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.drafts == nil {
		e.drafts = map[string]*app.Draft{}
	}
	if len(e.draftOrder) >= maxDrafts {
		delete(e.drafts, e.draftOrder[0])
		e.draftOrder = e.draftOrder[1:]
	}
	e.drafts[v.Draft] = d
	e.draftOrder = append(e.draftOrder, v.Draft)
	return Fetched{Draft: &v}, nil
}

// Discard forgets a draft that is not going to be added.
func (e *Editor) Discard(draft string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.drafts, draft)
	for i, id := range e.draftOrder {
		if id == draft {
			e.draftOrder = append(e.draftOrder[:i:i], e.draftOrder[i+1:]...)
			break
		}
	}
}

// Finish adds a downloaded track to the library with the details of the form.
func (e *Editor) Finish(ctx context.Context, draft string, f AddForm) (Result, error) {
	e.mu.Lock()
	d, ok := e.drafts[draft]
	e.mu.Unlock()
	if !ok {
		return Result{}, errors.New("That download is gone: fetch the link again")
	}
	edit, err := f.Edit()
	if err != nil {
		return Result{}, err
	}
	// the library is written without the editor locked: it may take a moment
	t, err := e.lib.Finish(ctx, d, edit)
	if err != nil {
		return Result{}, err
	}
	e.Discard(draft)
	e.mu.Lock()
	defer e.mu.Unlock()
	e.record(change{label: "addition of a track", after: []manifest.Track{t}})
	return Result{Message: fmt.Sprintf("Added %q", t.Title), Reveal: t.ID}, nil
}

// Ingested records tracks that were added to the library from files, as one step to undo.
func (e *Editor) Ingested(added []manifest.Track) {
	if len(added) == 0 {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.record(change{label: "addition of " + plural(len(added), "track"), after: added})
}
