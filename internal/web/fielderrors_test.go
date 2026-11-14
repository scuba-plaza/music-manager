package web

import (
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/testmp3"
	"music-manager/internal/web/webtest"
)

func addedFrom(t *testing.T) (obj, string, func(form obj) (int, reply)) {
	t.Helper()
	a, _ := seed(t, sample...)
	srv := start(t, a, WithDownloader(&webtest.FakeDownloader{Audio: testmp3.Silent(700), Info: app.Info{Title: "A song", Uploader: "Some Band - Topic"}}))
	draft := fetched(t, srv, "https://example.org/watch?v=1").Draft.Draft
	return addForm("", ""), draft, func(form obj) (int, reply) {
		return send(t, srv, "POST", "/api/add/finish", obj{"draft": draft, "form": form})
	}
}

func TestAnAddFormThatIsWrongSaysWhichFieldsAreWrong(t *testing.T) {
	form, _, finish := addedFrom(t)
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
}

func TestCutsThatLeaveNothingOfTheTrackAreRefusedOnTheCut(t *testing.T) {
	form, _, finish := addedFrom(t)
	form["cutEnd"] = "10:00"

	code, r := finish(form)

	if code != 422 || r.Fields["cutStart"] == "" || r.Fields["cutEnd"] == "" || len(r.Fields) != 2 {
		t.Errorf("%d %+v", code, r)
	}
}

func TestAnErrorThatIsNotAboutAFieldNamesNone(t *testing.T) {
	form, draft, finish := addedFrom(t)
	if code, r := finish(form); code != 200 {
		t.Fatalf("%d %+v", code, r)
	}
	_ = draft

	code, r := finish(form)

	if code != 422 || r.Error == "" || len(r.Fields) != 0 {
		t.Errorf("%d %+v", code, r)
	}
}

func TestAFieldEditedInPlaceSaysWhichFieldIsWrong(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a)

	code, r := send(t, srv, "PATCH", "/api/tracks/"+ts[0].ID, obj{"field": "released", "text": "soon"})

	if code != 422 || r.Fields["released"] == "" {
		t.Errorf("%d %+v", code, r)
	}
}

func TestABulkFormThatIsWrongSaysWhichFieldsAreWrong(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a)
	ids := []string{ts[0].ID, ts[1].ID}

	code, r := send(t, srv, "POST", "/api/bulk", obj{"ids": ids, "values": obj{"artist": "-", "of": "many"}})

	if code != 422 || r.Fields["artist"] == "" || r.Fields["of"] == "" || len(r.Fields) != 2 {
		t.Errorf("%d %+v", code, r)
	}
}
