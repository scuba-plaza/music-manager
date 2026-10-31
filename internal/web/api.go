package web

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // covers are JPEG or PNG
	_ "image/png"
	"net/http"
	"os"
	"strings"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/browse"
	"music-manager/internal/coverimg"
	"music-manager/internal/device"
)

// snapshot reads the records as they are now.
func (s *Server) snapshot() (browse.Snapshot, error) {
	m, err := s.app.Manifest()
	if err != nil {
		return browse.Snapshot{}, err
	}
	return browse.Snapshot{Tracks: m.Tracks, Playlists: m.Playlists}, nil
}

// libraryOptions reads what the left side shows from the query.
func libraryOptions(r *http.Request) (browse.Options, error) {
	q := r.URL.Query()
	o := browse.Options{Filter: q.Get("q"), Reverse: q.Get("reverse") == "1"}
	if k := q.Get("sort"); k != "" {
		key, err := app.ParseSortKey(k)
		if err != nil {
			return o, err
		}
		o.Sort = key
	}
	return o, nil
}

type UnsyncedView struct {
	Known bool `json:"known"`
	N     int  `json:"n"`
}

type LibraryResponse struct {
	History  browse.History     `json:"history"`
	Stamp    string             `json:"stamp"`
	Unsynced UnsyncedView       `json:"unsynced"`
	View     browse.LibraryView `json:"view"`
}

func (s *Server) library(w http.ResponseWriter, r *http.Request) {
	o, err := libraryOptions(r)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	stamp := s.stamp() // before reading, so that a change in between is noticed again
	snap, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	n, known := s.unsynced()
	writeJSON(w, LibraryResponse{
		History:  s.ed.History(),
		Stamp:    stamp,
		Unsynced: UnsyncedView{Known: known, N: n},
		View:     browse.Library(snap, o),
	})
}

func (s *Server) track(w http.ResponseWriter, r *http.Request) {
	snap, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	profile := r.URL.Query().Get("profile")
	if profile != "" {
		if _, err := s.profile(profile); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
	}
	v, ok := browse.Track(snap, r.PathValue("id"), browse.TrackOptions{Wolfgang: s.wolfgang, CoverInfo: s.describeCover, Profile: profile, Profiles: s.profiles})
	if !ok {
		fail(w, http.StatusNotFound, errNotFound)
		return
	}
	writeJSON(w, v)
}

func (s *Server) peek(w http.ResponseWriter, r *http.Request) {
	snap, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	v, ok := browse.Peek(snap, r.URL.Query().Get("key"), r.URL.Query().Get("q"))
	if !ok {
		fail(w, http.StatusNotFound, errNotFound)
		return
	}
	writeJSON(w, v)
}

// cover serves the cover of a track. The address carries the cover's hash as
// v only so that a new cover is a new address: the file is always found from
// the track.
func (s *Server) cover(w http.ResponseWriter, r *http.Request) {
	m, err := s.app.Manifest()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	t, ok := m.Get(r.PathValue("id"))
	if !ok || t.CoverSHA256 == "" {
		fail(w, http.StatusNotFound, errNotFound)
		return
	}
	data, err := os.ReadFile(s.app.CoverPath(t.CoverSHA256))
	if err != nil {
		fail(w, http.StatusNotFound, errNotFound)
		return
	}
	kind := coverimg.Sniff(data)
	if kind == "" {
		fail(w, http.StatusUnsupportedMediaType, errors.New("the cover is not a JPEG or PNG image"))
		return
	}
	w.Header().Set("Content-Type", kind)
	w.Header().Set("ETag", `"`+t.CoverSHA256+`"`)
	w.Header().Set("Cache-Control", "private, max-age=31536000")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
}

// describeCover says what a stored cover is, for example "500×500 JPEG".
func (s *Server) describeCover(sha string) string {
	s.infoMu.Lock()
	defer s.infoMu.Unlock()
	if info, ok := s.coverInfo[sha]; ok {
		return info
	}
	info := "unreadable image"
	if data, err := os.ReadFile(s.app.CoverPath(sha)); err == nil {
		if cfg, format, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
			info = fmt.Sprintf("%d×%d %s", cfg.Width, cfg.Height, strings.ToUpper(format))
		}
	}
	s.coverInfo[sha] = info
	return info
}

// doctor checks that every stored file is there and unharmed.
func (s *Server) doctor(w http.ResponseWriter, r *http.Request) {
	found, err := s.app.Doctor()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	snap, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, browse.Doctor(snap, found))
}

type ProfileView struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Builtin bool   `json:"builtin"`
}

func (s *Server) listProfiles(w http.ResponseWriter, r *http.Request) {
	infos := s.profiles.Profiles()
	views := make([]ProfileView, len(infos))
	for i, p := range infos {
		views[i] = ProfileView{Name: p.Name, Label: p.Label, Builtin: p.Builtin}
	}
	writeJSON(w, views)
}

func (s *Server) profile(name string) (device.Target, error) {
	if dev, ok := s.profiles.Get(name); ok {
		return dev, nil
	}
	return nil, fmt.Errorf("there is no device profile named %q", name)
}
