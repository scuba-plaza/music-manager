package web

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/browse"
)

// syncServer is a server that offers the given drives, not the ones of the machine.
func syncServer(t *testing.T, a *app.App, drives ...string) (*Server, *httptest.Server) {
	t.Helper()
	s := New(a)
	s.mounts = func() []string { return drives }
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return s, ts
}

// planned asks for a plan and waits for it.
func planned(t *testing.T, base string, body obj) (JobView, SyncData) {
	t.Helper()
	var v JobView
	if code := raw(t, base, "POST", "/api/sync/plan", body, &v); code != 200 || v.Kind != "sync" {
		t.Fatalf("%d %+v", code, v)
	}
	done := waitJob(t, base, v.ID)
	return done, syncData(t, done)
}

func syncData(t *testing.T, v JobView) SyncData {
	t.Helper()
	var d SyncData
	raw, _ := json.Marshal(v.Data)
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
	return d
}

func applied(t *testing.T, base, plan string, body obj) (int, JobView, reply) {
	t.Helper()
	var v JobView
	var r reply
	code := raw(t, base, "POST", "/api/sync/"+plan+"/apply", body, &v)
	if code != 200 {
		raw(t, base, "POST", "/api/sync/"+plan+"/apply", body, &r)
		return code, v, r
	}
	return code, waitJob(t, base, v.ID), r
}

func TestTheTargetsAreTheLibraryFolderAndTheDrives(t *testing.T) {
	a, _ := seed(t, sample...)
	_, ts := syncServer(t, a, "/run/media/tau/PLAYER")
	var out struct {
		Targets  []browse.TargetView
		Wolfgang bool
	}
	if code := getJSON(t, ts.URL+"/api/sync/targets", &out); code != 200 || len(out.Targets) != 2 ||
		out.Targets[0].Kind != "library" || out.Targets[1].Path != "/run/media/tau/PLAYER" || out.Wolfgang {
		t.Errorf("%d %+v", code, out)
	}
}

