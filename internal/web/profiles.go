package web

import (
	"errors"
	"net/http"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

func (s *Server) profileRoutes() {
	s.mux.HandleFunc("POST /api/profiles", s.profileCreate)
	s.mux.HandleFunc("POST /api/profiles/check", s.profileCheck)
	s.mux.HandleFunc("PUT /api/profiles/{id}", s.profileUpdate)
	s.mux.HandleFunc("DELETE /api/profiles/{id}", s.profileDelete)
}

type ProfileCheck struct {
	Fields map[string]string `json:"fields,omitempty"`
}

func (s *Server) profileCheck(w http.ResponseWriter, r *http.Request) {
	var in manifest.Profile
	if !read(w, r, &in) {
		return
	}
	fields, err := s.app.CheckProfile(in)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, ProfileCheck{Fields: fields})
}

func (s *Server) profileCreate(w http.ResponseWriter, r *http.Request) {
	var in manifest.Profile
	if !read(w, r, &in) {
		return
	}
	made, err := s.app.CreateProfile(in)
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, made)
}

func (s *Server) profileUpdate(w http.ResponseWriter, r *http.Request) {
	var in manifest.Profile
	if !read(w, r, &in) {
		return
	}
	changed, err := s.app.UpdateProfile(r.PathValue("id"), in)
	if errors.Is(err, app.ErrNoFormat) {
		fail(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, changed)
}

func (s *Server) profileDelete(w http.ResponseWriter, r *http.Request) {
	err := s.app.DeleteProfile(r.PathValue("id"))
	if errors.Is(err, app.ErrNoFormat) {
		fail(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
