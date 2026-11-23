package ytdlp

import (
	"strconv"
	"testing"

	"music-manager/internal/app"
)

func TestParseInfoSaysWhatTheVideoTellsOfTheMusic(t *testing.T) {
	for _, c := range []struct {
		name, json string
		want       app.Info
	}{
		{"what YouTube Music says is taken as it is: the first of several artists",
			`{"id":"v1","title":"Video Title (Official)","track":"So What","artists":["Miles Davis","Bill Evans"],"artist":"Miles Davis, Bill Evans","album":"Kind of Blue","track_number":1.0,"release_year":1959.0,"genre":"Jazz","uploader":"Miles Davis - Topic"}`,
			app.Info{Title: "So What", Artist: "Miles Davis", Uploader: "Miles Davis - Topic", Album: "Kind of Blue", TrackNumber: 1, Released: "1959", Genre: "Jazz", Source: app.FromMetadata}},
		{"the artist field is kept whole when there is no list",
			`{"id":"v1","title":"T","track":"Song","artist":"Earth, Wind & Fire"}`,
			app.Info{Title: "Song", Artist: "Earth, Wind & Fire", Source: app.FromMetadata}},
		{"the first of several genres is the genre",
			`{"id":"v1","title":"T","track":"T","artist":"A","genres":["Folk","Rock"]}`,
			app.Info{Title: "T", Artist: "A", Genre: "Folk", Source: app.FromMetadata}},
		{"the track field is not tidied",
			`{"id":"v1","title":"Whatever","track":"Song (Official Remix)","artist":"A"}`,
			app.Info{Title: "Song (Official Remix)", Artist: "A", Source: app.FromMetadata}},
		{"a title with no separator stays whole and only the noise goes",
			`{"id":"v1","title":"Li Beirut (Official Video)","uploader":"Fairuz - Topic"}`,
			app.Info{Title: "Li Beirut", Uploader: "Fairuz - Topic"}},
		{"a full release date, not the upload date",
			`{"id":"v1","title":"T","release_date":"20150904","release_year":2015,"upload_date":"20230101"}`,
			app.Info{Title: "T", Released: "2015-09-04"}},
		{"only a release year",
			`{"id":"v1","title":"T","release_year":2012,"upload_date":"20230101"}`,
			app.Info{Title: "T", Released: "2012"}},
		{"only an upload date",
			`{"id":"v1","title":"T","upload_date":"20230101"}`,
			app.Info{Title: "T"}},
		{"null dates",
			`{"id":"v1","title":"T","release_date":null,"release_year":null}`,
			app.Info{Title: "T"}},
		{"a nonsense date",
			`{"id":"v1","title":"T","release_date":"soon"}`,
			app.Info{Title: "T"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			id, got, err := parseInfo([]byte(c.json + "\n"))
			if err != nil {
				t.Fatal(err)
			}
			if id != "v1" || got != c.want {
				t.Errorf("%q, %+v; want v1, %+v", id, got, c.want)
			}
		})
	}
}

func TestAnOrdinaryVideoIsSplitIntoArtistAndTitleAndTheNoiseDropped(t *testing.T) {
	for title, want := range map[string][2]string{
		"Fairuz - Li Beirut (Official Video)":         {"Fairuz", "Li Beirut"},
		"Fairuz – Li Beirut [Official Music Video]":   {"Fairuz", "Li Beirut"},
		"Fairuz — Li Beirut (Lyrics)":                 {"Fairuz", "Li Beirut"},
		"Miles Davis - So What (Official Audio) (HD)": {"Miles Davis", "So What"},
		"Max Richter - On the Nature of Daylight":     {"Max Richter", "On the Nature of Daylight"},
		"AC-DC - Back in Black":                       {"AC-DC", "Back in Black"},
		"Band - Song - Live at Home":                  {"Band", "Song - Live at Home"},
	} {
		_, got, err := parseInfo([]byte(`{"id":"v1","title":` + strconv.Quote(title) + `,"uploader":"Some Channel"}` + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		if got.Artist != want[0] || got.Title != want[1] || got.Source != app.FromTitle {
			t.Errorf("%q: %q by %q from %q, want %q by %q from the title", title, got.Title, got.Artist, got.Source, want[1], want[0])
		}
	}
}

func TestOnlyTheNoiseAtTheEndGoesAndAMeaningfulBracketStays(t *testing.T) {
	for title, want := range map[string]string{
		"Song (Live)":                      "Song (Live)",
		"Song (feat. Someone)":             "Song (feat. Someone)",
		"Song (official video)":            "Song",
		"Song [4K]":                        "Song",
		"Song (Official Video) (Remaster)": "Song (Official Video) (Remaster)",
		"(Official Video)":                 "(Official Video)",
	} {
		if got := tidyTitle(title); got != want {
			t.Errorf("%q -> %q, want %q", title, got, want)
		}
	}
}