func TestASyncToAnotherFolderIsPlannedThenWritten(t *testing.T) {
	a, ts := seed(t, sample...)
	_, srv := syncServer(t, a)
	target := filepath.Join(t.TempDir(), "player")

	// something to undo, to see that a sync which clears nothing leaves it
	if code, _ := send(t, srv, "PATCH", "/api/tracks/"+ts[0].ID, obj{"field": "title", "text": "Wanderer II"}); code != 200 {
		t.Fatal(code)
	}
	job, d := planned(t, srv.URL, obj{"folder": target})
	if job.State != jobDone || d.Phase != "plan" || d.Plan == nil || d.Plan.Counts.Create != 4 || d.Plan.Counts.Update != 0 {
		t.Fatalf("%+v %+v", job, d)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("planning wrote %v", err)
	}
	if d.Plan.Where != target || !strings.HasSuffix(d.Plan.Text, "to create") {
		t.Errorf("%+v", d.Plan)
	}

	code, done, _ := applied(t, srv.URL, d.Plan.Plan, obj{})
	if sd := syncData(t, done); code != 200 || done.State != jobDone || sd.Phase != "apply" || sd.Summary != "4 created" {
		t.Fatalf("%d %+v %+v", code, done, sd)
	}
	if _, err := os.Stat(filepath.Join(target, "Shawn James", "Shadows", "01 - Wanderer II.mp3")); err != nil {
		t.Errorf("not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(a.Root(), "Shawn James")); !os.IsNotExist(err) {
		t.Error("the library folder was written to as well")
	}
	var lib struct{ History struct{ Undo string } }
	if getJSON(t, srv.URL+"/api/library", &lib); lib.History.Undo == "" {
		t.Error("a sync to another folder, which cleared nothing away, cleared the undo history")
	}
	if code, _, r := applied(t, srv.URL, d.Plan.Plan, obj{}); code != 410 || !strings.Contains(strings.ToLower(r.Error), "gone") {
		t.Errorf("a plan is applied once: %d %+v", code, r)
	}
}

func TestSyncingTheLibraryFolderRecordsItAndClearsUndo(t *testing.T) {
	a, ts := seed(t, sample...)
	_, srv := syncServer(t, a)
	send(t, srv, "PATCH", "/api/tracks/"+ts[0].ID, obj{"field": "title", "text": "Wanderer II"})

	var lib struct {
		Unsynced struct {
			N     int
			Known bool
		}
		History struct{ Undo string }
	}
	if getJSON(t, srv.URL+"/api/library", &lib); lib.Unsynced.Known || lib.History.Undo == "" {
		t.Fatalf("%+v", lib)
	}
	_, d := planned(t, srv.URL, obj{"library": true, "wolfgang": true})
	if d.Plan.Where != "Library folder" || d.Plan.Counts.Create != 4 {
		t.Fatalf("%+v", d.Plan)
	}
	if _, done, _ := applied(t, srv.URL, d.Plan.Plan, obj{}); done.State != jobDone {
		t.Fatalf("%+v", done)
	}
	if _, err := os.Stat(filepath.Join(a.Root(), "Shawn James", "Shadows", "01 - Wanderer II.mp3")); err != nil {
		t.Errorf("not written, the library folder is never given another format: %v", err)
	}
	lib.History.Undo = "?" // the answer leaves out what is empty
	after := lib
	after.History.Undo = ""
	getJSON(t, srv.URL+"/api/library", &after)
	if !after.Unsynced.Known || after.Unsynced.N != 0 || after.History.Undo != "" {
		t.Errorf("after the sync: %+v", after)
	}

	// edited again (a tag, so that the file keeps its name), then planned: it is written
	send(t, srv, "PATCH", "/api/tracks/"+ts[0].ID, obj{"field": "genre", "text": "Blues"})
	getJSON(t, srv.URL+"/api/library", &lib)
	if lib.Unsynced.N != 1 {
		t.Fatalf("one edit: %+v", lib)
	}
	_, d = planned(t, srv.URL, obj{"library": true})
	if d.Plan.Counts.Update != 1 {
		t.Fatalf("%+v", d.Plan)
	}
	applied(t, srv.URL, d.Plan.Plan, obj{})
	_, d = planned(t, srv.URL, obj{"library": true})
	if len(d.Plan.Ops) != 0 || d.Plan.Text != "Nothing to do" {
		t.Errorf("%+v", d.Plan)
	}
	if code, _, r := applied(t, srv.URL, d.Plan.Plan, obj{}); code != 422 || !strings.Contains(strings.ToLower(r.Error), "nothing to write") {
		t.Errorf("%d %+v", code, r)
	}
}

func TestStraysAreOnlyDeletedWhenAskedAndConfirmed(t *testing.T) {
	a, _ := seed(t, sample...)
	_, srv := syncServer(t, a)
	target := t.TempDir()
	stray := filepath.Join(target, "Old", "gone.mp3")
	os.MkdirAll(filepath.Dir(stray), 0o755)
	os.WriteFile(stray, []byte("x"), 0o644)

	_, d := planned(t, srv.URL, obj{"folder": target})
	if d.Plan.Counts.Stray != 1 || d.Plan.Counts.Remove != 0 || d.Plan.Prune {
		t.Fatalf("%+v", d.Plan)
	}
	var v browse.PlanView
	if code := raw(t, srv.URL, "POST", "/api/sync/"+d.Plan.Plan+"/prune", obj{"prune": true}, &v); code != 200 || v.Counts.Remove != 1 || v.Counts.Stray != 0 || !v.Prune {
		t.Fatalf("%d %+v", code, v)
	}
	for _, confirm := range []string{"", "yes", "DELETE"} {
		if code, _, r := applied(t, srv.URL, d.Plan.Plan, obj{"confirm": confirm}); code != 422 || !strings.Contains(strings.ToLower(r.Error), "type delete to remove 1 file") {
			t.Errorf("%q: %d %+v", confirm, code, r)
		}
	}
	if _, err := os.Stat(stray); err != nil {
		t.Fatalf("the stray went without the word: %v", err)
	}
	// back to leaving them: no word needed
	if code := raw(t, srv.URL, "POST", "/api/sync/"+d.Plan.Plan+"/prune", obj{"prune": false}, &v); code != 200 || v.Counts.Stray != 1 || v.Counts.Remove != 0 {
		t.Fatalf("%d %+v", code, v)
	}
	raw(t, srv.URL, "POST", "/api/sync/"+d.Plan.Plan+"/prune", obj{"prune": true}, &v)
	_, done, _ := applied(t, srv.URL, d.Plan.Plan, obj{"confirm": " delete "})
	if sd := syncData(t, done); done.State != jobDone || sd.Summary != "4 created · 1 deleted" {
		t.Fatalf("%+v %+v", done, sd)
	}
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Errorf("the stray is still there: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(stray)); !os.IsNotExist(err) {
		t.Errorf("the folder it left empty is still there: %v", err)
	}
}

func TestOnlyAFolderThatMakesSenseIsATarget(t *testing.T) {
	a, _ := seed(t, sample...)
	_, srv := syncServer(t, a)
	file := filepath.Join(t.TempDir(), "a file")
	os.WriteFile(file, []byte("x"), 0o644)
	cases := map[string]string{
		"":                                   "type the folder",
		"player":                             "whole path",
		file:                                 "is a file",
		filepath.Join(file, "inside"):        "not there",
		filepath.Join(t.TempDir(), "x", "y"): "not there",
		filepath.Join(a.Root(), "card"):      "inside the library folder",
		filepath.Dir(a.Root()):               "holds the library",
		"/":                                  "holds the library",
	}
	for folder, want := range cases {
		var r reply
		if code := raw(t, srv.URL, "POST", "/api/sync/plan", obj{"folder": folder}, &r); code != 422 || !strings.Contains(strings.ToLower(r.Error), want) {
			t.Errorf("%q: %d %+v, want %q", folder, code, r, want)
		}
	}
	// the library folder typed out is the library folder, and a new folder in an old one is made
	if _, d := planned(t, srv.URL, obj{"folder": a.Root() + "/"}); d.Plan == nil || d.Plan.Where != "Library folder" {
		t.Errorf("%+v", d)
	}
	if _, d := planned(t, srv.URL, obj{"folder": filepath.Join(t.TempDir(), "new")}); d.Plan == nil || d.Plan.Counts.Create != 4 {
		t.Errorf("%+v", d)
	}
}

func TestOnlyOneSyncGoesOnAtATime(t *testing.T) {
	a, _ := seed(t, sample...)
	s, srv := syncServer(t, a)
	release := make(chan struct{})
	if _, err := s.jobs.start("sync", func(ctx2 context.Context, _ func(any)) (any, error) { <-release; return nil, nil }); err != nil {
		t.Fatal(err)
	}
	defer close(release)
	var r reply
	if code := raw(t, srv.URL, "POST", "/api/sync/plan", obj{"library": true}, &r); code != 409 {
		t.Errorf("%d %+v", code, r)
	}
}

func TestAPlanThatWasDroppedCannotBeApplied(t *testing.T) {
	a, _ := seed(t, sample...)
	_, srv := syncServer(t, a)
	_, d := planned(t, srv.URL, obj{"folder": t.TempDir()})
	if code := raw(t, srv.URL, "DELETE", "/api/sync/"+d.Plan.Plan, nil, nil); code != 204 {
		t.Fatal(code)
	}
	for _, path := range []string{"/apply", "/prune"} {
		var r reply
		if code := raw(t, srv.URL, "POST", "/api/sync/"+d.Plan.Plan+path, obj{}, &r); code != 410 {
			t.Errorf("%s: %d %+v", path, code, r)
		}
	}
}

func TestAPlanNobodyWaitsForIsDropped(t *testing.T) {
	a, _ := seed(t, sample...)
	_, srv := syncServer(t, a)
	var v JobView
	raw(t, srv.URL, "POST", "/api/sync/plan", obj{"folder": t.TempDir()}, &v)
	raw(t, srv.URL, "DELETE", "/api/jobs/"+v.ID, nil, nil)
	if done := waitJob(t, srv.URL, v.ID); done.State != jobStopped && done.State != jobDone {
		t.Errorf("%+v", done)
	}
}

func TestACountIsFoundForALibraryNeverSynced(t *testing.T) {
	a, _ := seed(t, sample...)
	s, srv := syncServer(t, a)
	var lib struct {
		Unsynced struct {
			N     int
			Known bool
		}
	}
	getJSON(t, srv.URL+"/api/library", &lib)
	if lib.Unsynced.N != 0 || lib.Unsynced.Known {
		t.Fatalf("nothing is known before the folder is looked at: %+v", lib)
	}
	s.scanUnsynced()
	getJSON(t, srv.URL+"/api/library", &lib)
	if lib.Unsynced.N != 4 || lib.Unsynced.Known {
		t.Errorf("the one scan counts what would be written: %+v", lib)
	}
	// a folder that is up to date is recorded as such
	b, _ := seed(t, sample...)
	plan, err := b.PlanSync(app.SyncOptions{})
	if err != nil || plan.Apply() != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(b.Root(), ".mm", "synced.json"))
	s2, srv2 := syncServer(t, b)
	s2.scanUnsynced()
	getJSON(t, srv2.URL+"/api/library", &lib)
	if lib.Unsynced.N != 0 || !lib.Unsynced.Known {
		t.Errorf("an up-to-date folder is remembered: %+v", lib)
	}
}

