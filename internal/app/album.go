package app

import (
	"context"
	"fmt"
	"time"
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
