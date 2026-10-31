// Package web serves the library to a browser: a JSON API over the app layer,
// a stream of events that tells the page when the library changed, and the
// page itself, embedded in the program. It listens on the loopback address
// only and treats every request as one that a stranger's web page could be
// making (see guard).
package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/browse"
	"music-manager/internal/device"
	"music-manager/internal/version"
)

//go:embed static
var staticFiles embed.FS

// Server is the web interface to one library.
type Server struct {
	app        *app.App
	source     app.MetadataSource
	downloader app.Downloader
	wolfgang   bool
	profiles   *device.Registry
	frontend   fs.FS

	hub  *hub
	jobs *jobs
	mux  *http.ServeMux

	plans   plans
	scanned scan
	mounts  func() []string // the drives to offer; browse.DetectMounts unless a test says otherwise

	infoMu    sync.Mutex
	coverInfo map[string]string
	ed        *browse.Editor
}

// Option adds a capability to the server.
type Option func(*Server)

// WithSource lets the server look tracks up in a music database.
func WithSource(s app.MetadataSource) Option { return func(v *Server) { v.source = s } }

// WithDownloader lets the server add tracks by downloading them.
func WithDownloader(d app.Downloader) Option { return func(v *Server) { v.downloader = d } }

// WithWolfgang shows what a sync for the Wolfgang WMP01 writes.
func WithWolfgang() Option { return func(v *Server) { v.wolfgang = true } }

func WithProfiles(r *device.Registry) Option { return func(v *Server) { v.profiles = r } }

// New returns the server for a library.
func New(a *app.App, opts ...Option) *Server {
	s := &Server{app: a, hub: newHub(), mux: http.NewServeMux(), coverInfo: map[string]string{}, mounts: browse.DetectMounts}
	s.jobs = newJobs(s.hub)
	for _, o := range opts {
		o(s)
	}
	if s.profiles == nil {
		s.profiles = a.Profiles()
	}
	s.ed = browse.NewEditor(a, browse.WithSource(s.source), browse.WithDownloader(s.downloader))
	s.routes()
	return s
}

// Handler is the whole server as a handler, guard included.
func (s *Server) Handler() http.Handler { return guard(s.mux) }

func (s *Server) routes() {
	if s.frontend != nil {
		s.mux.Handle("GET /", frontendHandler(s.frontend))
	} else {
		static, err := fs.Sub(staticFiles, "static")
		if err != nil {
			panic(err)
		}
		s.mux.Handle("GET /", noCache(http.FileServerFS(static)))
	}

	s.mux.HandleFunc("GET /api/info", s.info)
	s.mux.HandleFunc("GET /api/profiles", s.listProfiles)
	s.mux.HandleFunc("GET /api/doctor", s.doctor)
	s.mux.HandleFunc("GET /api/library", s.library)
	s.mux.HandleFunc("GET /api/tracks/{id}", s.track)
	s.mux.HandleFunc("GET /api/tracks/{id}/cover", s.cover)
	s.mux.HandleFunc("GET /api/peek", s.peek)
	s.mux.HandleFunc("GET /api/events", s.hub.serve)
	s.editRoutes()
	s.identifyRoutes()
	s.addRoutes()
	s.syncRoutes()
}

// Serve answers requests on ln until ctx is done, and watches the library for
// changes made by anything else.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	s.jobs.useContext(ctx)
	go s.watch(ctx)
	go s.scanUnsynced()
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ln) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		s.hub.close()
		shut, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shut)
		<-done
		s.jobs.wait(5 * time.Second)
		return nil
	}
}

// Listen opens the address the page will be served on. An address that is
// taken is replaced by a free port on the same host if fallback is set.
func Listen(addr string, fallback bool) (net.Listener, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("address %q: %w", addr, err)
	}
	if !loopback(host) {
		return nil, fmt.Errorf("address %q: the interface may only listen on this computer (127.0.0.1, localhost or ::1)", addr)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil && fallback {
		ln, err = net.Listen("tcp", net.JoinHostPort(host, "0"))
	}
	return ln, err
}

func loopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// URL is the address to open in a browser for a listener.
func URL(ln net.Listener) string {
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return "http://" + host + ":" + port + "/"
}

type InfoView struct {
	CanAdd   bool   `json:"canAdd"`
	Lookup   bool   `json:"lookup"`
	Root     string `json:"root"`
	Version  string `json:"version"`
	Wolfgang bool   `json:"wolfgang"`
}

type ErrorView struct {
	Error string `json:"error"`
}

// info is what does not change while the program runs.
func (s *Server) info(w http.ResponseWriter, r *http.Request) {
	root := s.root()
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rel, err := filepath.Rel(home, root); err == nil && !strings.HasPrefix(rel, "..") {
			root = filepath.Join("~", rel)
		}
	}
	writeJSON(w, InfoView{
		CanAdd:   s.downloader != nil,
		Lookup:   s.source != nil,
		Root:     root,
		Version:  version.String(),
		Wolfgang: s.wolfgang,
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

// fail answers with an error the page shows to the person using it.
func fail(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorView{Error: capitalize(err.Error())})
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

var errNotFound = errors.New("not found")

func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		h.ServeHTTP(w, r)
	})
}

// root is the library folder as a full path, wherever mm was started from.
func (s *Server) root() string {
	root := s.app.Root()
	if abs, err := filepath.Abs(root); err == nil {
		return abs
	}
	return root
}
