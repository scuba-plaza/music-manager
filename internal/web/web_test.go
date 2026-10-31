package web

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/browse"
	"music-manager/internal/manifest"
	"music-manager/internal/testmp3"
	"music-manager/internal/version"
	"music-manager/internal/web/webtest"
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

type sortRow struct {
	Kind, TrackID, Right string
}

func rowsOf(t *testing.T, url string) []sortRow {
	t.Helper()
	var got struct {
		View struct{ Rows []sortRow }
	}
	if code := getJSON(t, url, &got); code != 200 {
		t.Fatalf("%s: status %d", url, code)
	}
	return got.View.Rows
}

func TestTheLibraryCanBeSortedByEveryKeyBothWaysAndStillFiltered(t *testing.T) {
	a, err := app.Open(filepath.Join(t.TempDir(), "lib"))
	if err != nil {
		t.Fatal(err)
	}
	titleOfID := map[string]string{}
	for _, s := range []struct {
		frames                        int
		title, artist, album, release string
		track                         int
	}{
		{300, "Beta", "Zed", "Second", "2001", 2},
		{200, "Gamma", "Abe", "First", "1999-05", 1},
		{100, "Alpha", "Mo", "", "", 0},
	} {
		file := filepath.Join(t.TempDir(), "song.mp3")
		if err := os.WriteFile(file, testmp3.Silent(s.frames), 0o644); err != nil {
			t.Fatal(err)
		}
		rep, err := a.Ingest([]string{file})
		if err != nil || len(rep.Added) != 1 {
			t.Fatalf("%+v, %v", rep, err)
		}
		tr, err := a.Edit(rep.Added[0].ID, app.Edit{Title: &s.title, Artist: &s.artist, Album: &s.album, Released: &s.release, TrackNumber: &s.track})
		if err != nil {
			t.Fatal(err)
		}
		titleOfID[tr.ID] = s.title
	}
	srv := start(t, a)
	order := func(query string) string {
		var titles []string
		for _, r := range rowsOf(t, srv.URL+"/api/library?"+query) {
			if r.Kind != "track" {
				t.Fatalf("%s: a %s row in a sorted list", query, r.Kind)
			}
			titles = append(titles, titleOfID[r.TrackID])
		}
		return strings.Join(titles, " ")
	}
	for _, c := range []struct{ query, want string }{
		{"sort=title", "Alpha Beta Gamma"},
		{"sort=title&reverse=1", "Gamma Beta Alpha"},
		{"sort=artist", "Gamma Alpha Beta"},
		{"sort=album", "Gamma Beta Alpha"},
		{"sort=track", "Gamma Beta Alpha"},
		{"sort=length", "Alpha Gamma Beta"},
		{"sort=length&reverse=1", "Beta Gamma Alpha"},
		{"sort=released", "Beta Gamma Alpha"},
		{"sort=released&reverse=1", "Gamma Beta Alpha"},
		{"sort=added", "Alpha Gamma Beta"},
		{"sort=edited", "Alpha Gamma Beta"},
		{"sort=TITLE&q=beta", "Beta"},
	} {
		if got := order(c.query); got != c.want {
			t.Errorf("?%s: %s, want %s", c.query, got, c.want)
		}
	}
}

func TestSortingByADateShowsTheDateBesideEachTrack(t *testing.T) {
	a, ts := seed(t, sample...)
	released := "1999-05"
	if _, err := a.Edit(ts[0].ID, app.Edit{Released: &released}); err != nil {
		t.Fatal(err)
	}
	srv := start(t, a)
	var shown []string
	for _, r := range rowsOf(t, srv.URL+"/api/library?sort=released") {
		shown = append(shown, r.Right)
	}
	if got := strings.Join(shown, "|"); got != "1999-05|—|—|—" {
		t.Errorf("beside the tracks: %s, want the date and a dash for each without one", got)
	}
	for _, r := range rowsOf(t, srv.URL+"/api/library?sort=title") {
		if r.Right != "" {
			t.Errorf("a date beside a track sorted by its title: %q", r.Right)
		}
	}
}

func TestASortKeyThatDoesNotExistSaysWhichDo(t *testing.T) {
	a, _ := seed(t, sample...)
	var r reply
	if code := sendInto(t, start(t, a).URL, "GET", "/api/library?sort=colour", nil, &r); code != 400 {
		t.Fatalf("status %d", code)
	}
	for _, key := range []string{"artist", "released", "edited"} {
		if !strings.Contains(r.Error, key) {
			t.Errorf("the error %q does not list %s", r.Error, key)
		}
	}
}

type inspectorField struct {
	Key, Text string
	Dim       bool
}

type inspected struct {
	Tabs []struct {
		Name   string
		Fields []inspectorField
	}
	Dates, About string
}

