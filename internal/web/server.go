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

	hub *hub
	mux *http.ServeMux

	infoMu    sync.Mutex
	coverInfo map[string]string
}

// Option adds a capability to the server.
type Option func(*Server)

// WithSource lets the server look tracks up in a music database.
func WithSource(s app.MetadataSource) Option { return func(v *Server) { v.source = s } }

// WithDownloader lets the server add tracks by downloading them.
func WithDownloader(d app.Downloader) Option { return func(v *Server) { v.downloader = d } }

// WithWolfgang shows what a sync for the Wolfgang WMP01 writes.
func WithWolfgang() Option { return func(v *Server) { v.wolfgang = true } }

// New returns the server for a library.
func New(a *app.App, opts ...Option) *Server {
	s := &Server{app: a, hub: newHub(), mux: http.NewServeMux(), coverInfo: map[string]string{}}
	for _, o := range opts {
		o(s)
	}
	s.routes()
	return s
}

// Handler is the whole server as a handler, guard included.
func (s *Server) Handler() http.Handler { return guard(s.mux) }

func (s *Server) routes() {
	static, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	files := http.FileServerFS(static)
	s.mux.Handle("GET /", noCache(files))

	s.mux.HandleFunc("GET /api/info", s.info)
	s.mux.HandleFunc("GET /api/library", s.library)
	s.mux.HandleFunc("GET /api/tracks/{id}", s.track)
	s.mux.HandleFunc("GET /api/tracks/{id}/cover", s.cover)
	s.mux.HandleFunc("GET /api/peek", s.peek)
	s.mux.HandleFunc("GET /api/events", s.hub.serve)
}

// Serve answers requests on ln until ctx is done, and watches the library for
// changes made by anything else.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go s.watch(ctx)
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

// info is what does not change while the program runs.
func (s *Server) info(w http.ResponseWriter, r *http.Request) {
	root := s.app.Root()
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rel, err := filepath.Rel(home, root); err == nil && !strings.HasPrefix(rel, "..") {
			root = filepath.Join("~", rel)
		}
	}
	writeJSON(w, map[string]any{
		"root": root, "version": version.String(), "wolfgang": s.wolfgang,
		"canAdd": s.downloader != nil, "lookup": s.source != nil,
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
	_ = json.NewEncoder(w).Encode(map[string]string{"error": capitalize(err.Error())})
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
