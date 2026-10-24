package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"
)

// A job is something that takes long enough for the page to show how far it
// has got: identifying every track, downloading one, writing the player. It
// runs on its own context, not the request's, so that closing the page does not
// stop it; the page asks for it to stop. Its state goes to every page as a
// "job" event, and can be asked for again at /api/jobs/{id}.

// job states
const (
	jobRunning = "running"
	jobDone    = "done"
	jobStopped = "stopped"
	jobFailed  = "failed"
)

// JobView is a job as the page sees it.
type JobView struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	State string `json:"state"`
	// Data is what the kind of job reports: its progress while it runs, its outcome after.
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

type job struct {
	view   JobView
	cancel context.CancelFunc
	sent   time.Time
}

type jobs struct {
	hub *hub

	mu   sync.Mutex
	base context.Context
	byID map[string]*job
	wg   sync.WaitGroup
}

func newJobs(h *hub) *jobs {
	return &jobs{hub: h, base: context.Background(), byID: map[string]*job{}}
}

// useContext makes every job after this one end when ctx does.
func (j *jobs) useContext(ctx context.Context) {
	j.mu.Lock()
	j.base = ctx
	j.mu.Unlock()
}

// wait gives running jobs a little time to wind up.
func (j *jobs) wait(d time.Duration) {
	done := make(chan struct{})
	go func() { j.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
	}
}

// maxFinished is how many finished jobs are remembered.
const maxFinished = 20

var errBusy = errors.New("that is going on already")

// start runs work as a job of the given kind; only one of a kind runs at a time.
// work reports how far it has got with report, and returns what it came to.
func (j *jobs) start(kind string, work func(ctx context.Context, report func(data any)) (any, error)) (JobView, error) {
	j.mu.Lock()
	for _, o := range j.byID {
		if o.view.Kind == kind && o.view.State == jobRunning {
			j.mu.Unlock()
			return JobView{}, errBusy
		}
	}
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err != nil {
		j.mu.Unlock()
		return JobView{}, err
	}
	ctx, cancel := context.WithCancel(j.base)
	jb := &job{view: JobView{ID: hex.EncodeToString(raw[:]), Kind: kind, State: jobRunning}, cancel: cancel}
	j.byID[jb.view.ID] = jb
	j.forgetOld()
	started := jb.view
	j.wg.Add(1)
	j.mu.Unlock()

	go func() {
		defer j.wg.Done()
		defer cancel()
		report := func(data any) {
			j.mu.Lock()
			if jb.view.State != jobRunning { // work that goes on after its job ended
				j.mu.Unlock()
				return
			}
			jb.view.Data = data
			if time.Since(jb.sent) < 100*time.Millisecond { // a fast job is not worth every step
				j.mu.Unlock()
				return
			}
			jb.sent = time.Now()
			view := jb.view
			j.mu.Unlock()
			j.publish(view)
		}
		final, err := work(ctx, report)
		j.mu.Lock()
		jb.view.Data = final
		switch {
		case ctx.Err() != nil:
			jb.view.State = jobStopped
		case err != nil:
			jb.view.State, jb.view.Error = jobFailed, capitalize(err.Error())
		default:
			jb.view.State = jobDone
		}
		view := jb.view
		j.mu.Unlock()
		j.publish(view)
	}()
	return started, nil
}

// forgetOld drops the oldest finished jobs; the caller holds the lock.
func (j *jobs) forgetOld() {
	var finished []string
	for id, o := range j.byID {
		if o.view.State != jobRunning {
			finished = append(finished, id)
		}
	}
	for len(finished) > maxFinished {
		delete(j.byID, finished[0])
		finished = finished[1:]
	}
}

func (j *jobs) publish(v JobView) {
	raw, err := json.Marshal(v)
	if err != nil {
		return
	}
	j.hub.publish("job", string(raw))
}

func (j *jobs) get(id string) (JobView, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	o, ok := j.byID[id]
	if !ok {
		return JobView{}, false
	}
	return o.view, true
}

// running lists the jobs that are going on, so that a page opened meanwhile can show them.
func (j *jobs) running() []JobView {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := []JobView{}
	for _, o := range j.byID {
		if o.view.State == jobRunning {
			out = append(out, o.view)
		}
	}
	return out
}

func (j *jobs) stop(id string) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	o, ok := j.byID[id]
	if ok && o.view.State == jobRunning {
		o.cancel()
	}
	return ok
}

func (s *Server) jobRoutes() {
	s.mux.HandleFunc("GET /api/jobs", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.jobs.running()) })
	s.mux.HandleFunc("GET /api/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, ok := s.jobs.get(r.PathValue("id"))
		if !ok {
			fail(w, http.StatusNotFound, errNotFound)
			return
		}
		writeJSON(w, v)
	})
	s.mux.HandleFunc("DELETE /api/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !s.jobs.stop(r.PathValue("id")) {
			fail(w, http.StatusNotFound, errNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
