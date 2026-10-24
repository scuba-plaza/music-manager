package tui

import (
	"testing"

	"music-manager/internal/app"
)

// An artist, an album and a playlist have fields of their own in the right
// pane, as a track has: what is edited there is edited on all their tracks.
// → goes into the fields, enter opens and closes the tree.

func TestRightOnAnArtistGoesIntoItsFieldsAndEnterOpensIt(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "down", "down", "down") // Shawn James
	if got := selectedLabel(t, m); got != "Shawn James" {
		t.Fatalf("setup: %q", got)
	}
	m = keys(t, m, "right")
	if m.focus != paneInspector {
		t.Fatal("right did not go into the fields of the artist")
	}
	contains(t, m, "Name", "Latin name", "Genre", "Latin genre", "all 3 tracks")
	notInTree(t, m, "Shadows") // the tree did not open

	m = keys(t, m, "left")
	if m.focus != paneBrowser {
		t.Fatal("left did not come back to the tree")
	}
	if treeLine(m, "Shadows") >= 0 {
		t.Fatal("coming back opened the artist")
	}
	m = keys(t, m, "enter")
	if treeLine(m, "Shadows") < 0 {
		t.Error("enter did not open the artist")
	}
	m = keys(t, m, "enter")
	if treeLine(m, "Shadows") >= 0 {
		t.Error("enter did not close the artist")
	}
}

func TestRightOnAnAlbumGoesIntoItsFields(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "down", "down", "down", "enter", "down") // the album Shadows
	if got := selectedLabel(t, m); got != "Shadows" {
		t.Fatalf("setup: %q", got)
	}
	m = keys(t, m, "right")
	if m.focus != paneInspector {
		t.Fatal("right did not go into the fields of the album")
	}
	contains(t, m, "Name", "Latin name", "Released", "Genre", "Latin genre", "Tracks on album", "all 3 tracks")
	notInTree(t, m, "The Wanderer")
	m = keys(t, m, "esc")
	m = keys(t, m, "enter")
	if treeLine(m, "The Wanderer") < 0 {
		t.Error("enter did not open the album")
	}
}

func TestRightOnAPlaylistGoesIntoItsFields(t *testing.T) {
	lib, _ := playlistLib(t)
	m := newTestModel(t, lib, 120, 36)
	m = keys(t, m, "right", "down") // open the group of playlists, then Study
	if got := selectedLabel(t, m); got != "Study" {
		t.Fatalf("setup: %q", got)
	}
	m = keys(t, m, "right")
	if m.focus != paneInspector {
		t.Fatal("right did not go into the fields of the playlist")
	}
	contains(t, m, "Name", "Latin name")
	lacks(t, m, "Genre", "Released")
	notInTree(t, m, "Flow")
	m = keys(t, m, "left", "enter")
	if treeLine(m, "Flow") < 0 {
		t.Error("enter did not open the playlist")
	}
}

func TestRightStillOpensTheRowsThatHaveNoFields(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	before := len(m.rows)
	m = keys(t, m, "right") // Needs work
	if m.focus != paneBrowser || len(m.rows) <= before {
		t.Errorf("Needs work: focus %v, %d rows, was %d", m.focus, len(m.rows), before)
	}

	m, _ = browserModel(t, 120, 36)
	m = keys(t, m, "down", "enter", "down") // Fairuz opened, then its Singles
	if got := selectedLabel(t, m); got != "Singles" {
		t.Fatalf("setup: %q", got)
	}
	m = keys(t, m, "right")
	if m.focus != paneBrowser || treeLine(m, "Wtni") < 0 {
		t.Errorf("Singles: focus %v, tree line of its track %d", m.focus, treeLine(m, "Wtni"))
	}

	lib, _ := playlistLib(t)
	m = newTestModel(t, lib, 120, 36) // the group of playlists
	m = keys(t, m, "right")
	if m.focus != paneBrowser || treeLine(m, "Study") < 0 {
		t.Errorf("Playlists: focus %v, tree line of Study %d", m.focus, treeLine(m, "Study"))
	}
}

