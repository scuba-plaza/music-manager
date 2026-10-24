package web

import (
	"context"
	"errors"
	"net/http"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/browse"
)

func (s *Server) identifyRoutes() {
	s.mux.HandleFunc("POST /api/tracks/{id}/candidates", s.candidates)
	s.mux.HandleFunc("POST /api/identify/apply", s.applyCandidate)
	s.mux.HandleFunc("POST /api/identify-all", s.identifyAll)
	s.mux.HandleFunc("POST /api/adopt", s.adopt)
	s.jobRoutes()
}

// lookupError says in words why a lookup did not come to anything.
func lookupError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("the music database took too long to answer: try again")
	}
	return err
}

// candidates searches for what a track could be: for the words in query if it
// is there, else for the track itself. The search ends when the page gives up on it.
func (s *Server) candidates(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Query *struct{ Title, Artist string }
	}
	if !read(w, r, &in) {
		return
	}
	var q *app.Query
	if in.Query != nil {
		q = &app.Query{Title: in.Query.Title, Artist: in.Query.Artist}
	}
	ctx, cancel := context.WithTimeout(r.Context(), browse.SearchTimeout)
	defer cancel()
	v, err := s.ed.Candidates(ctx, r.PathValue("id"), q)
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, lookupError(err))
		return
	}
	writeJSON(w, v)
}

// applyCandidate identifies the track as one of the candidates a search found.
// If the page gives up while the cover is fetched, the track is identified without it.
func (s *Server) applyCandidate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Search string
		Index  int
	}
	if !read(w, r, &in) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	res, err := s.ed.ApplyCandidate(ctx, in.Search, in.Index)
	s.answer(w, res, lookupError(err))
}

// identifyAll starts identifying every track that needs work, as a job.
func (s *Server) identifyAll(w http.ResponseWriter, r *http.Request) {
	if !s.ed.Lookup() {
		fail(w, http.StatusUnprocessableEntity, browse.ErrNoLookup)
		return
	}
	need, err := s.ed.NeedWork()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	if need == 0 {
		fail(w, http.StatusUnprocessableEntity, errors.New("nothing needs work"))
		return
	}
	v, err := s.jobs.start("identify", func(ctx context.Context, report func(any)) (any, error) {
		return s.ed.IdentifyAll(ctx, func(p browse.IdentifyProgress) { report(p) })
	})
	if errors.Is(err, errBusy) {
		fail(w, http.StatusConflict, errors.New("identifying is going on already"))
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, v)
}

// adopt identifies tracks by the tags their files had.
func (s *Server) adopt(w http.ResponseWriter, r *http.Request) {
	var in struct{ IDs []string }
	if !read(w, r, &in) {
		return
	}
	res, err := s.ed.Adopt(in.IDs)
	s.answer(w, res, err)
}
