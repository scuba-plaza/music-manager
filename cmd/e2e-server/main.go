package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"music-manager/internal/device"
	"music-manager/internal/web"
	"music-manager/internal/web/webtest"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "e2e-server:", err)
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", "127.0.0.1:7979", "address to listen on (only this computer: 127.0.0.1, localhost or ::1)")
	dist := flag.String("dist", "internal/web/dist", "folder with the built front end")
	config := flag.String("config", "", "device profiles file (only the built-in profiles when empty)")
	flag.Parse()

	profiles := device.Builtin()
	if *config != "" {
		loaded, err := device.LoadRegistry(*config)
		if err != nil {
			return err
		}
		profiles = loaded
	}
	ln, err := web.Listen(*addr, false)
	if err != nil {
		return err
	}
	parent, err := os.MkdirTemp("", "mm-e2e-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(parent)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	servers := &seededServers{ctx: ctx, parent: parent, profiles: profiles, frontend: os.DirFS(*dist)}
	if err := servers.reset(); err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /__e2e/reset", servers.serveReset)
	mux.Handle("/", servers)
	srv := &http.Server{Handler: loopbackOnly(mux), ReadHeaderTimeout: 10 * time.Second}

	fmt.Fprintf(os.Stdout, "e2e-server is at %s\n", web.URL(ln))
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ln) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		servers.close()
		shut, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return srv.Shutdown(shut)
	}
}

type seededServers struct {
	ctx      context.Context
	parent   string
	profiles *device.Registry
	frontend fs.FS

	mu      sync.Mutex
	made    int
	current atomic.Pointer[instance]
}

type instance struct {
	proxy *httputil.ReverseProxy
	stop  func()
}

func (l *seededServers) reset() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.made++
	dir := filepath.Join(l.parent, strconv.Itoa(l.made))
	a, err := webtest.Seed(dir)
	if err != nil {
		return err
	}
	a.UseProfiles(l.profiles)
	server := web.New(a,
		web.WithProfiles(l.profiles),
		web.WithSource(webtest.FakeSource{}),
		web.WithDownloader(&webtest.FakeDownloader{}),
		web.WithFrontend(l.frontend),
	)
	ln, err := web.Listen("127.0.0.1:0", false)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(l.ctx)
	ended := make(chan struct{})
	go func() {
		defer close(ended)
		_ = server.Serve(ctx, ln)
	}()
	next := &instance{
		proxy: httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: ln.Addr().String()}),
		stop: func() {
			cancel()
			<-ended
			_ = os.RemoveAll(dir)
		},
	}
	if previous := l.current.Swap(next); previous != nil {
		previous.stop()
	}
	return nil
}

func (l *seededServers) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if current := l.current.Swap(nil); current != nil {
		current.stop()
	}
}

func (l *seededServers) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	current := l.current.Load()
	if current == nil {
		http.Error(w, "the server is stopping", http.StatusServiceUnavailable)
		return
	}
	current.proxy.ServeHTTP(w, r)
}

func (l *seededServers) serveReset(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-MM") == "" {
		http.Error(w, "the X-MM header is missing", http.StatusForbidden)
		return
	}
	if err := l.reset(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func loopbackOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(r.Host); err == nil {
			host = h
		}
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
		if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			http.Error(w, "this address is not allowed", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
