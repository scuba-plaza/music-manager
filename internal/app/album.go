package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"music-manager/internal/manifest"
)

const MaxAlbumTracks = 30

const DownloadTimeout = 10 * time.Minute

type ListedTrack struct {
	URL, Title string
	Length     time.Duration
}

type Listing struct {
	Title, Uploader string
	Tracks          []ListedTrack
	Cover           []byte
}

type Album struct {
	URL, Title, Uploader string
	Tracks               []ListedTrack
	coverSHA             string
}

func (al *Album) CoverSHA() string { return al.coverSHA }

func (a *App) FetchAlbum(ctx context.Context, url string, dl Downloader) (*Album, error) {
	dir, remove, err := a.downloadDir()
	if err != nil {
		return nil, err
	}
	defer remove()

	l, err := dl.List(ctx, url, dir, MaxAlbumTracks+1)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", url, err)
	}
	if len(l.Tracks) > MaxAlbumTracks {
		return nil, fmt.Errorf("this playlist has more than %d tracks: only playlists of up to %d tracks can be added", MaxAlbumTracks, MaxAlbumTracks)
	}
	if len(l.Tracks) == 0 {
		return nil, fmt.Errorf("this playlist has no tracks to add")
	}
	al := &Album{URL: url, Title: l.Title, Uploader: l.Uploader, Tracks: l.Tracks}
	if sha, err := a.StoreCover(l.Cover); err == nil && sha != "" {
		al.coverSHA = sha
	}
	return al, nil
}

func (al *Album) Check(numbers []int) error {
	if len(numbers) == 0 {
		return errors.New("pick at least one track")
	}
	prev := 0
	for _, n := range numbers {
		if n < 1 || n > len(al.Tracks) || n <= prev {
			return errors.New("the chosen tracks are not in this playlist")
		}
		prev = n
	}
	return nil
}

type AlbumFailure struct {
	Title string
	Err   error
}

type AlbumReport struct {
	Added      []manifest.Track
	Duplicates int
	Failed     []AlbumFailure
}

func (a *App) AddAlbum(ctx context.Context, al *Album, numbers []int, shared Edit, dl Downloader, progress func(done, total int, title string)) (AlbumReport, error) {
	if err := al.Check(numbers); err != nil {
		return AlbumReport{}, err
	}
	type pending struct {
		n     int
		draft *Draft
	}
	var rep AlbumReport
	var drafts []pending
	for i, n := range numbers {
		if err := ctx.Err(); err != nil {
			return AlbumReport{}, err
		}
		lt := al.Tracks[n-1]
		progress(i, len(numbers), lt.Title)
		dctx, cancel := context.WithTimeout(ctx, DownloadTimeout)
		d, err := a.Fetch(dctx, AddRequest{URL: lt.URL}, dl)
		cancel()
		var dup *DuplicateError
		switch {
		case errors.As(err, &dup):
			rep.Duplicates++
		case err != nil:
			if ctx.Err() != nil {
				return AlbumReport{}, ctx.Err()
			}
			rep.Failed = append(rep.Failed, AlbumFailure{Title: lt.Title, Err: err})
		default:
			if al.coverSHA != "" {
				d.staged.coverSHA = al.coverSHA
			}
			drafts = append(drafts, pending{n, d})
		}
	}
	if err := ctx.Err(); err != nil {
		return AlbumReport{}, err
	}
	undo := func() (AlbumReport, error) {
		err := ctx.Err()
		if len(rep.Added) > 0 {
			ids := make([]string, len(rep.Added))
			for i, t := range rep.Added {
				ids[i] = t.ID
			}
			if _, rerr := a.RemoveMany(ids); rerr != nil {
				err = errors.Join(err, rerr)
			}
		}
		return AlbumReport{}, err
	}
	for _, p := range drafts {
		if ctx.Err() != nil {
			return undo()
		}
		t, err := a.Finish(p.draft, albumTrackEdit(shared, p.draft.Info, al.Tracks[p.n-1].Title, p.n, len(al.Tracks)))
		var dup *DuplicateError
		switch {
		case errors.As(err, &dup):
			rep.Duplicates++
		case err != nil:
			rep.Failed = append(rep.Failed, AlbumFailure{Title: al.Tracks[p.n-1].Title, Err: err})
		default:
			rep.Added = append(rep.Added, t)
		}
	}
	progress(len(numbers), len(numbers), "")
	if ctx.Err() != nil {
		return undo()
	}
	return rep, nil
}

func albumTrackEdit(shared Edit, info Info, title string, number, total int) Edit {
	e := shared
	e.Title = &title
	e.TrackNumber = &number
	e.TrackTotal = &total
	if e.Released == nil || *e.Released == "" {
		r := info.Released
		e.Released = &r
	}
	if e.Genre == nil || *e.Genre == "" {
		g := info.Genre
		e.Genre = &g
	}
	return e
}
