package web

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/testmp3"
)

// reply is what a change answers: the result, and how undo and redo stand.
type reply struct {
	Message string `json:"message"`
	Reveal  string `json:"reveal"`
	Same    bool   `json:"same"`
	Error   string `json:"error"`
	History struct {
		Undo string `json:"undo"`
		Redo string `json:"redo"`
		Clip string `json:"clip"`
	} `json:"history"`
	Moves map[string]string `json:"moves"`
}

// send makes a change the way the page does, and reads the answer.
func send(t *testing.T, srv *httptest.Server, method, path string, body any) (int, reply) {
	t.Helper()
	var in io.Reader
	switch b := body.(type) {
	case nil:
	case []byte:
		in = bytes.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		in = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, srv.URL+path, in)
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

func titleOf(t *testing.T, a *app.App, id string) string {
	t.Helper()
	m, err := a.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	tr, _ := m.Get(id)
	return tr.Title
}

type obj = map[string]any

func TestAFieldIsEditedOverHTTPAndUndone(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a)
	code, r := send(t, srv, "PATCH", "/api/tracks/"+ts[0].ID, obj{"field": "title", "text": "Wanderer II"})
	if code != 200 || r.Message != "Saved" || r.Reveal != ts[0].ID || r.History.Undo != "change of title" {
		t.Fatalf("%d %+v", code, r)
	}
	if got := titleOf(t, a, ts[0].ID); got != "Wanderer II" {
		t.Errorf("title %q", got)
	}
	var lib struct {
		History struct{ Undo string } `json:"history"`
	}
	getJSON(t, srv.URL+"/api/library", &lib)
	if lib.History.Undo != "change of title" {
		t.Errorf("the library does not say what undo does: %+v", lib.History)
	}
	if code, r := send(t, srv, "POST", "/api/undo", nil); code != 200 || r.Message != "Undid: change of title" || r.History.Redo != "change of title" {
		t.Errorf("%d %+v", code, r)
	}
	if got := titleOf(t, a, ts[0].ID); got != "The Wanderer" {
		t.Errorf("after undo %q", got)
	}
	if code, _ := send(t, srv, "POST", "/api/redo", nil); code != 200 || titleOf(t, a, ts[0].ID) != "Wanderer II" {
		t.Errorf("redo %d", code)
	}
}

func TestWhatCannotBeDoneIsA422WithTheReason(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a)
	code, r := send(t, srv, "PATCH", "/api/tracks/"+ts[0].ID, obj{"field": "title", "text": " "})
	if code != 422 || !strings.Contains(r.Error, "cannot be empty") {
		t.Errorf("%d %+v", code, r)
	}
	if code, _ := send(t, srv, "PATCH", "/api/tracks/"+ts[0].ID, obj{"field": "title", "text": "x", "extra": 1}); code != 400 {
		t.Errorf("a field nobody knows: %d", code)
	}
	if code, _ := send(t, srv, "PATCH", "/api/tracks/"+ts[0].ID, []byte("{nope")); code != 400 {
		t.Errorf("not JSON: %d", code)
	}
	if code, _ := send(t, srv, "PATCH", "/api/tracks/nope", obj{"field": "title", "text": "x"}); code != 422 {
		t.Errorf("a track that is not there: %d", code)
	}
	if code, _ := send(t, srv, "POST", "/api/move", obj{"id": ts[0].ID, "dir": 2}); code != 400 {
		t.Errorf("a move of two: %d", code)
	}
}

func TestThePathShownWhileTyping(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a)
	req, _ := http.NewRequest("POST", srv.URL+"/api/tracks/"+ts[0].ID+"/path", strings.NewReader(`{"field":"title","text":"Wanderer II"}`))
	req.Header.Set("X-MM", "1")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct{ Path string }
	json.NewDecoder(res.Body).Decode(&out)
	if res.StatusCode != 200 || out.Path != "Shawn James/Shadows/01 - Wanderer II.mp3" {
		t.Errorf("%d %q", res.StatusCode, out.Path)
	}
	if code, _ := send(t, srv, "POST", "/api/tracks/nope/path", obj{"field": "title", "text": "x"}); code != 404 {
		t.Errorf("a track that is not there: %d", code)
	}
}

func TestNudgesAndClearsOverHTTP(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a)
	for i := 0; i < 2; i++ {
		if code, _ := send(t, srv, "POST", "/api/tracks/"+ts[0].ID+"/nudge", obj{"field": "volume", "more": 1}); code != 200 {
			t.Fatal(code)
		}
	}
	m, _ := a.Manifest()
	if tr, _ := m.Get(ts[0].ID); tr.GainSteps != 2 {
		t.Errorf("steps %d", tr.GainSteps)
	}
	if code, _ := send(t, srv, "POST", "/api/tracks/"+ts[0].ID+"/clear", obj{"field": "volume"}); code != 200 {
		t.Fatal(code)
	}
	m, _ = a.Manifest()
	if tr, _ := m.Get(ts[0].ID); tr.GainSteps != 0 {
		t.Errorf("steps %d", tr.GainSteps)
	}
}

