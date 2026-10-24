package web

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

type database struct{ found []app.Candidate }

func (d database) Search(context.Context, app.Query) ([]app.Candidate, error) { return d.found, nil }
func (d database) CoverFront(context.Context, string) ([]byte, error)         { return nil, nil }

type search struct {
	Search     string `json:"search"`
	Title      string `json:"title"`
	Candidates []struct {
		Text string `json:"text"`
		Diff []struct{ Label, Old, New string }
	} `json:"candidates"`
}

// raw is sendInto under a shorter name.
var raw = sendInto

func TestATrackIsLookedUpAndIdentifiedOverHTTP(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a, WithSource(database{[]app.Candidate{{Title: "Found", Artist: "Someone", Album: "Somewhere", TrackNumber: 3, TrackTotal: 9, Score: 99}}}))
	unidentified := ts[3]
	var s search
	if code := raw(t, srv.URL, "POST", "/api/tracks/"+unidentified.ID+"/candidates", obj{}, &s); code != 200 || len(s.Candidates) != 1 || s.Search == "" {
		t.Fatalf("%d %+v", code, s)
	}
	if !strings.HasPrefix(s.Candidates[0].Text, " 99%  Somewhere") || s.Candidates[0].Diff[0].New != "Found" {
		t.Errorf("%+v", s.Candidates[0])
	}
	code, r := send(t, srv, "POST", "/api/identify/apply", obj{"search": s.Search, "index": 0})
	if code != 200 || r.Message != "Identified as Someone - Found" || r.History.Undo != "identification of a track" {
		t.Fatalf("%d %+v", code, r)
	}
	if got := titleOf(t, a, unidentified.ID); got != "Found" {
		t.Errorf("title %q", got)
	}
	if code, r := send(t, srv, "POST", "/api/identify/apply", obj{"search": s.Search, "index": 7}); code != 422 || !strings.Contains(r.Error, "search is gone") {
		t.Errorf("%d %+v", code, r)
	}
	if code := raw(t, srv.URL, "POST", "/api/tracks/"+unidentified.ID+"/candidates", obj{"query": obj{"title": "x", "artist": "y"}}, &s); code != 200 || s.Title != "x" {
		t.Errorf("%d %+v", code, s)
	}
}

func TestWithoutADatabaseThereIsNothingToLookUp(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a)
	for _, path := range []string{"/api/tracks/" + ts[0].ID + "/candidates", "/api/identify-all"} {
		if code, r := send(t, srv, "POST", path, obj{}); code != 422 || !strings.Contains(r.Error, "Lookup is not available") {
			t.Errorf("%s: %d %+v", path, code, r)
		}
	}
}

// waitJob follows a job until it is not running any more.
func waitJob(t *testing.T, base, id string) JobView {
	t.Helper()
	for i := 0; i < 200; i++ {
		var v JobView
		if code := getJSON(t, base+"/api/jobs/"+id, &v); code != 200 {
			t.Fatalf("job %s: %d", id, code)
		}
		if v.State != jobRunning {
			return v
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("the job never ended")
	return JobView{}
}

func TestEveryTrackThatNeedsWorkIsIdentifiedAsAJob(t *testing.T) {
	a, _ := seed(t, sample...) // one of the four is not identified
	srv := start(t, a, WithSource(database{[]app.Candidate{{Title: "Found", Artist: "Someone", Album: "X", TrackNumber: 1, TrackTotal: 1, Score: 99}}}))

	events := make(chan string, 20)
	res, err := http.Get(srv.URL + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	go func() {
		sc := bufio.NewScanner(res.Body)
		event := ""
		for sc.Scan() {
			if v, ok := strings.CutPrefix(sc.Text(), "event: "); ok {
				event = v
			} else if v, ok := strings.CutPrefix(sc.Text(), "data: "); ok && event == "job" {
				events <- v
			}
		}
	}()

	var v JobView
	if code := raw(t, srv.URL, "POST", "/api/identify-all", obj{}, &v); code != 200 || v.Kind != "identify" || v.ID == "" {
		t.Fatalf("%d %+v", code, v)
	}
	done := waitJob(t, srv.URL, v.ID)
	data, _ := json.Marshal(done.Data)
	if done.State != jobDone || !strings.Contains(string(data), `"identified":1`) || !strings.Contains(string(data), `"total":1`) {
		t.Errorf("%+v %s", done, data)
	}
	// the page is told when it ends
	deadline := time.After(3 * time.Second)
	for end := false; !end; {
		select {
		case e := <-events:
			var got JobView
			json.Unmarshal([]byte(e), &got)
			end = got.ID == v.ID && got.State == jobDone
		case <-deadline:
			t.Fatal("no job event for the end of the job")
		}
	}
	if code, r := send(t, srv, "POST", "/api/identify-all", obj{}); code != 422 || !strings.Contains(r.Error, "Nothing needs work") {
		t.Errorf("%d %+v", code, r)
	}
	if code, r := send(t, srv, "POST", "/api/undo", nil); code != 200 || r.Message != "Undid: identification of 1 track" {
		t.Errorf("one step to undo: %d %+v", code, r)
	}
}

func TestAJobCanBeStoppedAndOnlyOneOfAKindRuns(t *testing.T) {
	a, _ := seed(t, sample...)
	s := New(a)
	release := make(chan struct{})
	started := make(chan struct{})
	v, err := s.jobs.start("slow", func(ctx context.Context, report func(any)) (any, error) {
		report(obj{"done": 1})
		close(started)
		select {
		case <-ctx.Done():
		case <-release:
		}
		return obj{"done": 2}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if _, err := s.jobs.start("slow", nil); err != errBusy {
		t.Errorf("a second of the kind: %v", err)
	}
	if got := s.jobs.running(); len(got) != 1 || got[0].ID != v.ID {
		t.Errorf("%+v", got)
	}
	if !s.jobs.stop(v.ID) || s.jobs.stop("nope") {
		t.Error("stop")
	}
	for i := 0; i < 100; i++ {
		if got, _ := s.jobs.get(v.ID); got.State == jobStopped {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	got, _ := s.jobs.get(v.ID)
	t.Errorf("not stopped: %+v", got)
}

func TestOldTagsAreAdoptedOverHTTP(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a)
	u := ts[3]
	u.Hint.Title, u.Hint.Artist = "Old", "Someone"
	if err := a.Restore(u); err != nil {
		t.Fatal(err)
	}
	code, r := send(t, srv, "POST", "/api/adopt", obj{"ids": []string{u.ID}})
	if code != 200 || r.Message != "Used the old tags of 1 track" {
		t.Fatalf("%d %+v", code, r)
	}
	if code, r := send(t, srv, "POST", "/api/adopt", obj{"ids": []string{ts[0].ID}}); code != 422 || !strings.Contains(r.Error, "Select a track that needs work") {
		t.Errorf("%d %+v", code, r)
	}
	_ = manifest.Identified
}
