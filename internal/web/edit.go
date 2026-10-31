package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/browse"
)

// The routes that change the library. Each takes JSON (or, for a picture, the
// bytes of the picture) and answers with a browse.Result and what undo, redo
// and paste would do now. A refusal is a 422 with the reason in words.

const (
	maxJSON    = 1 << 20  // a request body that is not a picture
	maxPicture = 32 << 20 // a cover
	maxPixels  = 60e6     // a picture of more pixels is refused before it is decoded
)

func (s *Server) editRoutes() {
	m := s.mux
	m.HandleFunc("PATCH /api/tracks/{id}", s.setField)
	m.HandleFunc("POST /api/tracks/{id}/nudge", s.nudge)
	m.HandleFunc("POST /api/tracks/{id}/clear", s.clear)
	m.HandleFunc("POST /api/tracks/{id}/path", s.pathPreview)
	m.HandleFunc("GET /api/tracks/{id}/copyable", s.copyable)
	m.HandleFunc("GET /api/tracks/{id}/preview", s.previewAudio)
	m.HandleFunc("PUT /api/cover", s.putCover)
	m.HandleFunc("DELETE /api/cover", s.deleteCover)
	m.HandleFunc("PATCH /api/group", s.setGroupField)
	m.HandleFunc("POST /api/group/hint", s.groupHint)
	m.HandleFunc("POST /api/bulk/preview", s.bulkPreview)
	m.HandleFunc("POST /api/bulk", s.bulk)
	m.HandleFunc("POST /api/remove", s.remove)
	m.HandleFunc("POST /api/renumber", s.renumber)
	m.HandleFunc("POST /api/move", s.move)
	m.HandleFunc("POST /api/playlists", s.playlistNew)
	m.HandleFunc("POST /api/playlists/add", s.playlistAdd)
	m.HandleFunc("POST /api/playlists/takeout", s.playlistTakeOut)
	m.HandleFunc("POST /api/playlists/delete", s.playlistDelete)
	m.HandleFunc("POST /api/copy", s.copy)
	m.HandleFunc("POST /api/paste", s.paste)
	m.HandleFunc("POST /api/undo", s.undo)
	m.HandleFunc("POST /api/redo", s.redo)
}

type PathView struct {
	Path string `json:"path"`
}

type HintView struct {
	Hint string `json:"hint"`
}

type ChangeResponse struct {
	browse.Result
	History browse.History `json:"history"`
}

// answer writes the outcome of a change.
func (s *Server) answer(w http.ResponseWriter, r browse.Result, err error) {
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, ChangeResponse{r, s.ed.History()})
}

// read decodes the JSON body of a request.
func read(w http.ResponseWriter, r *http.Request, into any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSON))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("the request could not be read: %w", err))
		return false
	}
	return true
}

func (s *Server) setField(w http.ResponseWriter, r *http.Request) {
	var in struct{ Field, Text, Second string }
	if !read(w, r, &in) {
		return
	}
	res, err := s.ed.SetField(r.PathValue("id"), in.Field, in.Text, in.Second)
	s.answer(w, res, err)
}

func (s *Server) nudge(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Field string
		More  int
		Big   bool
	}
	if !read(w, r, &in) {
		return
	}
	res, err := s.ed.Nudge(r.PathValue("id"), in.Field, in.More, in.Big)
	s.answer(w, res, err)
}

func (s *Server) clear(w http.ResponseWriter, r *http.Request) {
	var in struct{ Field string }
	if !read(w, r, &in) {
		return
	}
	res, err := s.ed.Clear(r.PathValue("id"), in.Field)
	s.answer(w, res, err)
}

// pathPreview says where the file would go if the text typed were saved.
func (s *Server) pathPreview(w http.ResponseWriter, r *http.Request) {
	var in struct{ Field, Text, Second string }
	if !read(w, r, &in) {
		return
	}
	snap, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	path, ok := browse.PathPreview(snap, r.PathValue("id"), in.Field, in.Text, in.Second)
	if !ok {
		fail(w, http.StatusNotFound, errNotFound)
		return
	}
	writeJSON(w, PathView{Path: path})
}

func (s *Server) copyable(w http.ResponseWriter, r *http.Request) {
	snap, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	c, ok := browse.CopyChoices(snap, r.PathValue("id"))
	if !ok {
		fail(w, http.StatusNotFound, errNotFound)
		return
	}
	writeJSON(w, c)
}

// ---- covers

