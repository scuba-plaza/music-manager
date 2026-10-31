package web

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/browse"
)

func noNullCollections(t *testing.T, label string, body []byte, typ reflect.Type) {
	t.Helper()
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("%s: %v: %s", label, err, body)
	}
	for _, path := range nullCollections(v, typ, typ.String()) {
		t.Errorf("%s: %s is null", label, path)
	}
}

func nullCollections(v any, typ reflect.Type, path string) []string {
	switch typ.Kind() {
	case reflect.Pointer:
		if v == nil {
			return nil
		}
		return nullCollections(v, typ.Elem(), path)
	case reflect.Slice, reflect.Map:
		if v == nil {
			return []string{path}
		}
		var out []string
		switch items := v.(type) {
		case []any:
			for i, item := range items {
				out = append(out, nullCollections(item, typ.Elem(), fmt.Sprintf("%s[%d]", path, i))...)
			}
		case map[string]any:
			for k, item := range items {
				out = append(out, nullCollections(item, typ.Elem(), path+"["+k+"]")...)
			}
		}
		return out
	case reflect.Array:
		var out []string
		items, _ := v.([]any)
		for i, item := range items {
			out = append(out, nullCollections(item, typ.Elem(), fmt.Sprintf("%s[%d]", path, i))...)
		}
		return out
	case reflect.Struct:
		fields, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		return nullFields(fields, typ, path)
	}
	return nil
}

func nullFields(fields map[string]any, typ reflect.Type, path string) []string {
	var out []string
	for i := range typ.NumField() {
		f := typ.Field(i)
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		if f.Anonymous && name == "" {
			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				out = append(out, nullFields(fields, ft, path)...)
				continue
			}
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		value, present := fields[name]
		if !present {
			continue
		}
		out = append(out, nullCollections(value, f.Type, path+"."+name)...)
	}
	return out
}

func TestTheHelperFindsANullListByItsPath(t *testing.T) {
	got := nullCollections(map[string]any{"view": map[string]any{"rows": []any{}, "playlists": nil}},
		reflect.TypeFor[LibraryResponse](), "LibraryResponse")
	if len(got) != 1 || got[0] != "LibraryResponse.view.playlists" {
		t.Errorf("%v", got)
	}
}

func readRaw(t *testing.T, base, method, path string, body any) []byte {
	t.Helper()
	var out json.RawMessage
	if code := sendInto(t, base, method, path, body, &out); code != 200 {
		t.Fatalf("%s %s: %d %s", method, path, code, out)
	}
	return out
}

func jobData(t *testing.T, base string, started []byte) []byte {
	t.Helper()
	var v JobView
	if err := json.Unmarshal(started, &v); err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, base, v.ID)
	if done.State != jobDone {
		t.Fatalf("%+v", done)
	}
	data, err := json.Marshal(done.Data)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestNoListInAnAnswerIsNull(t *testing.T) {
	empty, _ := seed(t)
	full, ts := seed(t, sample...)
	if err := full.CreatePlaylist("Empty"); err != nil {
		t.Fatal(err)
	}
	if err := full.CreatePlaylist("Road", ts[0].ID, ts[1].ID); err != nil {
		t.Fatal(err)
	}
	source := WithSource(database{[]app.Candidate{{Title: "Found", Artist: "Someone", Album: "X", TrackNumber: 1, TrackTotal: 1, Score: 99}}})
	for _, lib := range []struct {
		name string
		app  *app.App
	}{{"empty", empty}, {"full", full}} {
		srv := start(t, lib.app, source, WithWolfgang())
		check := func(method, path string, body any, typ reflect.Type) []byte {
			raw := readRaw(t, srv.URL, method, path, body)
			noNullCollections(t, lib.name+" "+method+" "+path, raw, typ)
			return raw
		}
		check("GET", "/api/info", nil, reflect.TypeFor[InfoView]())
		check("GET", "/api/profiles", nil, reflect.TypeFor[[]ProfileView]())
		check("GET", "/api/library", nil, reflect.TypeFor[LibraryResponse]())
		check("GET", "/api/library?q=nothing-matches-this", nil, reflect.TypeFor[LibraryResponse]())
		check("GET", "/api/library?sort=title", nil, reflect.TypeFor[LibraryResponse]())
		check("GET", "/api/doctor", nil, reflect.TypeFor[browse.DoctorView]())
		check("GET", "/api/sync/targets", nil, reflect.TypeFor[SyncTargetsView]())
		check("GET", "/api/jobs", nil, reflect.TypeFor[[]JobView]())
		plan := check("POST", "/api/sync/plan", obj{"folder": t.TempDir()}, reflect.TypeFor[JobView]())
		var made SyncData
		data := jobData(t, srv.URL, plan)
		noNullCollections(t, lib.name+" sync plan", data, reflect.TypeFor[SyncData]())
		if err := json.Unmarshal(data, &made); err != nil || made.Plan == nil {
			t.Fatalf("%v %s", err, data)
		}
		check("POST", "/api/sync/"+made.Plan.Plan+"/prune", obj{"prune": true}, reflect.TypeFor[browse.PlanView]())
		if lib.app == empty {
			continue
		}
		for _, tr := range ts {
			for _, query := range []string{"", "?profile=wolfgang-wmp01", "?profile=generic"} {
				check("GET", "/api/tracks/"+tr.ID+query, nil, reflect.TypeFor[browse.TrackView]())
			}
			check("GET", "/api/tracks/"+tr.ID+"/copyable", nil, reflect.TypeFor[[]browse.CopyChoice]())
		}
		for _, key := range []string{
			"needs", "playlists", "playlist\x1fEmpty", "playlist\x1fRoad",
			"artist\x1fShawn James", "album\x1fShawn James\x1fShadows", "singles\x1fداود القَيسي",
		} {
			check("GET", "/api/peek?key="+url.QueryEscape(key), nil, reflect.TypeFor[browse.PeekView]())
		}
		check("POST", "/api/tracks/"+ts[3].ID+"/candidates", obj{}, reflect.TypeFor[browse.SearchView]())
		identify := check("POST", "/api/identify-all", obj{}, reflect.TypeFor[JobView]())
		noNullCollections(t, "identify-all", jobData(t, srv.URL, identify), reflect.TypeFor[browse.IdentifyProgress]())
	}
}

func TestNoListIsNullForALibraryWithoutPlaylists(t *testing.T) {
	a, ts := seed(t, sample...)
	srv := start(t, a, WithWolfgang())
	check := func(path string, typ reflect.Type) {
		noNullCollections(t, path, readRaw(t, srv.URL, "GET", path, nil), typ)
	}
	check("/api/library", reflect.TypeFor[LibraryResponse]())
	check("/api/peek?key=needs", reflect.TypeFor[browse.PeekView]())
	for _, tr := range ts {
		check("/api/tracks/"+tr.ID+"?profile=wolfgang-wmp01", reflect.TypeFor[browse.TrackView]())
	}
}
