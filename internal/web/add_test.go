package web

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/browse"
	"music-manager/internal/testmp3"
)

type downloader struct {
	audio []byte
	info  app.Info
	calls []string
}

func (d *downloader) Download(_ context.Context, url, dir string) (app.Downloaded, error) {
	d.calls = append(d.calls, url)
	path := filepath.Join(dir, "vid.mp3")
	if err := os.WriteFile(path, d.audio, 0o644); err != nil {
		return app.Downloaded{}, err
	}
	return app.Downloaded{Path: path, Info: d.info}, nil
}

func TestATrackIsFetchedFromALinkAndAdded(t *testing.T) {
	a, ts := seed(t, sample...)
	dl := &downloader{audio: testmp3.Silent(700), info: app.Info{Title: "A song", Uploader: "Some Band - Topic", TrackNumber: 2}}
	srv := start(t, a, WithDownloader(dl))

	var v JobView
	if code := raw(t, srv.URL, "POST", "/api/add/fetch", obj{"url": " https://example.org/watch?v=1\n"}, &v); code != 200 || v.Kind != "add" {
		t.Fatalf("%d %+v", code, v)
	}
	done := waitJob(t, srv.URL, v.ID)
	var out browse.Fetched
	data, _ := json.Marshal(done.Data)
	json.Unmarshal(data, &out)
	if done.State != jobDone || out.Draft == nil || out.Draft.Matched.Title != "A song" || out.Draft.Matched.Artist != "Some Band" || out.Draft.Matched.Track != "2" {
		t.Fatalf("%+v %s", done, data)
	}
	if len(dl.calls) != 1 || dl.calls[0] != "https://example.org/watch?v=1" {
		t.Errorf("the link is cleaned of what a paste brings: %v", dl.calls)
	}
	form := obj{"title": "A song", "artist": "Some Band", "album": "Record", "track": "2", "of": "9", "released": "", "genre": "", "cutStart": "", "cutEnd": ""}
	var p struct{ Path string }
	if code := raw(t, srv.URL, "POST", "/api/add/path", obj{"form": form}, &p); code != 200 || p.Path != "Some Band/Record/02 - A song.mp3" {
		t.Errorf("%d %q", code, p.Path)
	}
	if code, r := send(t, srv, "POST", "/api/add/finish", obj{"draft": out.Draft.Draft, "form": obj{"title": " "}}); code != 422 || !strings.Contains(r.Error, "cannot be empty") {
		t.Errorf("%d %+v", code, r)
	}
	code, r := send(t, srv, "POST", "/api/add/finish", obj{"draft": out.Draft.Draft, "form": form})
	if code != 200 || r.Message != `Added "A song"` || r.Reveal == "" || r.History.Undo != "addition of a track" {
		t.Fatalf("%d %+v", code, r)
	}
	if got := titleOf(t, a, r.Reveal); got != "A song" {
		t.Errorf("title %q", got)
	}
	if code, r := send(t, srv, "POST", "/api/add/finish", obj{"draft": out.Draft.Draft, "form": form}); code != 422 || !strings.Contains(r.Error, "download is gone") {
		t.Errorf("a draft is added once: %d %+v", code, r)
	}
	if code, _ := send(t, srv, "POST", "/api/undo", nil); code != 200 {
		t.Error("undo")
	}
	m, _ := a.Manifest()
	if len(m.Tracks) != len(ts) {
		t.Errorf("%d tracks after undoing the addition", len(m.Tracks))
	}
}

func TestAnAddedTrackThatIsThereAlreadyIsSaidSo(t *testing.T) {
	a, ts := seed(t, sample...)
	m, _ := a.Manifest()
	_ = m
	// the audio of the first sample track is what the "download" brings
	dl := &downloader{audio: testmp3.Silent(100)}
	srv := start(t, a, WithDownloader(dl))
	var v JobView
	raw(t, srv.URL, "POST", "/api/add/fetch", obj{"url": "https://example.org/x"}, &v)
	done := waitJob(t, srv.URL, v.ID)
	var out browse.Fetched
	data, _ := json.Marshal(done.Data)
	json.Unmarshal(data, &out)
	if done.State != jobDone || out.Duplicate == nil || out.Duplicate.ID != ts[0].ID || out.Duplicate.Title != "The Wanderer" {
		t.Errorf("%+v %s", done, data)
	}
}

