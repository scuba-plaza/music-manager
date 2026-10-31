package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/browse"
	"music-manager/internal/device"
)

// Writing the library to a folder or a drive takes two steps that the person
// reviews between: a plan, which reads everything and writes nothing, and the
// apply, which carries out the plan that was looked at. Both are jobs of kind
// "sync", so only one of them goes on at a time. The plan waits here, in
// memory, until it is applied, replaced or dropped.

// planTTL is how long a plan may wait for the person to look at it.
const planTTL = 30 * time.Minute

// SyncData is what a sync job reports: how far it has got, then how it ended.
type SyncData struct {
	Phase   string           `json:"phase"` // plan or apply
	Done    int              `json:"done,omitempty"`
	Total   int              `json:"total,omitempty"`
	Plan    *browse.PlanView `json:"plan,omitempty"`
	Where   string           `json:"where,omitempty"`
	Summary string           `json:"summary,omitempty"`
}

type syncEntry struct {
	id      string
	where   string
	library bool
	plan    *app.SyncPlan // as made: stray files are only reported
	current *app.SyncPlan // with the choice about strays applied
	prune   bool
	made    time.Time
}

// plans holds the one plan being looked at.
type plans struct {
	mu sync.Mutex
	e  *syncEntry
}

func (p *plans) put(e *syncEntry) {
	p.mu.Lock()
	p.e = e
	p.mu.Unlock()
}

func (p *plans) get(id string) (*syncEntry, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.e == nil || p.e.id != id || time.Since(p.e.made) > planTTL {
		return nil, false
	}
	return p.e, true
}

func (p *plans) drop(id string) {
	p.mu.Lock()
	if p.e != nil && p.e.id == id {
		p.e = nil
	}
	p.mu.Unlock()
}

var errPlanGone = errors.New("that plan is gone: prepare the sync again")

func (s *Server) syncRoutes() {
	s.mux.HandleFunc("GET /api/sync/targets", s.syncTargets)
	s.mux.HandleFunc("POST /api/sync/plan", s.syncPlan)
	s.mux.HandleFunc("POST /api/sync/{plan}/prune", s.syncPrune)
	s.mux.HandleFunc("POST /api/sync/{plan}/apply", s.syncApply)
	s.mux.HandleFunc("DELETE /api/sync/{plan}", s.syncDrop)
}

type SyncTargetsView struct {
	Targets  []browse.TargetView `json:"targets"`
	Wolfgang bool                `json:"wolfgang"`
}

func (s *Server) syncTargets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, SyncTargetsView{
		Targets:  browse.Targets(s.root(), s.mounts()),
		Wolfgang: s.wolfgang,
	})
}

// target works out where the person means: the library folder, or a folder
// that exists (or can be made in one that does). It refuses folders that hold
// the library or lie inside it, which no sync could do well.
func (s *Server) target(library bool, folder string) (path, where string, isLibrary bool, err error) {
	if library {
		return "", "Library folder", true, nil
	}
	p := browse.CleanPath(folder)
	if p == "" {
		return "", "", false, errors.New("type the folder to write into")
	}
	if !filepath.IsAbs(p) {
		return "", "", false, errors.New("write the whole path of the folder, starting with / or ~")
	}
	p = filepath.Clean(p)
	root := s.root()
	if sameFolder(p, root) {
		return "", "Library folder", true, nil
	}
	if inside(p, root) {
		return "", "", false, errors.New("that folder is inside the library folder: pick another one, or the library folder itself")
	}
	if inside(root, p) {
		return "", "", false, errors.New("that folder holds the library: pick a folder of its own")
	}
	switch info, err := os.Stat(p); {
	case err == nil && !info.IsDir():
		return "", "", false, fmt.Errorf("%s is a file, not a folder", browse.Tilde(p))
	case err != nil:
		if parent, perr := os.Stat(filepath.Dir(p)); perr != nil || !parent.IsDir() {
			return "", "", false, fmt.Errorf("%s is not there, and neither is the folder it would be made in", browse.Tilde(p))
		}
	}
	return p, browse.Tilde(p), false, nil
}

func sameFolder(a, b string) bool {
	x, err1 := filepath.Abs(a)
	y, err2 := filepath.Abs(b)
	return err1 == nil && err2 == nil && filepath.Clean(x) == filepath.Clean(y)
}