// picture reads the image sent with a request, and checks it is one before
// anything decodes it: a JPEG or PNG, and not a bomb.
func picture(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPicture))
	if err != nil {
		fail(w, http.StatusRequestEntityTooLarge, errors.New("the picture is too large"))
		return nil, false
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	switch {
	case err != nil || (format != "jpeg" && format != "png"):
		fail(w, http.StatusUnsupportedMediaType, errors.New("a cover must be a JPEG or PNG image"))
		return nil, false
	case float64(cfg.Width)*float64(cfg.Height) > maxPixels:
		fail(w, http.StatusRequestEntityTooLarge, errors.New("the picture has too many pixels"))
		return nil, false
	}
	return data, true
}

// coverTargets are the tracks a cover request is about: the tracks named by ids,
// or all the tracks of the artist or album with the row key in group.
func (s *Server) coverTargets(w http.ResponseWriter, r *http.Request) (ids []string, group string, ok bool) {
	q := r.URL.Query()
	if group = q.Get("group"); group != "" {
		return nil, group, true
	}
	if v := q.Get("ids"); v != "" {
		return strings.Split(v, ","), "", true
	}
	fail(w, http.StatusBadRequest, errors.New("the request says no track"))
	return nil, "", false
}

func (s *Server) putCover(w http.ResponseWriter, r *http.Request) {
	ids, group, ok := s.coverTargets(w, r)
	if !ok {
		return
	}
	data, ok := picture(w, r)
	if !ok {
		return
	}
	s.setCover(w, r, ids, group, data)
}

func (s *Server) deleteCover(w http.ResponseWriter, r *http.Request) {
	ids, group, ok := s.coverTargets(w, r)
	if !ok {
		return
	}
	s.setCover(w, r, ids, group, nil)
}

func (s *Server) setCover(w http.ResponseWriter, r *http.Request, ids []string, group string, data []byte) {
	if group != "" {
		res, err := s.ed.SetGroupCover(group, data)
		s.answer(w, res, err)
		return
	}
	// a picture pasted on tracks: that says where it came from, and spares those that have it
	if q := r.URL.Query(); q.Get("paste") == "1" && data != nil {
		res, err := s.ed.PastePicture(ids, data, q.Get("name"))
		s.answer(w, res, err)
		return
	}
	res, err := s.ed.SetCover(ids, data)
	s.answer(w, res, err)
}

// ---- groups and bulk

func (s *Server) setGroupField(w http.ResponseWriter, r *http.Request) {
	var in struct{ Key, Field, Text string }
	if !read(w, r, &in) {
		return
	}
	res, err := s.ed.SetGroupField(in.Key, in.Field, in.Text)
	s.answer(w, res, err)
}

func (s *Server) groupHint(w http.ResponseWriter, r *http.Request) {
	var in struct{ Key, Field, Text string }
	if !read(w, r, &in) {
		return
	}
	snap, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, HintView{Hint: browse.GroupHint(snap, in.Key, in.Field, in.Text)})
}

type bulkRequest struct {
	IDs    []string          `json:"ids"`
	Values browse.BulkValues `json:"values"`
}

func (s *Server) bulkPreview(w http.ResponseWriter, r *http.Request) {
	var in bulkRequest
	if !read(w, r, &in) {
		return
	}
	snap, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, browse.PreviewBulk(snap, in.IDs, in.Values))
}

func (s *Server) bulk(w http.ResponseWriter, r *http.Request) {
	var in bulkRequest
	if !read(w, r, &in) {
		return
	}
	res, err := s.ed.Bulk(in.IDs, in.Values)
	s.answer(w, res, err)
}

// ---- tracks and playlists

func (s *Server) remove(w http.ResponseWriter, r *http.Request) {
	var in struct{ IDs []string }
	if !read(w, r, &in) {
		return
	}
	res, err := s.ed.Remove(in.IDs)
	s.answer(w, res, err)
}

func (s *Server) renumber(w http.ResponseWriter, r *http.Request) {
	var in struct{ Key string }
	if !read(w, r, &in) {
		return
	}
	res, err := s.ed.Renumber(in.Key)
	s.answer(w, res, err)
}

// move moves a track one place in its album, or in the playlist if one is named.
func (s *Server) move(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID       string
		Dir      int
		Playlist string
	}
	if !read(w, r, &in) {
		return
	}
	if in.Dir != 1 && in.Dir != -1 {
		fail(w, http.StatusBadRequest, errors.New("dir must be 1 or -1"))
		return
	}
	var (
		res browse.Result
		err error
	)
	if in.Playlist != "" {
		res, err = s.ed.MoveInPlaylist(in.Playlist, in.ID, in.Dir)
	} else {
		res, err = s.ed.MoveInAlbum(in.ID, in.Dir)
	}
	s.answer(w, res, err)
}