func (v inspected) field(key string) inspectorField {
	for _, tab := range v.Tabs {
		for _, f := range tab.Fields {
			if f.Key == key {
				return f
			}
		}
	}
	return inspectorField{Key: "missing: " + key}
}

func inspect(t *testing.T, srv *httptest.Server, id string) inspected {
	t.Helper()
	var v inspected
	if code := getJSON(t, srv.URL+"/api/tracks/"+id, &v); code != 200 {
		t.Fatalf("inspector of %s: status %d", id, code)
	}
	return v
}

func trackIDsOf(t *testing.T, srv *httptest.Server, query string) []string {
	t.Helper()
	var ids []string
	for _, r := range rowsOf(t, srv.URL+"/api/library?q="+query) {
		if r.TrackID != "" {
			ids = append(ids, r.TrackID)
		}
	}
	return ids
}

func TestAGenreIsSetOverHTTPAndFoundByTheGenreFilter(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a)
	id := ts[0].ID

	if code, r := send(t, srv, "PATCH", "/api/tracks/"+id, obj{"field": "genre", "text": "Neoclassical"}); code != 200 || r.Message != "Saved" {
		t.Fatalf("%d %+v", code, r)
	}
	if got := inspect(t, srv, id).field("genre").Text; got != "Neoclassical" {
		t.Errorf("the inspector shows the genre %q", got)
	}
	if got := trackIDsOf(t, srv, "genre:neo"); len(got) != 1 || got[0] != id {
		t.Errorf("genre:neo finds %v, want only %s", got, id)
	}
	if got := trackIDsOf(t, srv, "genre:folk"); len(got) != 0 {
		t.Errorf("genre:folk finds %v", got)
	}

	if code, _ := send(t, srv, "PATCH", "/api/tracks/"+id, obj{"field": "genre", "text": ""}); code != 200 {
		t.Fatal(code)
	}
	if got := inspect(t, srv, id).field("genre").Text; got != "" {
		t.Errorf("a cleared genre is still shown: %q", got)
	}
	if got := trackIDsOf(t, srv, "genre:neo"); len(got) != 0 {
		t.Errorf("genre:neo still finds %v after the genre was cleared", got)
	}
}

func TestTheInspectorSaysWhenATrackWasReleasedAddedAndEdited(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a)
	id := ts[0].ID
	today := time.Now().Format("2006-01-02")

	if code, r := send(t, srv, "PATCH", "/api/tracks/"+id, obj{"field": "released", "text": "1999-05"}); code != 200 || r.Message != "Saved" {
		t.Fatalf("%d %+v", code, r)
	}
	v := inspect(t, srv, id)
	if got := v.field("released").Text; got != "1999-05" {
		t.Errorf("released %q", got)
	}
	if want := "added " + today + " · edited " + today; v.Dates != want {
		t.Errorf("dates %q, want %q", v.Dates, want)
	}

	if code, _ := send(t, srv, "PATCH", "/api/tracks/"+id, obj{"field": "released", "text": ""}); code != 200 {
		t.Fatal(code)
	}
	if got := inspect(t, srv, id).field("released").Text; got != "" {
		t.Errorf("a cleared date is still shown: %q", got)
	}
}

func TestTheInspectorSaysWhereATrackCameFrom(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a)
	id := ts[0].ID
	link, none := "https://www.youtube.com/watch?v=abc123", ""

	if _, err := a.Edit(id, app.Edit{SourceURL: &link}); err != nil {
		t.Fatal(err)
	}
	if got := inspect(t, srv, id).About; !strings.Contains(got, "from "+link) {
		t.Errorf("about %q does not say where the track is from", got)
	}
	if _, err := a.Edit(id, app.Edit{SourceURL: &none}); err != nil {
		t.Fatal(err)
	}
	if got := inspect(t, srv, id).About; strings.Contains(got, "from") {
		t.Errorf("about %q still says where the track is from", got)
	}
}

func TestTheLatinTabShowsAFormSetByHandAsSetAndAWorkedOutFormDimly(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a)
	id := ts[2].ID
	before := inspect(t, srv, id)
	workedOut := before.field("latinTitle")
	if !workedOut.Dim || workedOut.Text == "" || !before.field("latinArtist").Dim {
		t.Fatalf("nothing is set by hand yet: %+v, %+v", workedOut, before.field("latinArtist"))
	}

	if code, r := send(t, srv, "PATCH", "/api/tracks/"+id, obj{"field": "latinTitle", "text": "Salam"}); code != 200 || r.Message != "Saved" {
		t.Fatalf("%d %+v", code, r)
	}
	after := inspect(t, srv, id)
	if got := after.field("latinTitle"); got.Text != "Salam" || got.Dim {
		t.Errorf("the form set by hand: %+v", got)
	}
	if got := after.field("latinArtist"); !got.Dim {
		t.Errorf("the artist's form, which nobody set, is not dim: %+v", got)
	}

	if code, _ := send(t, srv, "PATCH", "/api/tracks/"+id, obj{"field": "latinTitle", "text": ""}); code != 200 {
		t.Fatal(code)
	}
	if got := inspect(t, srv, id).field("latinTitle"); got != workedOut {
		t.Errorf("after clearing: %+v, want the worked-out form back: %+v", got, workedOut)
	}
}