func TestAddingNeedsADownloaderAndALink(t *testing.T) {
	a, _ := seed(t, sample...)
	if code, r := send(t, start(t, a), "POST", "/api/add/fetch", obj{"url": "https://example.org"}); code != 422 || !strings.Contains(r.Error, "yt-dlp is not set up") {
		t.Errorf("%d %+v", code, r)
	}
	srv := start(t, a, WithDownloader(&downloader{}))
	if code, r := send(t, srv, "POST", "/api/add/fetch", obj{"url": "  "}); code != 422 || !strings.Contains(r.Error, "Paste a link") {
		t.Errorf("%d %+v", code, r)
	}
}

// upload sends files as the page does when some are dropped on it.
func upload(t *testing.T, base string, files map[string][]byte) (int, reply) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for name, data := range files {
		part, _ := mw.CreateFormFile("files", name)
		part.Write(data)
	}
	mw.Close()
	req, _ := http.NewRequest("POST", base+"/api/ingest", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-MM", "1")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var r reply
	json.NewDecoder(res.Body).Decode(&r)
	return res.StatusCode, r
}

func TestMP3FilesAreAddedFromUploads(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a)
	code, r := upload(t, srv.URL, map[string][]byte{
		"one.mp3": testmp3.Silent(801), "two.MP3": testmp3.Silent(802), "../../evil/three.mp3": testmp3.Silent(801), "notes.txt": []byte("hi"),
	})
	if code != 200 || !strings.HasPrefix(r.Message, "Added 2 tracks · 1 already there · 1 left out: notes.txt: only MP3 files can be added") || r.Reveal == "" {
		t.Fatalf("%d %+v", code, r)
	}
	m, _ := a.Manifest()
	if len(m.Tracks) != len(ts)+2 {
		t.Errorf("%d tracks", len(m.Tracks))
	}
	if entries, _ := os.ReadDir(filepath.Join(a.Root(), ".mm")); func() bool {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "upload-") {
				return true
			}
		}
		return false
	}() {
		t.Error("the files that came are left behind")
	}
	if r.History.Undo != "addition of 2 tracks" {
		t.Errorf("%+v", r.History)
	}
	if code, r := upload(t, srv.URL, map[string][]byte{"notes.txt": []byte("hi")}); code != 422 || !strings.Contains(r.Error, "only MP3 files") {
		t.Errorf("%d %+v", code, r)
	}
}

func TestTheLibraryCheckSaysWhatIsWrong(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a)
	var v browse.DoctorView
	if code := getJSON(t, srv.URL+"/api/doctor", &v); code != 200 {
		t.Fatal(code)
	}
	var found bool
	for _, p := range v.Problems {
		if p.TrackID == ts[3].ID && p.Severity == "warning" {
			found = true
		}
	}
	if !found || v.Warnings == 0 || v.Errors != 0 {
		t.Errorf("a track that is not identified is a warning: %+v", v)
	}
}

func TestTheLibraryCheckReportsAMissingFileAsAnError(t *testing.T) {
	a, ts := seed(t, sample...)
	if err := os.Remove(a.AudioPath(ts[0].AudioSHA256)); err != nil {
		t.Fatal(err)
	}
	var v browse.DoctorView
	if code := getJSON(t, start(t, a).URL+"/api/doctor", &v); code != 200 {
		t.Fatal(code)
	}
	var found bool
	for _, p := range v.Problems {
		if p.TrackID == ts[0].ID && p.Severity == "error" && strings.Contains(p.Message, "missing") {
			found = true
		}
	}
	if !found || v.Errors != 1 {
		t.Errorf("a track whose audio is gone is an error that says it is missing: %+v", v)
	}
}