// bigPNG is a header that says 8000×8000, which is more pixels than a cover may have.
func bigPNG() []byte {
	var b bytes.Buffer
	b.WriteString("\x89PNG\r\n\x1a\n")
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], 8000)
	binary.BigEndian.PutUint32(ihdr[4:], 8000)
	ihdr[8], ihdr[9] = 8, 2
	binary.Write(&b, binary.BigEndian, uint32(len(ihdr)))
	chunk := append([]byte("IHDR"), ihdr...)
	b.Write(chunk)
	binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return b.Bytes()
}

func TestCoversAreUploadedCheckedAndRemoved(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a)
	one := "/api/cover?ids=" + ts[0].ID
	if code, r := send(t, srv, "PUT", one, testmp3.JPEG); code != 200 || r.Message != "Saved" {
		t.Fatalf("%d %+v", code, r)
	}
	res, err := http.Get(srv.URL + "/api/tracks/" + ts[0].ID + "/cover")
	if err != nil || res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/jpeg" {
		t.Errorf("%v %v", err, res)
	}
	res.Body.Close()
	if code, _ := send(t, srv, "PUT", one, []byte("not a picture")); code != 415 {
		t.Errorf("text as a cover: %d", code)
	}
	if code, _ := send(t, srv, "PUT", one, bigPNG()); code != 413 {
		t.Errorf("a bomb as a cover: %d", code)
	}
	if code, _ := send(t, srv, "PUT", "/api/cover", testmp3.JPEG); code != 400 {
		t.Errorf("a cover for nobody: %d", code)
	}
	if code, r := send(t, srv, "DELETE", one, nil); code != 200 || r.Message != "Saved" {
		t.Errorf("%d %+v", code, r)
	}
	// every track of an album at once
	group := "/api/cover?group=" + "album%1FShawn%20James%1FShadows"
	if code, r := send(t, srv, "PUT", group, testmp3.PNG); code != 200 || r.Message != "Changed 2 tracks" {
		t.Errorf("%d %+v", code, r)
	}
	m, _ := a.Manifest()
	for _, tr := range m.Tracks[:2] {
		if tr.CoverSHA256 == "" {
			t.Errorf("%s has no cover", tr.Title)
		}
	}
	// a picture pasted from the desktop says where it came from
	if code, r := send(t, srv, "PUT", "/api/cover?paste=1&name=front.jpg&ids="+ts[2].ID, testmp3.JPEG2); code != 200 || !strings.Contains(r.Message, "front.jpg") {
		t.Errorf("%d %+v", code, r)
	}
}

func TestGroupsBulkAndPlaylistsOverHTTP(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a)
	key := "artist\x1fShawn James"
	if code, r := send(t, srv, "PATCH", "/api/group", obj{"key": key, "field": "artist", "text": "S. James"}); code != 200 || r.Message != "Changed 2 tracks" || r.Moves[key] != "artist\x1fS. James" {
		t.Fatalf("%d %+v", code, r)
	}
	var hint struct{ Hint string }
	post := func(path string, body any, into any) int {
		raw, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", srv.URL+path, bytes.NewReader(raw))
		req.Header.Set("X-MM", "1")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		json.NewDecoder(res.Body).Decode(into)
		return res.StatusCode
	}
	if code := post("/api/group/hint", obj{"key": "artist\x1fS. James", "field": "artist", "text": "داود القَيسي"}, &hint); code != 200 || !strings.HasPrefix(hint.Hint, "joins the artist") {
		t.Errorf("%d %q", code, hint.Hint)
	}

	ids := []string{ts[0].ID, ts[1].ID}
	var p struct {
		Changed int
		Summary string
		Error   string
	}
	if code := post("/api/bulk/preview", obj{"ids": ids, "values": obj{"genre": "Folk"}}, &p); code != 200 || p.Changed != 2 || p.Summary != "Genre → Folk" {
		t.Errorf("%d %+v", code, p)
	}
	if code, r := send(t, srv, "POST", "/api/bulk", obj{"ids": ids, "values": obj{"genre": "Folk"}}); code != 200 || r.Message != "Changed 2 tracks" {
		t.Errorf("%d %+v", code, r)
	}
	if code, r := send(t, srv, "POST", "/api/bulk", obj{"ids": ids, "values": obj{"genre": "Folk"}}); code != 422 || !strings.Contains(r.Error, "Nothing to change") {
		t.Errorf("%d %+v", code, r)
	}

	if code, r := send(t, srv, "POST", "/api/playlists", obj{"name": "Road", "ids": ids}); code != 200 || !strings.HasPrefix(r.Message, "Made Road with 2 tracks") {
		t.Fatalf("%d %+v", code, r)
	}
	if code, r := send(t, srv, "POST", "/api/playlists/add", obj{"name": "Road", "ids": []string{ts[2].ID}}); code != 200 || !strings.Contains(r.Message, "Added") {
		t.Errorf("%d %+v", code, r)
	}
	if code, r := send(t, srv, "POST", "/api/move", obj{"id": ts[0].ID, "dir": 1, "playlist": "Road"}); code != 200 || r.Reveal != ts[0].ID {
		t.Errorf("%d %+v", code, r)
	}
	if code, _ := send(t, srv, "POST", "/api/playlists/takeout", obj{"name": "Road", "ids": []string{ts[2].ID}}); code != 200 {
		t.Errorf("take out %d", code)
	}
	if code, _ := send(t, srv, "POST", "/api/playlists/delete", obj{"name": "Road"}); code != 200 {
		t.Errorf("delete %d", code)
	}
	m, _ := a.Manifest()
	if len(m.Playlists) != 0 {
		t.Errorf("%+v", m.Playlists)
	}
	if code, _ := send(t, srv, "POST", "/api/undo", nil); code != 200 {
		t.Errorf("undo %d", code)
	}
	m, _ = a.Manifest()
	if len(m.Playlists) != 1 {
		t.Errorf("undo brings the playlist back: %+v", m.Playlists)
	}
}