type wolfgangPreview struct {
	Rows     []struct{ Label, Value string }
	Warnings []string
}

func wolfgangTabOf(t *testing.T, srv *httptest.Server, id string) wolfgangPreview {
	t.Helper()
	var v struct {
		Tabs []struct {
			Name     string
			Wolfgang *wolfgangPreview
		}
	}
	if code := getJSON(t, srv.URL+"/api/tracks/"+id, &v); code != 200 || len(v.Tabs) != 4 || v.Tabs[3].Name != "Wolfgang" || v.Tabs[3].Wolfgang == nil {
		t.Fatalf("%d %+v", code, v)
	}
	return *v.Tabs[3].Wolfgang
}

func TestTheWolfgangTabSaysWhatThePlayerWillShowAndWhatItWillNotShowWell(t *testing.T) {
	a, ts := seed(t, seedTrack{"Bahlam Bek", "Abdel Halim Hafez", "Abd El Halim Hafez", 1}, seedTrack{"Song", "Band", "", 0})
	genre := "Alternative Rock"
	if _, err := a.Edit(ts[1].ID, app.Edit{Genre: &genre}); err != nil {
		t.Fatal(err)
	}
	srv := start(t, a, WithWolfgang())

	shown := map[string]string{}
	for _, r := range wolfgangTabOf(t, srv, ts[0].ID).Rows {
		shown[r.Label] = r.Value
	}
	if shown["Artist"] != "A. H. Hafez" || shown["Album"] != "Abd El Halim" {
		t.Errorf("the player shows %+v, want the artist and the album within its 12 characters", shown)
	}
	if w := wolfgangTabOf(t, srv, ts[0].ID).Warnings; len(w) != 0 {
		t.Errorf("a track the player shows well has warnings: %v", w)
	}
	if w := wolfgangTabOf(t, srv, ts[1].ID).Warnings; len(w) != 1 || !strings.Contains(w[0], "genre") {
		t.Errorf("warnings %v, want one about the genre", w)
	}
}

func TestProfilesEndpointListsBuiltinsThenUser(t *testing.T) {
	a, _ := seed(t)
	user := "[[profile]]\nname = \"my-player\"\nlabel = \"My Player\"\nlatin_names = true\n"
	var got []ProfileView
	if code := getJSON(t, start(t, a, WithProfiles(registryWith(t, user))).URL+"/api/profiles", &got); code != 200 {
		t.Fatal(code)
	}
	want := []ProfileView{
		{Name: "generic", Label: "Generic", Builtin: true},
		{Name: "wolfgang-wmp01", Label: "Wolfgang WMP01", Builtin: true},
		{Name: "my-player", Label: "My Player", Builtin: false},
	}
	if !slices.Equal(got, want) {
		t.Errorf("profiles %+v, want %+v", got, want)
	}

	a.UseProfiles(registryWith(t, user))
	var fromApp []ProfileView
	if getJSON(t, start(t, a).URL+"/api/profiles", &fromApp); !slices.Equal(fromApp, want) {
		t.Errorf("a server without its own profiles does not use the app's: %+v", fromApp)
	}
	b, _ := seed(t)
	var builtin []ProfileView
	if getJSON(t, start(t, b).URL+"/api/profiles", &builtin); !slices.Equal(builtin, want[:2]) {
		t.Errorf("profiles without a config %+v", builtin)
	}
}

