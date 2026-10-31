package device_test

import (
	"reflect"
	"strings"
	"testing"

	"music-manager/internal/device"
	"music-manager/internal/layout"
	"music-manager/internal/manifest"
)

func track(id, artist, album, title string) manifest.Track {
	return manifest.Track{ID: id, Status: manifest.Identified, Artist: artist, Album: album, Title: title}
}

// warned is the messages of the warnings for one track.
func warned(ws []device.Warning, id string) []string {
	var out []string
	for _, w := range ws {
		if w.TrackID == id {
			out = append(out, w.Message)
		}
	}
	return out
}

func hasMessage(msgs []string, part string) bool {
	for _, m := range msgs {
		if strings.Contains(m, part) {
			return true
		}
	}
	return false
}

func TestTheWolfgangWarnsAboutAGenreItCannotShow(t *testing.T) {
	long, short := track("long", "A", "", "S"), track("short", "A", "", "T")
	long.Genre, short.Genre = "Alternative Rock", "Rock"
	ws := device.Wolfgang.Diagnose([]manifest.Track{long, short})
	if got := warned(ws, "long"); !hasMessage(got, "genre") || !hasMessage(got, "12") {
		t.Errorf("long genre: %q", got)
	}
	if got := warned(ws, "short"); len(got) != 0 {
		t.Errorf("a genre that fits drew %q", got)
	}
}

func TestTheWolfgangWarnsAboutANameWithNoLatinForm(t *testing.T) {
	emoji := track("emoji", "🎵🎶", "", "Song")
	plain := track("plain", "Max Richter", "", "Dream")
	ws := device.Wolfgang.Diagnose([]manifest.Track{emoji, plain})
	if got := warned(ws, "emoji"); !hasMessage(got, "artist") || !hasMessage(got, "Unknown") {
		t.Errorf("artist with no Latin form: %q", got)
	}
	if got := warned(ws, "plain"); len(got) != 0 {
		t.Errorf("a plain name drew %q", got)
	}
}

func TestTheWolfgangWarnsWhenCuttingMakesTwoFileNamesAlike(t *testing.T) {
	a := track("aaaaaa", "Artist", "", "A Very Long Song Title Indeed Yes")
	b := track("bbbbbb", "Artist", "", "A Very Long Song Title Indeed No")
	other := track("cccccc", "Artist", "", "A Different Song")
	ws := device.Wolfgang.Diagnose([]manifest.Track{a, b, other})
	for _, id := range []string{"aaaaaa", "bbbbbb"} {
		if got := warned(ws, id); !hasMessage(got, "cut") {
			t.Errorf("%s: %q", id, got)
		}
	}
	if got := warned(ws, "cccccc"); len(got) != 0 {
		t.Errorf("a name that is not cut like another drew %q", got)
	}
}

func TestTheWolfgangDoesNotBlameTheCutForTwoTracksThatAreTheSame(t *testing.T) {
	a, b := track("aaaaaa", "Artist", "", "Same"), track("bbbbbb", "Artist", "", "Same")
	if ws := device.Wolfgang.Diagnose([]manifest.Track{a, b}); len(ws) != 0 {
		t.Errorf("warnings = %+v", ws)
	}
}

func TestTheWolfgangWarnsAboutAnArtistThatLooksLikeSeveral(t *testing.T) {
	duo := track("duo", "Fairuz & Ziad Rahbani", "", "Song")
	ws := device.Wolfgang.Diagnose([]manifest.Track{duo})
	if got := warned(ws, "duo"); !hasMessage(got, "several") {
		t.Errorf("%q", got)
	}
}

func TestAGenericTargetHasNothingToWarnAbout(t *testing.T) {
	long := track("long", "🎵 & Fairuz", "", "A Very Long Song Title Indeed Yes")
	long.Genre = "Alternative Rock"
	if ws := device.Generic.Diagnose([]manifest.Track{long}); len(ws) != 0 {
		t.Errorf("warnings = %+v", ws)
	}
}

// The Latin forms set by hand are what the Wolfgang is written from.

func handSet() manifest.Track {
	return manifest.Track{
		ID: "k7f2qd", Status: manifest.Identified,
		Title: "بحلم بيك", Artist: "عبد الحليم حافظ", Album: "روائع", Genre: "طرب",
		TitleLatin: "Bahlam Bek", ArtistLatin: "Abdel Halim Hafez", AlbumLatin: "Rawaa'e", GenreLatin: "Tarab",
		TrackNumber: 1, TrackTotal: 6,
	}
}

func TestTheWolfgangWritesTheTagsFromTheLatinFormsSetByHand(t *testing.T) {
	tags := device.Wolfgang.Tags(handSet())
	if tags.Artist != "A. H. Hafez" || tags.Album != "Rawaa'e" {
		t.Errorf("artist %q, album %q", tags.Artist, tags.Album)
	}
	if tags.Title != "بحلم بيك" {
		t.Errorf("the title is %q; the player shows file names, so it stays the true one", tags.Title)
	}
	if tags.Genre != "Tarab" {
		t.Errorf("genre %q", tags.Genre)
	}
}

