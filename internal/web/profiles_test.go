package web

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"music-manager/internal/manifest"
)

func profilesOf(t *testing.T, base string) []manifest.Profile {
	t.Helper()
	var lib LibraryResponse
	if code := getJSON(t, base+"/api/library", &lib); code != 200 {
		t.Fatal(code)
	}
	return lib.Profiles
}

func playerForm(name string) obj {
	return obj{
		"id": "", "name": name, "latin_names": false, "component_max": 100, "stem_max": 0, "tag_width": 0, "latin_genre": false,
		"cover_format": "keep", "cover_max_edge": 0, "cover_quality": 0, "single_covers": false, "replaygain": true,
		"playlist_ext": ".m3u8", "playlist_style": "relative",
	}
}

func TestTheLibraryCarriesTheStoredFormats(t *testing.T) {
	a, _ := seed(t)
	srv := start(t, a)

	got := profilesOf(t, srv.URL)

	if len(got) != 1 || got[0].ID != "wolfgang-wmp01" || got[0].Name != "Wolfgang WMP01" {
		t.Errorf("%+v", got)
	}
}

func TestAFormatIsMadeChangedAndDeletedOverHTTP(t *testing.T) {
	a, _ := seed(t)
	srv := start(t, a)

	var made manifest.Profile
	if code := sendInto(t, srv.URL, "POST", "/api/profiles", playerForm("My Player"), &made); code != 200 || made.ID != "my-player" {
		t.Fatalf("%d %+v", code, made)
	}
	if got := profilesOf(t, srv.URL); !slices.ContainsFunc(got, func(p manifest.Profile) bool { return p.ID == "my-player" }) {
		t.Errorf("%+v", got)
	}

	edit := playerForm("Car")
	edit["component_max"] = 50
	var changed manifest.Profile
	if code := sendInto(t, srv.URL, "PUT", "/api/profiles/my-player", edit, &changed); code != 200 || changed.Name != "Car" || changed.ComponentMax != 50 || changed.ID != "my-player" {
		t.Errorf("%d %+v", code, changed)
	}

	if code, _ := send(t, srv, "DELETE", "/api/profiles/my-player", nil); code != 200 && code != 204 {
		t.Errorf("delete %d", code)
	}
	if got := profilesOf(t, srv.URL); slices.ContainsFunc(got, func(p manifest.Profile) bool { return p.ID == "my-player" }) {
		t.Errorf("still there: %+v", got)
	}
}

func TestAFormatThatIsWrongSaysWhichSettingsAre(t *testing.T) {
	a, _ := seed(t)
	srv := start(t, a)
	form := playerForm("")
	form["component_max"] = 5
	form["playlist_ext"] = ".pls"

	code, r := send(t, srv, "POST", "/api/profiles", form)

	if code != 422 || r.Fields["name"] == "" || r.Fields["component_max"] == "" || r.Fields["playlist_ext"] == "" || len(r.Fields) != 3 {
		t.Errorf("%d %+v", code, r)
	}
}

func TestTwoFormatsMayNotShareANameAndNoMoreThanEightCanBeMade(t *testing.T) {
	a, _ := seed(t)
	srv := start(t, a)

	if code, r := send(t, srv, "POST", "/api/profiles", playerForm("wolfgang wmp01")); code != 422 || r.Fields["name"] == "" {
		t.Errorf("%d %+v", code, r)
	}
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		if code, r := send(t, srv, "POST", "/api/profiles", playerForm(name)); code != 200 {
			t.Fatalf("%s: %d %+v", name, code, r)
		}
	}
	code, r := send(t, srv, "POST", "/api/profiles", playerForm("i"))
	if code != 422 || r.Error == "" || len(r.Fields) != 0 {
		t.Errorf("%d %+v", code, r)
	}
}

func TestAFormatThatIsGoneIs404(t *testing.T) {
	a, _ := seed(t)
	srv := start(t, a)

	if code, _ := send(t, srv, "PUT", "/api/profiles/nope", playerForm("x")); code != http.StatusNotFound {
		t.Errorf("put %d", code)
	}
	if code, _ := send(t, srv, "DELETE", "/api/profiles/nope", nil); code != http.StatusNotFound {
		t.Errorf("delete %d", code)
	}
}

func TestChangingAFormatAfterAPlanDoesNotChangeWhatThePlanWrites(t *testing.T) {
	a, _ := seed(t, sample...)
	srv := start(t, a)
	var made manifest.Profile
	form := playerForm("Short Stems")
	form["stem_max"] = 8
	if code := sendInto(t, srv.URL, "POST", "/api/profiles", form, &made); code != 200 {
		t.Fatal(code)
	}
	dest := filepath.Join(t.TempDir(), "player")
	_, d := planned(t, srv.URL, obj{"folder": dest, "profile": made.ID})

	edit := playerForm("Short Stems")
	edit["stem_max"] = 0
	if code, _ := send(t, srv, "PUT", "/api/profiles/"+made.ID, edit); code != 200 {
		t.Fatal(code)
	}
	code, job, r := applied(t, srv.URL, d.Plan.Plan, obj{})

	if code != 200 || job.State != jobDone {
		t.Fatalf("%d %+v %+v", code, job, r)
	}
	var mp3s int
	filepath.WalkDir(dest, func(p string, e fs.DirEntry, err error) error {
		if stem, ok := strings.CutSuffix(e.Name(), ".mp3"); ok && err == nil {
			mp3s++
			if n := utf8.RuneCountInString(stem); n > 8 {
				t.Errorf("%s: %d characters, the plan was made with a cut at 8", p, n)
			}
		}
		return nil
	})
	if mp3s != 4 {
		t.Errorf("%d tracks written", mp3s)
	}
}

func TestTheCheckOfAFormatNamesTheWrongSettingsAndStoresNothing(t *testing.T) {
	a, _ := seed(t)
	srv := start(t, a)
	form := playerForm("")
	form["component_max"] = 5

	var wrong struct{ Fields map[string]string }
	if code := sendInto(t, srv.URL, "POST", "/api/profiles/check", form, &wrong); code != 200 || wrong.Fields["name"] == "" || wrong.Fields["component_max"] == "" || len(wrong.Fields) != 2 {
		t.Errorf("%d %+v", code, wrong)
	}
	var right struct{ Fields map[string]string }
	if code := sendInto(t, srv.URL, "POST", "/api/profiles/check", playerForm("Car"), &right); code != 200 || len(right.Fields) != 0 {
		t.Errorf("%d %+v", code, right)
	}
	if got := profilesOf(t, srv.URL); len(got) != 1 {
		t.Errorf("the check stored something: %+v", got)
	}
}

func TestTheCheckOfAFormatLetsItKeepItsOwnName(t *testing.T) {
	a, _ := seed(t)
	srv := start(t, a)
	own := playerForm("Wolfgang WMP01")
	own["id"] = "wolfgang-wmp01"
	own["component_max"] = 100

	var edited, other struct{ Fields map[string]string }
	sendInto(t, srv.URL, "POST", "/api/profiles/check", own, &edited)
	own["id"] = ""
	sendInto(t, srv.URL, "POST", "/api/profiles/check", own, &other)

	if len(edited.Fields) != 0 {
		t.Errorf("a format is its own name: %+v", edited.Fields)
	}
	if other.Fields["name"] == "" {
		t.Errorf("a new format cannot take that name: %+v", other.Fields)
	}
}