// inside says whether p is a folder below dir.
func inside(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (s *Server) syncPlan(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Library  bool
		Folder   string
		Wolfgang bool
		Profile  string
	}
	if !read(w, r, &in) {
		return
	}
	target, where, isLibrary, err := s.target(in.Library, in.Folder)
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, err)
		return
	}
	dev, err := s.syncDevice(isLibrary, in.Profile, in.Wolfgang)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if dev.Name() != device.Generic.Name() {
		where += " · " + dev.Profile().Label
	}
	v, err := s.jobs.start("sync", func(ctx context.Context, report func(any)) (any, error) {
		type made struct {
			plan *app.SyncPlan
			err  error
		}
		out := make(chan made, 1)
		go func() { // planning cannot be stopped: a plan nobody waits for any more is dropped
			plan, err := s.app.PlanSync(app.SyncOptions{Target: target, Device: dev, OnProgress: func(done, total int) {
				report(SyncData{Phase: "plan", Done: done, Total: total, Where: where})
			}})
			out <- made{plan, err}
		}()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case m := <-out:
			if m.err != nil {
				return nil, m.err
			}
			id, err := newID()
			if err != nil {
				return nil, err
			}
			e := &syncEntry{id: id, where: where, library: isLibrary, plan: m.plan, current: m.plan, made: time.Now()}
			s.plans.put(e)
			if isLibrary && len(m.plan.Ops) == 0 { // the folder is up to date: remember it
				_ = s.app.RecordSynced()
			}
			view, err := s.planView(e)
			if err != nil {
				return nil, err
			}
			return SyncData{Phase: "plan", Where: where, Plan: &view}, nil
		}
	})
	if errors.Is(err, errBusy) {
		fail(w, http.StatusConflict, errors.New("a sync is going on already"))
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, v)
}

func (s *Server) syncDevice(isLibrary bool, profile string, wolfgang bool) (device.Target, error) {
	switch {
	case isLibrary:
		return device.Generic, nil
	case profile != "":
		return s.profile(profile)
	case wolfgang:
		return s.profile("wolfgang-wmp01")
	}
	return device.Generic, nil
}

func newID() (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func (s *Server) planView(e *syncEntry) (browse.PlanView, error) {
	snap, err := s.snapshot()
	if err != nil {
		return browse.PlanView{}, err
	}
	return browse.Plan(snap, e.id, e.where, e.prune, e.current.Ops, e.current.Warnings), nil
}

func (s *Server) syncPrune(w http.ResponseWriter, r *http.Request) {
	var in struct{ Prune bool }
	if !read(w, r, &in) {
		return
	}
	e, ok := s.plans.get(r.PathValue("plan"))
	if !ok {
		fail(w, http.StatusGone, errPlanGone)
		return
	}
	s.plans.mu.Lock()
	e.prune = in.Prune
	e.current = e.plan.WithPrune(in.Prune)
	s.plans.mu.Unlock()
	view, err := s.planView(e)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, view)
}

func (s *Server) syncApply(w http.ResponseWriter, r *http.Request) {
	var in struct{ Confirm string }
	if !read(w, r, &in) {
		return
	}
	e, ok := s.plans.get(r.PathValue("plan"))
	if !ok {
		fail(w, http.StatusGone, errPlanGone)
		return
	}
	s.plans.mu.Lock()
	plan, where, isLibrary := e.current, e.where, e.library
	s.plans.mu.Unlock()
	counts := browse.CountOps(plan.Ops)
	switch {
	case len(plan.Ops) == 0:
		fail(w, http.StatusUnprocessableEntity, errors.New("there is nothing to write"))
		return
	case counts.Remove > 0 && strings.TrimSpace(in.Confirm) != "delete":
		fail(w, http.StatusUnprocessableEntity, fmt.Errorf("type delete to remove %s", plural(counts.Remove, "file")))
		return
	}
	v, err := s.jobs.start("sync", func(_ context.Context, report func(any)) (any, error) {
		err := plan.ApplyWith(func(done, total int) {
			report(SyncData{Phase: "apply", Done: done, Total: total, Where: where})
		})
		if err != nil {
			return nil, err
		}
		s.plans.drop(e.id)
		if isLibrary || plan.Collected > 0 { // what was deleted cannot be brought back
			s.ed.Forget()
		}
		return SyncData{Phase: "apply", Where: where, Summary: browse.Summary(counts, plan.Collected)}, nil
	})
	if errors.Is(err, errBusy) {
		fail(w, http.StatusConflict, errors.New("a sync is going on already"))
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, v)
}

func (s *Server) syncDrop(w http.ResponseWriter, r *http.Request) {
	s.plans.drop(r.PathValue("plan"))
	w.WriteHeader(http.StatusNoContent)
}

// ---- the count in the header

// scan is the count of tracks the library folder lags behind by, for a library
// that has never been synced. Normally the count is a comparison of records and
// nothing more; only the first time, when there is nothing to compare with, the
// folder itself is looked at, once, in the background.
type scan struct {
	mu sync.Mutex
	n  int
}

// unsynced is the header's count.
func (s *Server) unsynced() (n int, known bool) {
	n, known, err := s.app.Unsynced()
	if err != nil {
		return 0, false
	}
	if known {
		return n, true
	}
	s.scanned.mu.Lock()
	defer s.scanned.mu.Unlock()
	return s.scanned.n, false
}

// scanUnsynced looks at the library folder once, if nothing says how far behind
// it is, and tells the pages. A folder that is up to date is remembered as such.
func (s *Server) scanUnsynced() {
	if _, known, err := s.app.Unsynced(); err != nil || known {
		return
	}
	plan, err := s.app.PlanSync(app.SyncOptions{})
	if err != nil {
		return
	}
	if writes := browse.CountOps(plan.Ops).Writes(); writes > 0 {
		s.scanned.mu.Lock()
		s.scanned.n = writes
		s.scanned.mu.Unlock()
		s.hub.publish("library", fmt.Sprintf("%q", s.stamp()+"scanned"))
		return
	}
	_ = s.app.RecordSynced()
}
