package web

import (
	"context"
	"net/http"

	"music-manager/internal/browse"
)

type AlbumProgress struct {
	Done  int    `json:"done"`
	Total int    `json:"total"`
	Title string `json:"title"`
}

func (s *Server) addAlbum(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID     string
		Form   browse.AlbumForm
		Tracks []int
	}
	if !read(w, r, &in) {
		return
	}
	if err := s.ed.CheckAlbum(in.ID, in.Form, in.Tracks); err != nil {
		fail(w, http.StatusUnprocessableEntity, err)
		return
	}
	s.startJob(w, "addalbum", "an album is being added already", func(ctx context.Context, report func(any)) (any, error) {
		res, err := s.ed.AddAlbum(ctx, in.ID, in.Form, in.Tracks, func(done, total int, title string) {
			report(AlbumProgress{Done: done, Total: total, Title: title})
		})
		if err != nil {
			return nil, err
		}
		return ChangeResponse{res, s.ed.History()}, nil
	})
}