type playlistRequest struct {
	Name string   `json:"name"`
	IDs  []string `json:"ids"`
}

func (s *Server) playlistNew(w http.ResponseWriter, r *http.Request) {
	var in playlistRequest
	if !read(w, r, &in) {
		return
	}
	res, err := s.ed.PlaylistNew(in.Name, in.IDs)
	s.answer(w, res, err)
}

func (s *Server) playlistAdd(w http.ResponseWriter, r *http.Request) {
	var in playlistRequest
	if !read(w, r, &in) {
		return
	}
	res, err := s.ed.PlaylistAdd(in.Name, in.IDs)
	s.answer(w, res, err)
}

func (s *Server) playlistTakeOut(w http.ResponseWriter, r *http.Request) {
	var in playlistRequest
	if !read(w, r, &in) {
		return
	}
	res, err := s.ed.PlaylistTakeOut(in.Name, in.IDs)
	s.answer(w, res, err)
}

func (s *Server) playlistDelete(w http.ResponseWriter, r *http.Request) {
	var in playlistRequest
	if !read(w, r, &in) {
		return
	}
	res, err := s.ed.PlaylistDelete(in.Name)
	s.answer(w, res, err)
}

// ---- copy, paste, undo

func (s *Server) copy(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID    string
		Field string   // c in the fields: what the field stands for
		Attrs []string // c in the tree: what was ticked
	}
	if !read(w, r, &in) {
		return
	}
	var (
		res browse.Result
		err error
	)
	if in.Field != "" {
		res, err = s.ed.CopyField(in.ID, in.Field)
	} else {
		res, err = s.ed.Copy(in.ID, in.Attrs)
	}
	s.answer(w, res, err)
}

// paste puts what was copied on the tracks named by ids, or on all the tracks
// of the artist or album with the row key in group.
func (s *Server) paste(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDs   []string
		Group string
	}
	if !read(w, r, &in) {
		return
	}
	ids := in.IDs
	if in.Group != "" {
		snap, err := s.snapshot()
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		var ok bool
		if ids, ok = browse.GroupIDs(snap, in.Group); !ok {
			fail(w, http.StatusUnprocessableEntity, errors.New("Select a track, an album or an artist to paste on"))
			return
		}
	}
	res, err := s.ed.Paste(ids)
	s.answer(w, res, err)
}

func (s *Server) undo(w http.ResponseWriter, r *http.Request) {
	res, err := s.ed.Undo()
	s.answer(w, res, err)
}

func (s *Server) redo(w http.ResponseWriter, r *http.Request) {
	res, err := s.ed.Redo()
	s.answer(w, res, err)
}

// ---- audio

// previewAudio serves the audio around one of a track's cuts as it will sound
// once synced: with edge=start the first window of what is kept, with
// edge=end the last. The file is made for the request and removed after it.
func (s *Server) previewAudio(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	edge := app.EdgeStart
	window := 8 * time.Second
	if field := q.Get("field"); field != "" {
		// the end and the length that suit checking this field by ear
		snap, err := s.snapshot()
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		t, found := snap.Track(r.PathValue("id"))
		if !found {
			fail(w, http.StatusNotFound, errNotFound)
			return
		}
		var ok bool
		if edge, window, ok = browse.Hearable(t, field); !ok {
			fail(w, http.StatusBadRequest, errors.New("there is nothing to hear in that field"))
			return
		}
	} else {
		switch q.Get("edge") {
		case "", "start":
		case "end":
			edge = app.EdgeEnd
		default:
			fail(w, http.StatusBadRequest, errors.New("edge must be start or end"))
			return
		}
		if v := q.Get("window"); v != "" {
			secs, err := strconv.ParseFloat(v, 64)
			if err != nil || secs <= 0 || secs > 6*3600 {
				fail(w, http.StatusBadRequest, errors.New("window must be a number of seconds"))
				return
			}
			window = time.Duration(secs * float64(time.Second))
		}
	}
	path, cleanup, err := s.app.PreviewTrim(r.PathValue("id"), edge, window)
	if err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	defer cleanup()
	f, err := os.Open(path)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "", info.ModTime(), f)
}
