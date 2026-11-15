package ytdlp

import (
	"testing"

	"music-manager/internal/app"
)

func TestWhatYouTubeMusicSaysIsTakenAsItIs(t *testing.T) {
	got, err := parseInfo([]byte(`{"id":"v1","title":"Video Title (Official)","track":"So What","artists":["Miles Davis","Bill Evans"],"artist":"Miles Davis, Bill Evans","album":"Kind of Blue","track_number":1.0,"release_year":1959.0,"genre":"Jazz","uploader":"Miles Davis - Topic"}` + "\n"))
	if err != nil {
		t.Fatal(err)
	}

	want := app.Info{Title: "So What", Artist: "Miles Davis", Uploader: "Miles Davis - Topic", Album: "Kind of Blue", TrackNumber: 1, Released: "1959", Genre: "Jazz", Source: app.FromMetadata}
	if got.id != "v1" || got.Info != want {
		t.Errorf("%+v, want %+v", got.Info, want)
	}
}

func TestTheFirstOfSeveralGenresIsTheGenre(t *testing.T) {
	got, _ := parseInfo([]byte(`{"id":"v1","title":"T","track":"T","artist":"A","genres":["Folk","Rock"]}` + "\n"))

	if got.Genre != "Folk" {
		t.Errorf("genre %q", got.Genre)
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
		got, err := parseInfo([]byte(`{"id":"v1","title":` + quote(title) + `,"uploader":"Some Channel"}` + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		if got.Artist != want[0] || got.Title != want[1] || got.Source != app.FromTitle {
			t.Errorf("%q: %q by %q from %q, want %q by %q from the title", title, got.Title, got.Artist, got.Source, want[1], want[0])
		}
	}
}

func TestATitleWithNoSeparatorStaysWholeAndOnlyTheNoiseGoes(t *testing.T) {
	got, _ := parseInfo([]byte(`{"id":"v1","title":"Li Beirut (Official Video)","uploader":"Fairuz - Topic"}` + "\n"))

	if got.Title != "Li Beirut" || got.Artist != "" || got.Source != "" || got.Uploader != "Fairuz - Topic" {
		t.Errorf("%+v", got.Info)
	}
}

func TestTheTrackFieldIsNotTidied(t *testing.T) {
	got, _ := parseInfo([]byte(`{"id":"v1","title":"Whatever","track":"Song (Official Remix)","artist":"A"}` + "\n"))

	if got.Title != "Song (Official Remix)" {
		t.Errorf("title %q", got.Title)
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

func quote(s string) string {
	out := []byte{'"'}
	for _, r := range s {
		if r == '"' || r == '\\' {
			out = append(out, '\\')
		}
		out = append(out, string(r)...)
	}
	return string(append(out, '"'))
}
