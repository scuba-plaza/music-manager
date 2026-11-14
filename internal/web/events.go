package web

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// hub sends events to every page that has the stream open.
type hub struct {
	mu      sync.Mutex
	clients map[chan string]struct{}
	closed  chan struct{}
	once    sync.Once
}

func newHub() *hub {
	return &hub{clients: map[chan string]struct{}{}, closed: make(chan struct{})}
}

func (h *hub) close() { h.once.Do(func() { close(h.closed) }) }

// publish sends an event with the given name and data (one line) to all pages.
func (h *hub) publish(event, data string) {
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", event, data)
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c <- msg:
		default: // a page that does not keep up misses it, and asks again when it looks
		}
	}
}

func (h *hub) serve(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	c := make(chan string, 16)
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
	}()
	fmt.Fprint(w, "retry: 2000\n\nevent: hello\ndata: {}\n\n")
	fl.Flush()
	beat := time.NewTicker(20 * time.Second)
	defer beat.Stop()
	for {
		select {
		case msg := <-c:
			fmt.Fprint(w, msg)
		case <-beat.C:
			fmt.Fprint(w, ": keep-alive\n\n")
		case <-r.Context().Done():
			return
		case <-h.closed:
			return
		}
		fl.Flush()
	}
}

// stamp is a short text that changes whenever the records, or the note of what
// was last synced, change, however that happens: it is how changes made by
// another program are noticed.
func (s *Server) stamp() string {
	var out string
	for _, p := range []string{
		filepath.Join(s.app.Root(), "library.json"),
		filepath.Join(s.app.Root(), ".mm", "synced.json"),
	} {
		if info, err := os.Stat(p); err == nil {
			out += fmt.Sprintf("%d:%d;", info.ModTime().UnixNano(), info.Size())
		} else {
			out += "-;"
		}
	}
	return out
}

// watch tells the pages when the library changes, until ctx is done.
func (s *Server) watch(ctx context.Context) {
	last := s.stamp()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if now := s.stamp(); now != last {
				last = now
				s.hub.publish("library", fmt.Sprintf("%q", now))
			}
		}
	}
}

type HelloEvent struct{}

type ServerEvents struct {
	Hello   HelloEvent `json:"hello"`
	Job     JobView    `json:"job"`
	Library string     `json:"library"`
}
