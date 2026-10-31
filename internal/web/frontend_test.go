package web

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

func fetch(t *testing.T, url string) (*http.Response, string) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res, string(body)
}

func TestWithoutAFrontendTheServerSays503AndWhatToRun(t *testing.T) {
	a, _ := seed(t, sample...)
	srv := start(t, a, WithFrontend(fstest.MapFS{}))
	res, body := fetch(t, srv.URL+"/")
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status %d, want 503", res.StatusCode)
	}
	if !strings.Contains(body, "pnpm --dir web run build") {
		t.Errorf("the page does not say what to run: %q", body)
	}
	if !strings.Contains(res.Header.Get("Content-Type"), "text/html") {
		t.Errorf("content type %q", res.Header.Get("Content-Type"))
	}
}

func TestWithFrontendServesItsIndex(t *testing.T) {
	a, _ := seed(t, sample...)
	srv := start(t, a, WithFrontend(fstest.MapFS{
		"index.html":      {Data: []byte("<title>new</title>")},
		"assets/app-1.js": {Data: []byte("export {}")},
		"favicon.svg":     {Data: []byte("<svg/>")},
	}))
	res, body := fetch(t, srv.URL+"/")
	if res.StatusCode != http.StatusOK || !strings.Contains(body, "<title>new</title>") {
		t.Errorf("status %d, body %q", res.StatusCode, body)
	}
	if got := res.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control %q, want no-cache", got)
	}
	res, body = fetch(t, srv.URL+"/assets/app-1.js")
	if res.StatusCode != http.StatusOK || body != "export {}" {
		t.Errorf("asset: status %d, body %q", res.StatusCode, body)
	}
	if got := res.Header.Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("a hashed asset should be cached for good, Cache-Control %q", got)
	}
	res, _ = fetch(t, srv.URL+"/favicon.svg")
	if res.StatusCode != http.StatusOK {
		t.Errorf("favicon: status %d", res.StatusCode)
	}
	if got := res.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("a file outside assets/ must be revalidated, Cache-Control %q", got)
	}
}

func TestAPIRoutesStillWinOverTheFrontend(t *testing.T) {
	a, _ := seed(t, sample...)
	for name, fsys := range map[string]fstest.MapFS{
		"built":     {"index.html": {Data: []byte("<title>new</title>")}, "api/info": {Data: []byte("not json")}},
		"not built": {},
	} {
		srv := start(t, a, WithFrontend(fsys))
		var info InfoView
		if code := getJSON(t, srv.URL+"/api/info", &info); code != http.StatusOK {
			t.Errorf("%s: /api/info status %d", name, code)
		}
		if info.Root == "" {
			t.Errorf("%s: /api/info did not answer with the info: %+v", name, info)
		}
	}
}