func TestAFolderInsideALibraryGivenByARelativePathIsStillRefused(t *testing.T) {
	t.Chdir(t.TempDir())
	a, err := app.Open("lib")
	if err != nil {
		t.Fatal(err)
	}
	_, srv := syncServer(t, a)
	abs, _ := filepath.Abs("lib")
	for folder, want := range map[string]string{filepath.Join(abs, "card"): "inside the library folder", filepath.Dir(abs): "holds the library"} {
		var r reply
		if code := raw(t, srv.URL, "POST", "/api/sync/plan", obj{"folder": folder}, &r); code != 422 || !strings.Contains(strings.ToLower(r.Error), want) {
			t.Errorf("%q: %d %+v, want %q", folder, code, r, want)
		}
	}
	var out struct{ Targets []browse.TargetView }
	getJSON(t, srv.URL+"/api/sync/targets", &out)
	if !filepath.IsAbs(strings.Replace(out.Targets[0].Path, "~", "/home", 1)) {
		t.Errorf("the library folder is shown as %q", out.Targets[0].Path)
	}
}

func TestAPlanForTheWolfgangNamesItAndWarnsAboutWhatItWillNotShowWell(t *testing.T) {
	a, ts := seed(t, sample...)
	genre := "Alternative Rock"
	if _, err := a.Edit(ts[0].ID, app.Edit{Genre: &genre}); err != nil {
		t.Fatal(err)
	}
	_, srv := syncServer(t, a)
	target := t.TempDir()

	_, d := planned(t, srv.URL, obj{"folder": target, "wolfgang": true})
	if !strings.HasSuffix(d.Plan.Where, "Wolfgang WMP01") {
		t.Errorf("the plan is for %q, want it to name the Wolfgang", d.Plan.Where)
	}
	if len(d.Plan.Warnings) != 1 || d.Plan.Warnings[0].Track != ts[0].Title || !strings.Contains(d.Plan.Warnings[0].Message, genre) {
		t.Errorf("warnings %+v, want one about the genre of %q", d.Plan.Warnings, ts[0].Title)
	}

	_, plain := planned(t, srv.URL, obj{"folder": target})
	if strings.HasSuffix(plain.Plan.Where, "Wolfgang WMP01") || len(plain.Plan.Warnings) != 0 {
		t.Errorf("a plan that is not for the Wolfgang talks about it: %q, %+v", plain.Plan.Where, plain.Plan.Warnings)
	}
}
