package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"music-manager/internal/browse"
)

// downloadTimeout bounds a download, in case it never ends.
const downloadTimeout = 10 * time.Minute

// maxUpload is the most audio one request may bring.
const maxUpload = 2 << 30

func (s *Server) addRoutes() {
	s.mux.HandleFunc("POST /api/add/fetch", s.addFetch)
	s.mux.HandleFunc("POST /api/add/path", s.addPath)
	s.mux.HandleFunc("POST /api/add/finish", s.addFinish)
	s.mux.HandleFunc("DELETE /api/add/{draft}", s.addDiscard)
	s.mux.HandleFunc("POST /api/ingest", s.ingest)
}

// addFetch downloads a link, as a job; the draft or the duplicate it comes to
// is the job's outcome.
func (s *Server) addFetch(w http.ResponseWriter, r *http.Request) {
	var in struct{ URL string }
	if !read(w, r, &in) {
		return
	}
	if !s.ed.CanAdd() {
		fail(w, http.StatusUnprocessableEntity, errors.New("adding from YouTube is not available: yt-dlp is not set up"))
		return
	}
	if browse.CleanLink(in.URL) == "" {
		fail(w, http.StatusUnprocessableEntity, errors.New("paste a link first"))
		return
	}
	v, err := s.jobs.start("add", func(ctx context.Context, _ func(any)) (any, error) {
		ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
		defer cancel()
		return s.ed.Fetch(ctx, in.URL)
	})
	if errors.Is(err, errBusy) {
		fail(w, http.StatusConflict, errors.New("a download is going on already"))
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, v)
}

func (s *Server) addPath(w http.ResponseWriter, r *http.Request) {
	var in struct{ Form browse.AddForm }
	if !read(w, r, &in) {
		return
	}
	writeJSON(w, map[string]string{"path": browse.AddPath(in.Form)})
}

func (s *Server) addFinish(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Draft string
		Form  browse.AddForm
	}
	if !read(w, r, &in) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	res, err := s.ed.Finish(ctx, in.Draft, in.Form)
	s.answer(w, res, err)
}

func (s *Server) addDiscard(w http.ResponseWriter, r *http.Request) {
	s.ed.Discard(r.PathValue("draft"))
	w.WriteHeader(http.StatusNoContent)
}

// ingest adds the MP3 files the request brings, as unidentified tracks.
func (s *Server) ingest(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	mr, err := r.MultipartReader()
	if err != nil {
		fail(w, http.StatusBadRequest, errors.New("the files could not be read"))
		return
	}
	if err := os.MkdirAll(filepath.Join(s.app.Root(), ".mm"), 0o755); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	dir, err := os.MkdirTemp(filepath.Join(s.app.Root(), ".mm"), "upload-")
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	defer os.RemoveAll(dir)

	var skipped []string
	saved := 0
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			status := http.StatusBadRequest
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				status = http.StatusRequestEntityTooLarge
			}
			fail(w, status, errors.New("the files could not be read, or are too large"))
			return
		}
		if part.FileName() == "" {
			continue
		}
		base := cleanName(part.FileName())
		if !strings.EqualFold(filepath.Ext(base), ".mp3") {
			skipped = append(skipped, base+": only MP3 files can be added")
			continue
		}
		if err := saveUpload(part, filepath.Join(dir, fmt.Sprintf("%03d-%s", saved, base))); err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		saved++
	}
	if saved == 0 {
		msg := "no MP3 file came with the request"
		if len(skipped) > 0 {
			msg = strings.Join(skipped, "; ")
		}
		fail(w, http.StatusUnprocessableEntity, errors.New(msg))
		return
	}
	rep, err := s.app.Ingest([]string{dir})
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	s.ed.Ingested(rep.Added)
	for _, f := range rep.Failed {
		skipped = append(skipped, fmt.Sprintf("%s: %v", filepath.Base(f.Path), f.Err))
	}
	msg := "Added " + plural(len(rep.Added), "track")
	if n := len(rep.Duplicates); n > 0 {
		msg += fmt.Sprintf(" · %d already there", n)
	}
	if n := len(skipped); n > 0 {
		msg += fmt.Sprintf(" · %d left out: %s", n, strings.Join(skipped, "; "))
	}
	res := browse.Result{Message: msg}
	if len(rep.Added) > 0 {
		res.Reveal = rep.Added[0].ID
		res.Message += " · u undoes it"
	} else {
		res.Same = true
	}
	s.answer(w, res, nil)
}

func saveUpload(part *multipart.Part, path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, part); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// cleanName is the name of an uploaded file, without any folder it came from.
func cleanName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	if name == "." || name == "/" || name == "" {
		return "file.mp3"
	}
	return name
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
