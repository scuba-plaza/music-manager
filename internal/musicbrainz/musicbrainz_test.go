package musicbrainz_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/musicbrainz"
	"music-manager/internal/testmp3"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func newClient(srv *httptest.Server) *musicbrainz.Client {
	c := musicbrainz.New("music-manager-test/0 (test@example.com)")
	c.BaseURL, c.CoverURL = srv.URL, srv.URL
	c.Interval = 0
	return c
}

func TestSearchFlattensRecordingsIntoCandidatesPerRelease(t *testing.T) {
	var gotQuery, gotUA, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery, gotUA, gotPath = r.URL.Query().Get("query"), r.UserAgent(), r.URL.Path
		if r.URL.Query().Get("fmt") != "json" {
			t.Errorf("fmt = %q", r.URL.Query().Get("fmt"))
		}
		w.Write(fixture(t, "search_dream3.json"))
	}))
	defer srv.Close()

	got, err := newClient(srv).Search(context.Background(), app.Query{Title: "Dream 3", Artist: "Max Richter"})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/ws/2/recording" || gotUA != "music-manager-test/0 (test@example.com)" {
		t.Errorf("path %q, user agent %q", gotPath, gotUA)
	}
	for _, want := range []string{`recording:"Dream 3"`, `artist:"Max Richter"`} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q lacks %s", gotQuery, want)
		}
	}
	if len(got) < 4 {
		t.Fatalf("only %d candidates", len(got))
	}

	first := got[0] // best score, then the earliest official release
	want := app.Candidate{
		RecordingID: "526986a4-84bd-4094-8e65-7bd6e26b39f1", ReleaseID: "e8e13f22-c640-44aa-b0fb-091aa19f2252",
		Title: "Dream 3 (In the Midst of My Life)", Artist: "Max Richter", Album: "From Sleep",
		TrackNumber: 1, TrackTotal: 7, Length: 603600 * time.Millisecond, Score: 100, Date: "2015-09-04",
	}
	if first != want {
		t.Errorf("first = %+v\nwant   %+v", first, want)
	}
	for i := 1; i < len(got); i++ {
		if got[i].Score > got[i-1].Score {
			t.Errorf("candidate %d scores %d, above the one before (%d)", i, got[i].Score, got[i-1].Score)
		}
	}
	for _, c := range got {
		if c.RecordingID == "" || c.ReleaseID == "" || c.Title == "" || c.Artist == "" {
			t.Errorf("incomplete candidate %+v", c)
		}
	}
}

func TestSearchJoinsArtistCreditsAndNumbersTracksWithinTheirDisc(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(fixture(t, "search_dream3.json"))
	}))
	defer srv.Close()
	got, _ := newClient(srv).Search(context.Background(), app.Query{Title: "Dream 3"})
	var multi *app.Candidate
	for i, c := range got {
		if c.Title == "Dream 3" && strings.Contains(c.Artist, "Clarice Jensen") {
			multi = &got[i]
			break
		}
	}
	if multi == nil {
		t.Fatal("collaboration not found")
	}
	if multi.Artist != "Clarice Jensen, Max Richter, Ben Russell" {
		t.Errorf("Artist = %q", multi.Artist)
	}
	if multi.TrackNumber != 12 || multi.TrackTotal != 15 { // track 12 on disc 2, which has 15 tracks
		t.Errorf("track %d of %d, want 12 of 15", multi.TrackNumber, multi.TrackTotal)
	}
}

