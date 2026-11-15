package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/browse"
	"music-manager/internal/testmp3"
	"music-manager/internal/web/webtest"
)

func draftWithCover(t *testing.T) (*app.App, string, string, browse.DraftView) {
	t.Helper()
	a, _ := seed(t, sample...)
	srv := start(t, a, WithDownloader(&webtest.FakeDownloader{Audio: testmp3.Silent(700), Info: app.Info{Title: "A song", Artist: "Some Band", Source: app.FromMetadata}, Cover: testmp3.JPEG}))
	return a, srv.URL, "", *fetched(t, srv, "https://example.org/watch?v=1").Draft
}

func getBytes(t *testing.T, url string) (int, string, []byte) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res.StatusCode, res.Header.Get("Content-Type"), data
}

func putBytes(t *testing.T, method, url string, body []byte) (int, map[string]string) {
	t.Helper()
	req, _ := http.NewRequest(method, url, bytes.NewReader(body))
	req.Header.Set("X-MM", "1")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]string
	data, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(data, &out)
	return res.StatusCode, out
}

func TestTheCoverYouTubeGaveIsOnTheDraftAndServedAsAPicture(t *testing.T) {
	_, base, _, draft := draftWithCover(t)

	if draft.Cover == "" || draft.Note == "" {
		t.Fatalf("%+v", draft)
	}
	code, kind, data := getBytes(t, base+"/api/add/"+draft.Draft+"/cover")
	if code != 200 || kind != "image/jpeg" || !bytes.Equal(data, testmp3.JPEG) {
		t.Errorf("%d %q %d bytes", code, kind, len(data))
	}
}

func TestTheCoverOfADraftIsReplacedAndTakenAway(t *testing.T) {
	_, base, _, draft := draftWithCover(t)

	code, replaced := putBytes(t, "PUT", base+"/api/add/"+draft.Draft+"/cover", testmp3.PNG)

	if code != 200 || replaced["cover"] == "" || replaced["cover"] == draft.Cover {
		t.Fatalf("%d %v", code, replaced)
	}
	if _, kind, data := getBytes(t, base+"/api/add/"+draft.Draft+"/cover"); kind != "image/png" || !bytes.Equal(data, testmp3.PNG) {
		t.Errorf("%q %d bytes", kind, len(data))
	}
	if code, _ := putBytes(t, "PUT", base+"/api/add/"+draft.Draft+"/cover", []byte("not a picture")); code != http.StatusUnsupportedMediaType {
		t.Errorf("a text cover: %d", code)
	}

	code, gone := putBytes(t, "DELETE", base+"/api/add/"+draft.Draft+"/cover", nil)

	if code != 200 || gone["cover"] != "" {
		t.Errorf("%d %v", code, gone)
	}
	if code, _, _ := getBytes(t, base+"/api/add/"+draft.Draft+"/cover"); code != http.StatusNotFound {
		t.Errorf("a draft with no cover: %d", code)
	}
}

func TestADraftThatIsGoneHasNoCoverToServeChangeOrTakeAway(t *testing.T) {
	_, base, _, _ := draftWithCover(t)

	if code, _, _ := getBytes(t, base+"/api/add/nope/cover"); code != http.StatusNotFound {
		t.Errorf("get %d", code)
	}
	if code, _ := putBytes(t, "PUT", base+"/api/add/nope/cover", testmp3.PNG); code != http.StatusNotFound {
		t.Errorf("put %d", code)
	}
	if code, _ := putBytes(t, "DELETE", base+"/api/add/nope/cover", nil); code != http.StatusNotFound {
		t.Errorf("delete %d", code)
	}
}

func TestTheTrackIsAddedWithTheCoverTheDraftHasThen(t *testing.T) {
	a, base, _, draft := draftWithCover(t)
	if code, _ := putBytes(t, "PUT", base+"/api/add/"+draft.Draft+"/cover", testmp3.PNG); code != 200 {
		t.Fatal(code)
	}
	var r reply

	code := sendInto(t, base, "POST", "/api/add/finish", obj{"draft": draft.Draft, "form": addForm("", "")}, &r)

	if code != 200 || r.Reveal == "" {
		t.Fatalf("%d %+v", code, r)
	}
	m, _ := a.Manifest()
	added, _ := m.Get(r.Reveal)
	if added.CoverSHA256 == "" || added.CoverSHA256 == draft.Cover {
		t.Errorf("cover %q, want the replacement of %q", added.CoverSHA256, draft.Cover)
	}
}