func TestTheLibraryCheckOfALibraryWithNothingWrongFindsNothing(t *testing.T) {
	a, _ := seed(t, seedTrack{"The Wanderer", "Shawn James", "Shadows", 1}, seedTrack{"Flow", "Shawn James", "Shadows", 2})
	var found map[string]json.RawMessage
	if code := getJSON(t, start(t, a).URL+"/api/doctor", &found); code != 200 {
		t.Fatal(code)
	}
	if string(found["problems"]) != "[]" || string(found["errors"]) != "0" || string(found["warnings"]) != "0" {
		t.Errorf("a library with nothing wrong reports %s", found)
	}
}

func fetched(t *testing.T, srv *httptest.Server, link string) browse.Fetched {
	t.Helper()
	var v JobView
	if code := raw(t, srv.URL, "POST", "/api/add/fetch", obj{"url": link}, &v); code != 200 {
		t.Fatalf("fetch: %d", code)
	}
	done := waitJob(t, srv.URL, v.ID)
	var out browse.Fetched
	data, _ := json.Marshal(done.Data)
	if err := json.Unmarshal(data, &out); err != nil || done.State != jobDone || out.Draft == nil {
		t.Fatalf("%+v %s %v", done, data, err)
	}
	return out
}

func addForm(released, genre string) obj {
	return obj{"title": "A song", "artist": "Some Band", "album": "", "track": "", "of": "", "released": released, "genre": genre, "cutStart": "", "cutEnd": ""}
}

func TestATrackIsAddedWithItsReleaseDate(t *testing.T) {
	a, _ := seed(t, sample...)
	srv := start(t, a, WithDownloader(&downloader{audio: testmp3.Silent(700), info: app.Info{Title: "A song", Uploader: "Some Band - Topic"}}))
	draft := fetched(t, srv, "https://example.org/watch?v=1").Draft.Draft

	code, r := send(t, srv, "POST", "/api/add/finish", obj{"draft": draft, "form": addForm("2001", "")})
	if code != 200 || r.Reveal == "" {
		t.Fatalf("%d %+v", code, r)
	}
	m, _ := a.Manifest()
	if added, _ := m.Get(r.Reveal); added.Released != "2001" {
		t.Errorf("released %q, want 2001", added.Released)
	}
}

func TestATrackIsAddedWithItsGenre(t *testing.T) {
	a, _ := seed(t, sample...)
	srv := start(t, a, WithDownloader(&downloader{audio: testmp3.Silent(700), info: app.Info{Title: "A song", Uploader: "Some Band - Topic"}}))
	draft := fetched(t, srv, "https://example.org/watch?v=1").Draft.Draft

	code, r := send(t, srv, "POST", "/api/add/finish", obj{"draft": draft, "form": addForm("", "Folk")})
	if code != 200 || r.Reveal == "" {
		t.Fatalf("%d %+v", code, r)
	}
	m, _ := a.Manifest()
	if added, _ := m.Get(r.Reveal); added.Genre != "Folk" {
		t.Errorf("genre %q, want Folk", added.Genre)
	}
}

func TestATrackIsNotAddedWithAReleaseDateThatIsNotADate(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a, WithDownloader(&downloader{audio: testmp3.Silent(700), info: app.Info{Title: "A song", Uploader: "Some Band - Topic"}}))
	draft := fetched(t, srv, "https://example.org/watch?v=1").Draft.Draft

	code, r := send(t, srv, "POST", "/api/add/finish", obj{"draft": draft, "form": addForm("soon", "")})
	if code != 422 || !strings.Contains(r.Error, "1999-05-17") {
		t.Errorf("%d %+v; want a refusal that shows the format of a date", code, r)
	}
	if m, _ := a.Manifest(); len(m.Tracks) != len(ts) {
		t.Errorf("%d tracks, want %d: a track was added anyway", len(m.Tracks), len(ts))
	}
}