func TestSearchOffersALatinNameForANonLatinArtist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"recordings":[{"id":"r1","score":100,"title":"سأرجع","length":200000,
		  "artist-credit":[{"name":"فيروز","artist":{"name":"فيروز","aliases":[
		    {"name":"Fairuz","locale":"en","type":"Artist name","primary":true},
		    {"name":"فيروز","locale":"ar","type":"Artist name","primary":true}]}}],
		  "releases":[{"id":"rel1","title":"القدس في البال","status":"Official","date":"1971",
		    "media":[{"position":1,"track-count":9,"track":[{"number":"3","title":"سأرجع"}]}]}]}]}`))
	}))
	defer srv.Close()
	got, err := newClient(srv).Search(context.Background(), app.Query{Title: "x"})
	if err != nil || len(got) != 1 {
		t.Fatalf("%v, %v", got, err)
	}
	if got[0].Artist != "فيروز" || got[0].ArtistASCII != "Fairuz" || got[0].Album != "القدس في البال" {
		t.Errorf("candidate = %+v", got[0])
	}
}

func TestSearchQueryEscapingAndOptionalParts(t *testing.T) {
	var q string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query().Get("query")
		w.Write([]byte(`{"recordings":[]}`))
	}))
	defer srv.Close()
	c := newClient(srv)
	if _, err := c.Search(context.Background(), app.Query{Title: `He said "hi" \o/`}); err != nil {
		t.Fatal(err)
	}
	if want := `recording:"He said \"hi\" \\o/"`; q != want {
		t.Errorf("query = %q, want %q", q, want)
	}
	c.Search(context.Background(), app.Query{Title: "T", Artist: "A", Album: "B"})
	if want := `recording:"T" AND artist:"A" AND release:"B"`; q != want {
		t.Errorf("query = %q, want %q", q, want)
	}
	if _, err := c.Search(context.Background(), app.Query{}); err == nil {
		t.Error("an empty query was sent")
	}
}

func TestRequestsAreSpacedOutAndRetriedOn503(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(`{"recordings":[]}`))
	}))
	defer srv.Close()

	now := time.Unix(1000, 0)
	var slept []time.Duration
	c := newClient(srv)
	c.Interval = time.Second
	c.Now = func() time.Time { return now }
	c.Sleep = func(d time.Duration) { slept = append(slept, d); now = now.Add(d) }

	if _, err := c.Search(context.Background(), app.Query{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Errorf("%d requests, want the 503 and one retry", calls.Load())
	}
	if len(slept) != 1 || slept[0] != 2*time.Second {
		t.Errorf("slept %v, want one wait of the 2s Retry-After", slept)
	}

	slept = nil
	c.Search(context.Background(), app.Query{Title: "b"}) // immediately after the last one
	if len(slept) != 1 || slept[0] != time.Second {
		t.Errorf("a second request right away slept %v, want 1s", slept)
	}
}

func TestSearchGivesUpAfterRepeated503AndReportsOtherErrors(t *testing.T) {
	var calls atomic.Int32
	status := http.StatusServiceUnavailable
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(status)
	}))
	defer srv.Close()
	c := newClient(srv)
	c.Sleep = func(time.Duration) {}
	if _, err := c.Search(context.Background(), app.Query{Title: "a"}); err == nil || !strings.Contains(err.Error(), "503") {
		t.Errorf("err = %v", err)
	}
	if calls.Load() != 4 { // the request and three retries
		t.Errorf("%d requests", calls.Load())
	}
	status = http.StatusBadRequest
	calls.Store(0)
	if _, err := c.Search(context.Background(), app.Query{Title: "a"}); err == nil || calls.Load() != 1 {
		t.Errorf("err = %v after %d requests; a 400 must not be retried", err, calls.Load())
	}
}

func TestCoverFront(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/release/has-cover/front-500":
			http.Redirect(w, r, "/files/img.jpg", http.StatusFound) // the real archive redirects
		case "/files/img.jpg":
			w.Write(testmp3.JPEG)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := newClient(srv)

	got, err := c.CoverFront(context.Background(), "has-cover")
	if err != nil || string(got) != string(testmp3.JPEG) {
		t.Errorf("got %q, %v", got, err)
	}
	got, err = c.CoverFront(context.Background(), "no-cover")
	if err != nil || got != nil {
		t.Errorf("a release without art should give nil, nil; got %q, %v", got, err)
	}
}
