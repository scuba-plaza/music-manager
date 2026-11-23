package web

import (
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/testmp3"
	"music-manager/internal/web/webtest"
)

// addedFrom fetches a draft, and gives the form it is added with and a way to
// add it, and the library it goes to.
func addedFrom(t *testing.T) (*app.App, obj, func(form obj) (int, reply)) {
	t.Helper()
	a, _ := seed(t, sample...)
	srv := start(t, a, WithDownloader(&webtest.FakeDownloader{Audio: testmp3.Silent(700), Info: app.Info{Title: "A song", Uploader: "Some Band - Topic"}}))
	draft := fetched(t, srv, "https://example.org/watch?v=1").Draft.Draft
	return a, addForm("", ""), func(form obj) (int, reply) {
		return send(t, srv, "POST", "/api/add/finish", obj{"draft": draft, "form": form})
	}
}

func TestAnAddFormThatIsWrongSaysWhichFieldsAreWrong(t *testing.T) {
	a, form, finish := addedFrom(t)
	before, _ := a.Manifest()
	form["title"] = " "
	form["released"] = "soon"
	form["track"] = "two"

	code, r := finish(form)

	if code != 422 {
		t.Fatalf("%d %+v", code, r)
	}
	for _, key := range []string{"title", "released", "track"} {
		if r.Fields[key] == "" {
			t.Errorf("no message for %s: %v", key, r.Fields)
		}
	}
	if len(r.Fields) != 3 {
		t.Errorf("%v", r.Fields)
	}
	if !strings.HasPrefix(r.Fields["released"], "Release date") || !strings.Contains(r.Fields["released"], "1999-05-17") {
		t.Errorf("a message under a field starts with a capital and shows the format of a date: %q", r.Fields["released"])
	}
	if after, _ := a.Manifest(); len(after.Tracks) != len(before.Tracks) {
		t.Errorf("%d tracks, want %d: a track was added anyway", len(after.Tracks), len(before.Tracks))
	}
}

func TestCutsThatLeaveNothingOfTheTrackAreRefusedOnTheCut(t *testing.T) {
	_, form, finish := addedFrom(t)
	form["cutEnd"] = "10:00"

	code, r := finish(form)

	if code != 422 || r.Fields["cutStart"] == "" || r.Fields["cutEnd"] == "" || len(r.Fields) != 2 {
		t.Errorf("%d %+v", code, r)
	}
}

func TestTheCheckOfAnAddFormNamesTheWrongFieldsAndSaysWhereTheFileWouldGo(t *testing.T) {
	a, _ := seed(t, sample...)
	srv := start(t, a, WithDownloader(&webtest.FakeDownloader{Audio: testmp3.Silent(700)}))
	var wrong struct {
		Path   string
		Fields map[string]string
	}

	form := addForm("soon", "")
	form["track"] = "x"
	if code := sendInto(t, srv.URL, "POST", "/api/add/path", obj{"form": form}, &wrong); code != 200 {
		t.Fatalf("%d", code)
	}

	if wrong.Fields["released"] == "" || wrong.Fields["track"] == "" || len(wrong.Fields) != 2 || wrong.Path == "" {
		t.Errorf("%+v", wrong)
	}

	var right struct {
		Path   string
		Fields map[string]string
	}
	if code := sendInto(t, srv.URL, "POST", "/api/add/path", obj{"form": addForm("2001", "")}, &right); code != 200 {
		t.Fatalf("%d", code)
	}
	if len(right.Fields) != 0 || right.Path != "Some Band/A song.mp3" {
		t.Errorf("%+v", right)
	}
}
