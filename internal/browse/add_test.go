package browse

import (
	"testing"

	"music-manager/internal/app"
)

func TestTheFormStartsWithWhatYouTubeSaid(t *testing.T) {
	got := addDefaults(app.Info{Title: "So What", Artist: "Miles Davis", Album: "Kind of Blue", TrackNumber: 1, TrackTotal: 5, Released: "1959-08-17", Genre: "Jazz"})

	want := AddForm{Title: "So What", Artist: "Miles Davis", Album: "Kind of Blue", Track: "1", Of: "5", Released: "1959-08-17", Genre: "Jazz"}
	if got != want {
		t.Errorf("%+v, want %+v", got, want)
	}
}

func TestAChannelNameStandsInForAnArtistTheSourceDidNotName(t *testing.T) {
	got := addDefaults(app.Info{Title: "Song", Uploader: "Some Band - Topic"})

	if got.Artist != "Some Band" {
		t.Errorf("artist %q", got.Artist)
	}
}

func TestTheNoteSaysWhereTheDetailsCameFrom(t *testing.T) {
	for name, c := range map[string]struct {
		info app.Info
		want string
	}{
		"metadata":       {app.Info{Title: "T", Artist: "A", Source: app.FromMetadata}, "From YouTube Music metadata"},
		"title":          {app.Info{Title: "T", Artist: "A", Source: app.FromTitle}, "Guessed from the video title"},
		"channel":        {app.Info{Title: "T", Uploader: "Band - Topic"}, "Guessed from the channel name"},
		"only a title":   {app.Info{Title: "T"}, "YouTube gave only a title"},
		"artist no flag": {app.Info{Title: "T", Artist: "A"}, "From YouTube Music metadata"},
	} {
		if got := addNote(c.info); len(got) < len(c.want) || got[:len(c.want)] != c.want {
			t.Errorf("%s: %q, want it to start with %q", name, got, c.want)
		}
	}
}
