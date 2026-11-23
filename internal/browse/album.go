package browse

import (
	"context"
	"errors"
	"maps"
	"net/url"
	"strings"

	"music-manager/internal/app"
)

const topicSuffix = " - Topic"

const albumNote = "From the playlist: check the album and the artist. A year or genre left empty is taken from each track."

var playlistHosts = map[string]bool{
	"youtube.com":       true,
	"www.youtube.com":   true,
	"m.youtube.com":     true,
	"music.youtube.com": true,
}

func IsPlaylistLink(link string) bool {
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	return playlistHosts[u.Host] && u.Path == "/playlist" && u.Query().Get("list") != ""
}

type AlbumForm struct {
	Album    string `json:"album"`
	Artist   string `json:"artist"`
	Released string `json:"released"`
	Genre    string `json:"genre"`
}

func (f AlbumForm) Edit() (app.Edit, error) {
	var e app.Edit
	parts := []draft{
		{field: fAlbum, text: f.Album},
		{field: fArtist, text: f.Artist},
		{field: fReleased, text: f.Released},
		{field: fGenre, text: f.Genre},
	}
	bad := app.FieldErrors{}
	if strings.TrimSpace(f.Album) == "" {
		bad.Set(fAlbum.key(), "Album cannot be empty")
	}
	for _, p := range parts {
		part, err := p.edit()
		var fields app.FieldErrors
		if errors.As(err, &fields) {
			maps.Copy(bad, fields)
			continue
		}
		if err != nil {
			return app.Edit{}, err
		}
		e = mergeEdits(e, part)
	}
	if err := bad.OrNil(); err != nil {
		return app.Edit{}, err
	}
	return e, nil
}

type AlbumTrackView struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Seconds int    `json:"seconds"`
}

type AlbumView struct {
	ID     string           `json:"id"`
	Form   AlbumForm        `json:"form"`
	Note   string           `json:"note"`
	Cover  string           `json:"cover"`
	Tracks []AlbumTrackView `json:"tracks"`
}

func albumDefaults(al *app.Album) AlbumForm {
	return AlbumForm{Album: al.Title, Artist: strings.TrimSuffix(al.Uploader, topicSuffix)}
}

func albumView(id string, al *app.Album) *AlbumView {
	tracks := make([]AlbumTrackView, 0, len(al.Tracks))
	for i, t := range al.Tracks {
		tracks = append(tracks, AlbumTrackView{Number: i + 1, Title: t.Title, Seconds: int(t.Length.Seconds())})
	}
	return &AlbumView{ID: id, Form: albumDefaults(al), Note: albumNote, Cover: al.CoverSHA(), Tracks: tracks}
}

func (e *Editor) fetchAlbum(ctx context.Context, link string) (Fetched, error) {
	al, err := e.lib.FetchAlbum(ctx, link, e.downloader)
	if err != nil {
		return Fetched{}, err
	}
	id, err := newID()
	if err != nil {
		return Fetched{}, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.albums.put(id, al)
	return Fetched{Album: albumView(id, al)}, nil
}