func TestTheWolfgangsSingleCarriesItsLatinTitleAsTheAlbum(t *testing.T) {
	tr := handSet()
	tr.Album, tr.AlbumLatin = "", ""
	if got := device.Wolfgang.Tags(tr).Album; got != "Bahlam Bek" {
		t.Errorf("album of a single = %q", got)
	}
}

func TestTheGenericTargetKeepsTheTrueNamesWhateverIsSetByHand(t *testing.T) {
	tags := device.Generic.Tags(handSet())
	if tags.Artist != "عبد الحليم حافظ" || tags.Album != "روائع" || tags.Genre != "طرب" {
		t.Errorf("%+v", tags)
	}
}

func TestTheWolfgangNamesTheFilesAfterTheLatinFormsSetByHand(t *testing.T) {
	got := layout.PathFor(handSet(), device.Wolfgang.Layout())
	if want := "Abdel Halim Hafez/Rawaa'e/01 - Bahlam Bek.mp3"; got != want {
		t.Errorf("PathFor = %q, want %q", got, want)
	}
	if got := layout.PathFor(handSet(), device.Generic.Layout()); !strings.Contains(got, "عبد الحليم حافظ") {
		t.Errorf("the generic path lost the true name: %q", got)
	}
}

func TestANameWithALatinFormSetByHandIsNotUnknownOnTheWolfgang(t *testing.T) {
	tr := track("emoji", "🎵🎶", "", "Song")
	if got := warned(device.Wolfgang.Diagnose([]manifest.Track{tr}), "emoji"); !hasMessage(got, "Unknown") {
		t.Fatalf("no warning to start with: %q", got)
	}
	tr.ArtistLatin = "Music"
	if got := warned(device.Wolfgang.Diagnose([]manifest.Track{tr}), "emoji"); len(got) != 0 {
		t.Errorf("a Latin form was set by hand, yet %q", got)
	}
}

func TestTheGenreWarningIsAboutWhatIsWritten(t *testing.T) {
	tr := track("g", "A", "", "S")
	tr.Genre = "Alternative Rock"
	tr.GenreLatin = "Alt Rock"
	if got := warned(device.Wolfgang.Diagnose([]manifest.Track{tr}), "g"); len(got) != 0 {
		t.Errorf("the Latin genre fits, yet %q", got)
	}
}

func TestTheWarningsComeInTheSameOrderEveryTime(t *testing.T) {
	var tracks []manifest.Track
	for _, artist := range []string{"Zed", "Yan", "Xia", "Wen", "Vic", "Uma"} {
		tracks = append(tracks,
			track("b-"+artist, artist, "", "A Very Long Song Title Indeed Yes"),
			track("a-"+artist, artist, "", "A Very Long Song Title Indeed No"))
	}
	first := device.Wolfgang.Diagnose(tracks)
	if len(first) != len(tracks) {
		t.Fatalf("%d warnings, want one for each of the %d tracks", len(first), len(tracks))
	}
	for i := 1; i < 20; i++ { // a map would give another order now and then
		if ws := device.Wolfgang.Diagnose(tracks); !reflect.DeepEqual(ws, first) {
			t.Fatalf("run %d came in another order:\n%+v\nafter\n%+v", i, ws, first)
		}
	}
}

func warningKinds(ws []device.Warning) []string {
	kinds := map[string]string{"genre": "the genre", "no-latin": "no Latin letters", "several": "looks like several", "cut": "is cut at"}
	var out []string
	for _, kind := range []string{"genre", "no-latin", "several", "cut"} {
		for _, w := range ws {
			if strings.Contains(w.Message, kinds[kind]) {
				out = append(out, kind)
				break
			}
		}
	}
	return out
}

func TestEachDiagnosticFollowsItsProfileField(t *testing.T) {
	genre := track("genre1", "Rock Band", "", "Loud")
	genre.Genre = "Alternative Rock Pop"
	tracks := []manifest.Track{
		genre,
		track("emoji1", "🎵🎶", "", "Song"),
		track("duo001", "Fairuz & Ziad Rahbani", "", "Song"),
		track("cuta01", "Artist", "", "A Very Long Song Title Indeed Yes"),
		track("cutb01", "Artist", "", "A Very Long Song Title Indeed No"),
	}
	for _, c := range []struct {
		name   string
		fields string
		want   []string
	}{
		{"nothing set", ``, nil},
		{"latin_names only", `latin_names = true`, []string{"no-latin"}},
		{"stem_max only", `stem_max = 20`, []string{"cut"}},
		{"latin_names and tag_width without latin_genre", "latin_names = true\ntag_width = 12", []string{"no-latin", "several"}},
		{"latin_genre without tag_width", `latin_genre = true`, nil},
		{"every check on", "latin_names = true\ntag_width = 12\nlatin_genre = true\nstem_max = 20", []string{"genre", "no-latin", "several", "cut"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			profiles, err := device.ParseProfiles([]byte("[[profile]]\nname = \"my-player\"\n" + c.fields + "\n"))
			if err != nil {
				t.Fatal(err)
			}
			ws := profiles[0].Target().Diagnose(tracks)
			if got := warningKinds(ws); !reflect.DeepEqual(got, c.want) {
				t.Errorf("warnings %v, want %v: %+v", got, c.want, ws)
			}
		})
	}
}
