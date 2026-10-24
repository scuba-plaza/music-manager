package web

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/testmp3"
)

type seedTrack struct {
	title, artist, album string
	n                    int
}

// seed adds identified tracks (one real audio file each) to a fresh library.
func seed(t *testing.T, specs ...seedTrack) (*app.App, []manifest.Track) {
	t.Helper()
	a, err := app.Open(filepath.Join(t.TempDir(), "lib"))
	if err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	for i, s := range specs {
		name := filepath.Join(src, string(rune('a'+i))+".mp3")
		if err := os.WriteFile(name, testmp3.Silent(100+i), 0o644); err != nil {
			t.Fatal(err)
		}
		_ = s
	}
	if _, err := a.Ingest([]string{src}); err != nil {
		t.Fatal(err)
	}
	m, err := a.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range specs {
		if s.title == "" {
			continue
		}
		e := app.Edit{Title: &s.title, Artist: &s.artist, Album: &s.album, TrackNumber: &s.n}
		if _, err := a.Edit(m.Tracks[i].ID, e); err != nil {
			t.Fatal(err)
		}
	}
	m, _ = a.Manifest()
	return a, m.Tracks
}

func start(t *testing.T, a *app.App, opts ...Option) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(New(a, opts...).Handler())
	t.Cleanup(srv.Close)
	return srv
}

func getJSON(t *testing.T, url string, into any) int {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if into != nil {
		if err := json.NewDecoder(res.Body).Decode(into); err != nil {
			t.Fatalf("%s: %v", url, err)
		}
	}
	return res.StatusCode
}

var sample = []seedTrack{
	{"The Wanderer", "Shawn James", "Shadows", 1},
	{"Flow", "Shawn James", "Shadows", 2},
	{"سلام", "داود القَيسي", "", 0},
	{"", "", "", 0}, // not identified
}

func TestThePageIsServed(t *testing.T) {
	a, _ := seed(t, sample...)
	srv := start(t, a)
	res, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(string(body), "<title>mm</title>") {
		t.Errorf("status %d, body %.80q", res.StatusCode, body)
	}
	for _, p := range []string{"/app.css", "/js/main.js"} {
		if r, err := http.Get(srv.URL + p); err != nil || r.StatusCode != 200 {
			t.Errorf("%s: %v %v", p, r, err)
		}
	}
}

func TestInfoSaysWhatTheServerCanDo(t *testing.T) {
	a, _ := seed(t, sample...)
	var info map[string]any
	getJSON(t, start(t, a).URL+"/api/info", &info)
	if info["canAdd"] != false || info["lookup"] != false || info["wolfgang"] != false {
		t.Errorf("a server with nothing added: %v", info)
	}
	var full map[string]any
	getJSON(t, start(t, a, WithWolfgang(), WithSource(nil)).URL+"/api/info", &full)
	if full["wolfgang"] != true {
		t.Errorf("wolfgang not on: %v", full)
	}
}

func TestTheLibraryComesAsAFlatOpenTree(t *testing.T) {
	a, tracks := seed(t, sample...)
	var got struct {
		Stamp string
		View  struct {
			Rows []struct {
				Kind, Key, Label, Parent, TrackID string
				Depth                             int
			}
			Tracks, NeedWork, Shown int
		}
		Unsynced struct{ N int }
	}
	if code := getJSON(t, start(t, a).URL+"/api/library", &got); code != 200 {
		t.Fatalf("status %d", code)
	}
	if got.Stamp == "" || got.View.Tracks != 4 || got.View.NeedWork != 1 {
		t.Errorf("stamp %q, tracks %d, need work %d", got.Stamp, got.View.Tracks, got.View.NeedWork)
	}
	var labels []string
	ids := map[string]bool{}
	for _, r := range got.View.Rows {
		labels = append(labels, strings.Repeat(" ", r.Depth)+r.Label)
		if r.TrackID != "" {
			ids[r.TrackID] = true
		}
	}
	if len(ids) != len(tracks) {
		t.Errorf("tracks in rows: %d of %d", len(ids), len(tracks))
	}
	// The Arabic artist is put in order by how it reads in Latin letters (Dawud…).
	want := []string{"Needs work", " d", "داود القَيسي", " Singles", "  سلام", "Shawn James", " Shadows", "  The Wanderer", "  Flow"}
	if strings.Join(labels, "|") != strings.Join(want, "|") {
		t.Errorf("rows %q, want %q", labels, want)
	}
}

func TestTheLibraryCanBeFilteredAndSorted(t *testing.T) {
	a, _ := seed(t, sample...)
	srv := start(t, a)
	var got struct {
		View struct {
			Rows []struct{ Kind, Label string }
			Sort struct{ Key, Title string }
		}
	}
	getJSON(t, srv.URL+"/api/library?q=flow", &got)
	if len(got.View.Rows) != 3 || got.View.Rows[2].Label != "Flow" {
		t.Errorf("filtered: %+v", got.View.Rows)
	}
	getJSON(t, srv.URL+"/api/library?sort=title", &got)
	if got.View.Sort.Key != "title" || !strings.HasPrefix(got.View.Sort.Title, "Library · Title") || got.View.Rows[0].Kind != "track" {
		t.Errorf("sorted: %+v", got.View)
	}
	if code := getJSON(t, srv.URL+"/api/library?sort=nonsense", nil); code != 400 {
		t.Errorf("a key that does not exist: status %d", code)
	}
}