func TestTheDeviceTabFollowsTheProfileParameter(t *testing.T) {
	a, ts := seed(t, seedTrack{"Bahlam Bek Ya Habibi Forever", "Abdel Halim Hafez", "Abd El Halim Hafez", 1})
	srv := start(t, a, WithWolfgang(), WithProfiles(registryWith(t, "[[profile]]\nname = \"my-player\"\nlatin_names = true\n")))
	type tab struct {
		ID       string
		Name     string
		Wolfgang *wolfgangPreview
		Device   *wolfgangPreview
	}
	tabsFor := func(query string) (int, []tab) {
		var v struct{ Tabs []tab }
		code := getJSON(t, srv.URL+"/api/tracks/"+ts[0].ID+query, &v)
		return code, v.Tabs
	}
	rows := func(p *wolfgangPreview) map[string]string {
		shown := map[string]string{}
		for _, r := range p.Rows {
			shown[r.Label] = r.Value
		}
		return shown
	}

	_, plain := tabsFor("")
	if len(plain) != 4 || plain[3].ID != "wolfgang" || plain[3].Wolfgang == nil {
		t.Fatalf("without a profile the old page's tabs change: %+v", plain)
	}

	_, wolfgang := tabsFor("?profile=wolfgang-wmp01")
	if len(wolfgang) != 5 || wolfgang[4].ID != "device" || wolfgang[4].Device == nil {
		t.Fatalf("tabs %+v", wolfgang)
	}
	if got := rows(wolfgang[4].Device); got["Artist"] != "A. H. Hafez" || got["Cut"] == "" {
		t.Errorf("the Wolfgang preview %+v", got)
	}

	_, mine := tabsFor("?profile=my-player")
	if got := rows(mine[4].Device); got["Artist"] != "Abdel Halim Hafez" || got["Cut"] != "" {
		t.Errorf("the my-player preview %+v", got)
	}

	var r reply
	if code := getJSON(t, srv.URL+"/api/tracks/"+ts[0].ID+"?profile=nobody", &r); code != 400 || !strings.Contains(r.Error, `"nobody"`) {
		t.Errorf("unknown profile: %d %+v", code, r)
	}
}

func answerStrictly(t *testing.T, base, method, path string, body, into any) int {
	t.Helper()
	var in io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		in = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, base+path, in)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-MM", "1")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	dec := json.NewDecoder(res.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return res.StatusCode
}

func TestEveryResponseDecodesIntoItsNamedType(t *testing.T) {
	a, ts := seed(t, sample...)
	drive := t.TempDir()
	s := New(a, WithWolfgang(), WithSource(webtest.FakeSource{}))
	s.mounts = func() []string { return []string{drive} }
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	var info InfoView
	if code := answerStrictly(t, srv.URL, "GET", "/api/info", nil, &info); code != 200 || !strings.HasSuffix(info.Root, "lib") || info.Version != version.String() || !info.Wolfgang || info.CanAdd || !info.Lookup {
		t.Errorf("info %d %+v", code, info)
	}

	if err := a.RecordSynced(); err != nil {
		t.Fatal(err)
	}
	if code, r := send(t, srv, "PATCH", "/api/tracks/"+ts[0].ID, obj{"field": "title", "text": "Wanderer II"}); code != 200 {
		t.Fatalf("%d %+v", code, r)
	}
	var lib LibraryResponse
	if code := answerStrictly(t, srv.URL, "GET", "/api/library", nil, &lib); code != 200 || lib.Stamp == "" || lib.View.Tracks != 4 || lib.View.Shown != 4 || lib.Unsynced != (UnsyncedView{N: 1, Known: true}) || lib.History != (browse.History{Undo: "change of title"}) {
		t.Errorf("library %d %+v", code, lib)
	}

	var targets SyncTargetsView
	if code := answerStrictly(t, srv.URL, "GET", "/api/sync/targets", nil, &targets); code != 200 || !targets.Wolfgang || len(targets.Targets) != 2 || targets.Targets[0].Kind != "library" || targets.Targets[1].Kind != "drive" || targets.Targets[1].Path != drive {
		t.Errorf("targets %d %+v", code, targets)
	}

	form := obj{"title": "A song", "artist": "Some Band", "album": "Record", "track": "2", "of": "9", "released": "", "genre": "", "cutStart": "", "cutEnd": ""}
	var added PathView
	if code := answerStrictly(t, srv.URL, "POST", "/api/add/path", obj{"form": form}, &added); code != 200 || added != (PathView{Path: "Some Band/Record/02 - A song.mp3"}) {
		t.Errorf("add path %d %+v", code, added)
	}

	var typed PathView
	if code := answerStrictly(t, srv.URL, "POST", "/api/tracks/"+ts[1].ID+"/path", obj{"field": "title", "text": "Flow II"}, &typed); code != 200 || typed != (PathView{Path: "Shawn James/Shadows/02 - Flow II.mp3"}) {
		t.Errorf("track path %d %+v", code, typed)
	}

	var hint HintView
	if code := answerStrictly(t, srv.URL, "POST", "/api/group/hint", obj{"key": "artist\x1fShawn James", "field": "artist", "text": "داود القَيسي"}, &hint); code != 200 || hint != (HintView{Hint: "joins the artist داود القَيسي (1 track)"}) {
		t.Errorf("group hint %d %+v", code, hint)
	}

	var failed ErrorView
	if code := answerStrictly(t, srv.URL, "GET", "/api/library?sort=nope", nil, &failed); code != 400 || !strings.HasPrefix(failed.Error, "Cannot sort by") {
		t.Errorf("error %d %+v", code, failed)
	}
}
