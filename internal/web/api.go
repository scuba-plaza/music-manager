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
	n, known, err := s.app.Unsynced()
	if err != nil {
		n, known = 0, false
	}
	writeJSON(w, map[string]any{
		"stamp":    stamp,
		"view":     browse.Library(snap, o),
		"unsynced": map[string]any{"n": n, "known": known},
	})
}

func (s *Server) track(w http.ResponseWriter, r *http.Request) {
	snap, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	v, ok := browse.Track(snap, r.PathValue("id"), browse.TrackOptions{Wolfgang: s.wolfgang, CoverInfo: s.describeCover})
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