func TestUpAndDownChooseTheFieldOfAGroup(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "down", "down", "down", "right")
	if f := m.curField(); f != fArtist {
		t.Fatalf("starts on %v, want the name", f)
	}
	m = keys(t, m, "down", "down")
	if f := m.curField(); f != fGenre {
		t.Errorf("on %v, want the genre", f)
	}
	for range 10 {
		m = keys(t, m, "down")
	}
	if f := m.curField(); f != fLatinGenre {
		t.Errorf("down stops on %v, want the Latin genre", f)
	}
	m = keys(t, m, "k", "k", "k", "k", "k")
	if f := m.curField(); f != fArtist {
		t.Errorf("up stops on %v, want the name", f)
	}
	m = keys(t, m, "esc")
	if m.focus != paneBrowser || selectedLabel(t, m) != "Shawn James" {
		t.Errorf("esc: focus %v, on %q", m.focus, selectedLabel(t, m))
	}
}

func TestAGroupLeavesTheTabOfTheTracksAlone(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "right", "3") // the Latin tab of a track
	field := m.field
	m = keys(t, m, "esc", "left", "left", "left", "right") // back to the tree, its album, closed, its artist, into the fields of the artist
	if m.focus != paneInspector {
		t.Fatalf("setup: focus %v", m.focus)
	}
	m = keys(t, m, "down", "down", "esc")
	if m.field != field {
		t.Errorf("the track's field moved from %d to %d", field, m.field)
	}
	m = selectTrack(t, m, "Flow")
	m = keys(t, m, "right")
	contains(t, m, "Dim forms") // still on the Latin tab
}

func TestTheFooterOfAGroupSaysRightEditsAndEnterOpens(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "down", "down", "down")
	contains(t, m, "→ edit", "enter open")
	lacks(t, m, "→ open")
}

// agreeing is a group of tracks by one artist for the unit tests of what the
// fields show.
func agreeing(genres ...string) group {
	g := group{kind: kindArtist, name: "Band"}
	for i, genre := range genres {
		tr := trk(string(rune('a'+i)), "Song", "Band", "Hits", i+1)
		tr.Genre = genre
		g.tracks = append(g.tracks, tr)
	}
	return g
}

func TestAFieldTheTracksAgreeOnShowsItAndOneThatDiffersIsMixed(t *testing.T) {
	text, mixed, _ := groupValue(agreeing("Folk", "Folk"), fGenre)
	if text != "Folk" || mixed {
		t.Errorf("tracks that agree: %q, mixed %v", text, mixed)
	}
	if _, mixed, _ = groupValue(agreeing("Folk", "Rock"), fGenre); !mixed {
		t.Error("tracks that differ should be mixed")
	}
	if text, mixed, _ = groupValue(agreeing("", ""), fGenre); text != "" || mixed {
		t.Errorf("tracks with none: %q, mixed %v", text, mixed)
	}
	if _, mixed, _ = groupValue(agreeing("Folk", ""), fGenre); !mixed {
		t.Error("a genre on some tracks only should be mixed")
	}
}

func TestALatinFormIsOneOfTheGroupWhenItIsTheSameOnAllTracks(t *testing.T) {
	g := agreeing("", "")
	for i := range g.tracks {
		g.tracks[i].Artist = "فيروز"
	}
	if text, mixed, byHand := groupValue(g, fLatinArtist); text != "Firuz" || mixed || byHand {
		t.Errorf("worked out: %q, mixed %v, by hand %v", text, mixed, byHand)
	}
	g.tracks[0].ArtistLatin = "Fairouz"
	if _, mixed, _ := groupValue(g, fLatinArtist); !mixed {
		t.Error("one track with a form of its own and one without should be mixed")
	}
	g.tracks[1].ArtistLatin = "Fairouz"
	if text, mixed, byHand := groupValue(g, fLatinArtist); text != "Fairouz" || mixed || !byHand {
		t.Errorf("set by hand on all: %q, mixed %v, by hand %v", text, mixed, byHand)
	}
}

func TestTheFieldsOfAnArtistShowWhatItsTracksAgreeOnAndMixedForTheRest(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	a := appOf(t, hook.Library)
	for _, c := range []struct{ title, genre string }{{"Flow", "Folk"}, {"The Wanderer", "Folk"}, {"Along Our Way", "Rock"}} {
		if _, err := a.Edit(trackNamed(t, hook.Library, c.title).ID, app.Edit{Genre: ptr(c.genre)}); err != nil {
			t.Fatal(err)
		}
	}
	m = send(t, m, m.Init())
	m = keys(t, selectTrack(t, m, "Flow"), "left", "left", "left", "right") // the artist's fields
	contains(t, m, "Shawn James", "(mixed)")
}