func TestCopyingPastingRemovingAndRenumberingOverHTTP(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a)
	var c []struct{ Key, Label string }
	getJSON(t, srv.URL+"/api/tracks/"+ts[0].ID+"/copyable", &c)
	if len(c) == 0 {
		t.Errorf("nothing can be copied: %+v", c)
	}
	if code, _ := getStatus(srv.URL + "/api/tracks/nope/copyable"); code != 404 {
		t.Errorf("%d", code)
	}
	if code, r := send(t, srv, "POST", "/api/copy", obj{"id": ts[0].ID, "attrs": []string{"artist", "album"}}); code != 200 || r.History.Clip != "artist and album" {
		t.Fatalf("%d %+v", code, r)
	}
	if code, r := send(t, srv, "POST", "/api/paste", obj{"ids": []string{ts[2].ID}}); code != 200 || !strings.HasPrefix(r.Message, "Pasted") {
		t.Errorf("%d %+v", code, r)
	}
	if code, r := send(t, srv, "POST", "/api/paste", obj{"group": "album\x1fnobody\x1fnothing"}); code != 422 {
		t.Errorf("%d %+v", code, r)
	}
	if code, r := send(t, srv, "POST", "/api/renumber", obj{"key": "album\x1fShawn James\x1fShadows"}); code != 200 {
		t.Errorf("%d %+v", code, r)
	}
	if code, r := send(t, srv, "POST", "/api/remove", obj{"ids": []string{ts[3].ID}}); code != 200 || !strings.HasPrefix(r.Message, "Removed") {
		t.Errorf("%d %+v", code, r)
	}
	m, _ := a.Manifest()
	if len(m.Tracks) != len(ts)-1 {
		t.Errorf("%d tracks", len(m.Tracks))
	}
}

func getStatus(url string) (int, error) {
	res, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	res.Body.Close()
	return res.StatusCode, nil
}

func TestAudioToHearACut(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("no ffmpeg")
	}
	a, ts := seed(t, sample...)
	srv := start(t, a)
	req, _ := http.NewRequest("GET", srv.URL+"/api/tracks/"+ts[0].ID+"/preview?edge=start&window=2", nil)
	req.Header.Set("Range", "bytes=0-99")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusPartialContent || len(body) != 100 || res.Header.Get("Content-Type") != "audio/mpeg" || res.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("%d %d bytes %v", res.StatusCode, len(body), res.Header)
	}
	if code, _ := getStatus(srv.URL + "/api/tracks/" + ts[0].ID + "/preview?field=fadeOut"); code != 200 {
		t.Errorf("a fade is heard by its field: %d", code)
	}
	if code, _ := getStatus(srv.URL + "/api/tracks/" + ts[0].ID + "/preview?field=title"); code != 400 {
		t.Errorf("a title is not heard: %d", code)
	}
	for _, bad := range []string{"?edge=middle", "?window=0", "?window=x", "?window=999999"} {
		if code, _ := getStatus(srv.URL + "/api/tracks/" + ts[0].ID + "/preview" + bad); code != 400 {
			t.Errorf("%s: %d", bad, code)
		}
	}
	if code, _ := getStatus(srv.URL + "/api/tracks/nope/preview"); code != 404 {
		t.Errorf("a track that is not there: %d", code)
	}
}