func TestATracksInspectorAndItsCover(t *testing.T) {
	a, tracks := seed(t, sample...)
	srv := start(t, a)
	var v struct {
		ID, Title string
		Tabs      []struct{ Name string }
		Path      string
	}
	if code := getJSON(t, srv.URL+"/api/tracks/"+tracks[0].ID, &v); code != 200 || v.Title != "The Wanderer" || len(v.Tabs) != 3 || v.Path == "" {
		t.Errorf("status %d, %+v", code, v)
	}
	if code := getJSON(t, srv.URL+"/api/tracks/nope", nil); code != 404 {
		t.Errorf("unknown track: status %d", code)
	}
	if res, _ := http.Get(srv.URL + "/api/tracks/" + tracks[0].ID + "/cover"); res.StatusCode != 404 {
		t.Errorf("no cover yet: status %d", res.StatusCode)
	}
	if err := a.SetCover(tracks[0].ID, testmp3.PNG); err != nil {
		t.Fatal(err)
	}
	res, err := http.Get(srv.URL + "/api/tracks/" + tracks[0].ID + "/cover")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/png" || len(data) != len(testmp3.PNG) || res.Header.Get("ETag") == "" {
		t.Errorf("status %d, type %q, %d bytes", res.StatusCode, res.Header.Get("Content-Type"), len(data))
	}
	getJSON(t, srv.URL+"/api/tracks/"+tracks[0].ID, &v)
	var cover struct {
		CoverInfo string
		Tabs      []struct{ Fields []struct{ Key, Text string } }
	}
	getJSON(t, srv.URL+"/api/tracks/"+tracks[0].ID, &cover)
	if !strings.Contains(cover.CoverInfo, "16×12 PNG") {
		t.Errorf("cover info %q", cover.CoverInfo)
	}
}

func TestThePeekOfAnArtist(t *testing.T) {
	a, _ := seed(t, sample...)
	srv := start(t, a)
	var p struct {
		Kind, Title string
		Info        []string
		List        []struct{ Key, Left string }
	}
	if code := getJSON(t, srv.URL+"/api/peek?key=artist%1FShawn+James", &p); code != 200 || p.Kind != "artist" || len(p.List) != 1 || p.List[0].Left != "Shadows" {
		t.Errorf("status %d, %+v", code, p)
	}
	if code := getJSON(t, srv.URL+"/api/peek?key=artist%1FNobody", nil); code != 404 {
		t.Errorf("unknown row: status %d", code)
	}
}

func TestTheGuardRefusesStrangers(t *testing.T) {
	a, _ := seed(t, sample...)
	srv := start(t, a)
	do := func(method, path string, hdr map[string]string) int {
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader("{}"))
		for k, v := range hdr {
			if k == "Host" {
				req.Host = v
			} else {
				req.Header.Set(k, v)
			}
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if c := do("GET", "/api/info", map[string]string{"Host": "evil.example:7878"}); c != 403 {
		t.Errorf("a name that is not this computer's: %d", c)
	}
	if c := do("GET", "/api/info", map[string]string{"Host": "localhost:7878"}); c != 200 {
		t.Errorf("localhost: %d", c)
	}
	if c := do("POST", "/api/nothing", nil); c != 403 {
		t.Errorf("a change without the X-MM header: %d", c)
	}
	if c := do("POST", "/api/nothing", map[string]string{"X-MM": "1", "Origin": "https://evil.example"}); c != 403 {
		t.Errorf("a change from another origin: %d", c)
	}
	if c := do("POST", "/api/nothing", map[string]string{"X-MM": "1", "Origin": srv.URL}); c == 403 {
		t.Errorf("a change from the page itself was refused")
	}
	if c := do("OPTIONS", "/api/info", map[string]string{"Origin": "https://evil.example", "Access-Control-Request-Method": "POST"}); c == 200 {
		t.Errorf("a preflight request was answered: %d", c)
	}
}

func TestListeningOnlyOnThisComputer(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:0", "192.168.1.10:0", "example.com:80"} {
		if ln, err := Listen(addr, false); err == nil {
			ln.Close()
			t.Errorf("listened on %s", addr)
		}
	}
	ln, err := Listen("127.0.0.1:0", false)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if !strings.HasPrefix(URL(ln), "http://127.0.0.1:") {
		t.Errorf("url %q", URL(ln))
	}
	taken, err := Listen(ln.Addr().String(), true)
	if err != nil {
		t.Fatalf("a taken address should fall back to a free port: %v", err)
	}
	taken.Close()
	if _, err := Listen(ln.Addr().String(), false); err == nil {
		t.Error("a taken address was listened on twice")
	}
}

func TestThePageIsToldWhenTheLibraryChanges(t *testing.T) {
	a, tracks := seed(t, sample...)
	s := New(a)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	ctx := t.Context()
	go s.watch(ctx)

	res, err := http.Get(srv.URL + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	events := make(chan string, 8)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			if v, ok := strings.CutPrefix(sc.Text(), "event: "); ok {
				events <- v
			}
		}
	}()
	wait := func(want string) {
		t.Helper()
		select {
		case got := <-events:
			if got != want {
				t.Fatalf("event %q, want %q", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("no %q event", want)
		}
	}
	wait("hello")
	title := "Changed"
	if _, err := a.Edit(tracks[0].ID, app.Edit{Title: &title}); err != nil {
		t.Fatal(err)
	}
	wait("library")
}
