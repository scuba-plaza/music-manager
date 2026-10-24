package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/testmp3"
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

// toArtist is the model with the cursor on the artist Shawn James, its album
// still open, behind the hook that counts the calls to the library.
func toArtist(t *testing.T) (Model, *editHook) {
	t.Helper()
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "up", "up", "up") // The Wanderer, the album Shadows, the artist
	if got := selectedLabel(t, m); got != "Shawn James" {
		t.Fatalf("setup: %q", got)
	}
	return m, hook
}

func artistOf(t *testing.T, lib Library, title string) string {
	t.Helper()
	return trackNamed(t, lib, title).Artist
}

func TestRenamingAnArtistInThePaneRenamesAllItsTracks(t *testing.T) {
	m, hook := toArtist(t)
	m = keys(t, m, "right", "enter")
	if got := m.in[0].Value(); got != "Shawn James" {
		t.Fatalf("the editor starts from %q", got)
	}
	contains(t, m, "enter save", "esc cancel")
	m = clearField(t, m)
	m = typed(t, m, "Shawn J.")
	m = keys(t, m, "enter")

	for _, title := range []string{"The Wanderer", "Flow", "Along Our Way"} {
		if got := artistOf(t, hook.Library, title); got != "Shawn J." {
			t.Errorf("%s is by %q", title, got)
		}
	}
	if got := artistOf(t, hook.Library, "Current"); got != "Phoria" {
		t.Errorf("another artist's track was changed: %q", got)
	}
	contains(t, m, "Changed 3 tracks", "Shawn J.")
	if hook.manyCalls != 1 {
		t.Errorf("%d saves, want the three tracks saved together", hook.manyCalls)
	}
}

func TestAFilterDoesNotNarrowWhatARenameOfAnArtistReaches(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "esc", "/")
	m = typed(t, m, "flow")
	m = keys(t, m, "enter")
	if got := selectedLabel(t, m); got != "Shawn James" {
		t.Fatalf("setup: the filter leaves the cursor on %q", got)
	}
	m = keys(t, m, "right", "enter")
	contains(t, m, "all 3 tracks") // not only the one the filter shows
	m = typed(t, clearField(t, m), "Shawn J.")
	m = keys(t, m, "enter")
	for _, title := range []string{"The Wanderer", "Flow", "Along Our Way"} {
		if got := artistOf(t, hook.Library, title); got != "Shawn J." {
			t.Errorf("%s is by %q: the filter narrowed the rename", title, got)
		}
	}
}

func TestTheCursorAndTheOpenAlbumsFollowTheNewNameOfAnArtist(t *testing.T) {
	m, _ := toArtist(t)
	if treeLine(m, "Flow") < 0 {
		t.Fatal("setup: the album is not open")
	}
	m = keys(t, m, "right", "enter")
	m = typed(t, clearField(t, m), "Shawn J.")
	m = keys(t, m, "enter")
	if got := selectedLabel(t, m); got != "Shawn J." {
		t.Errorf("the cursor is on %q, want the renamed artist", got)
	}
	if m.focus != paneInspector {
		t.Error("the fields lost the keyboard after saving")
	}
	if treeLine(m, "Shadows") < 0 || treeLine(m, "Flow") < 0 {
		t.Errorf("the artist or its album closed with the rename:\n%s", screen(m))
	}
}

func TestMovingTheKeysOfAnArtistMovesThoseOfItsAlbumsToo(t *testing.T) {
	open := map[string]bool{
		artistKey("A"): true, albumRowKey("A", "X"): true, singlesKey("A"): true,
		albumRowKey("B", "X"): true, // another artist's album of the same name
	}
	moveKeys(open, artistMoves("A", "C", []string{"X", "Y"}))
	want := map[string]bool{
		artistKey("C"): true, albumRowKey("C", "X"): true, singlesKey("C"): true,
		albumRowKey("B", "X"): true,
	}
	if !reflect.DeepEqual(open, want) {
		t.Errorf("keys = %v, want %v", open, want)
	}
}

func TestRenamingAnArtistToAnotherOneJoinsThemAndSaysSoWhileTyping(t *testing.T) {
	m, hook := editingModel(t, "Current")
	m = keys(t, m, "up", "up") // the album, then the artist Phoria
	if got := selectedLabel(t, m); got != "Phoria" {
		t.Fatalf("setup: %q", got)
	}
	m = keys(t, m, "right", "enter")
	lacks(t, m, "joins")
	m = typed(t, clearField(t, m), "Shawn James")
	contains(t, m, "joins the artist Shawn James (3 tracks)")
	m = keys(t, m, "enter")
	if got := artistOf(t, hook.Library, "Current"); got != "Shawn James" {
		t.Errorf("artist = %q", got)
	}
	n := 0
	for _, r := range m.rows {
		if r.Kind == kindArtist && r.Label == "Shawn James" {
			n++
			if r.Count != 4 {
				t.Errorf("the joined artist has %d tracks, want 4", r.Count)
			}
		}
	}
	if n != 1 || selectedLabel(t, m) != "Shawn James" {
		t.Errorf("%d rows for the artist, cursor on %q", n, selectedLabel(t, m))
	}
}

func TestTheNameOfAnArtistCannotBeEmpty(t *testing.T) {
	m, hook := toArtist(t)
	m = keys(t, m, "right", "enter")
	m = clearField(t, m)
	m = keys(t, m, "enter")
	contains(t, m, "cannot be empty")
	if !m.editing || hook.manyCalls != 0 {
		t.Errorf("editing %v, saves %d", m.editing, hook.manyCalls)
	}
}

func TestEscCancelsAndTheSameNameSavesNothing(t *testing.T) {
	m, hook := toArtist(t)
	m = typed(t, keys(t, m, "right", "enter"), "X")
	m = keys(t, m, "esc")
	if m.editing || m.focus != paneInspector || hook.manyCalls != 0 {
		t.Errorf("editing %v, focus %v, saves %d", m.editing, m.focus, hook.manyCalls)
	}
	m = keys(t, m, "enter", "enter") // open it, change nothing, save
	if m.editing || hook.manyCalls != 0 {
		t.Errorf("editing %v, saves %d", m.editing, hook.manyCalls)
	}
}

func TestOneUndoPutsTheOldNameOfAnArtistBack(t *testing.T) {
	m, hook := toArtist(t)
	m = keys(t, m, "right", "enter")
	m = typed(t, clearField(t, m), "Shawn J.")
	m = keys(t, m, "enter", "u")
	contains(t, m, "Undid")
	for _, title := range []string{"The Wanderer", "Flow", "Along Our Way"} {
		if got := artistOf(t, hook.Library, title); got != "Shawn James" {
			t.Errorf("after undo %s is by %q", title, got)
		}
	}
	m = keys(t, m, "ctrl+r")
	if got := artistOf(t, hook.Library, "Flow"); got != "Shawn J." {
		t.Errorf("after redo Flow is by %q", got)
	}
}

// withEdits changes tracks behind the model's back, as another run of mm would,
// and loads them again.
func withEdits(t *testing.T, m Model, hook *editHook, edits map[string]app.Edit) Model {
	t.Helper()
	a := appOf(t, hook.Library)
	for title, e := range edits {
		if _, err := a.Edit(trackNamed(t, hook.Library, title).ID, e); err != nil {
			t.Fatal(err)
		}
	}
	return send(t, m, m.Init())
}

// toArtistFields is the model on the fields of Shawn James, the cursor on the
// field f.
func toArtistFields(t *testing.T, m Model, f fieldKind) Model {
	t.Helper()
	if m.focus == paneInspector {
		m = keys(t, m, "esc")
	}
	m = selectTrack(t, m, "Flow")
	m = keys(t, m, "up", "up", "up", "right")
	for i := 0; i < 10 && m.curField() != f; i++ {
		m = keys(t, m, "down")
	}
	if m.curField() != f {
		t.Fatalf("no field %v", f)
	}
	return m
}

func genreOf(t *testing.T, lib Library, title string) string {
	t.Helper()
	return trackNamed(t, lib, title).Genre
}

func TestAMixedFieldOpensEmptyAndEmptyKeepsAndDashClears(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = withEdits(t, m, hook, map[string]app.Edit{"Flow": {Genre: ptr("Folk")}, "The Wanderer": {Genre: ptr("Folk")}, "Along Our Way": {Genre: ptr("Rock")}})
	m = toArtistFields(t, m, fGenre)
	contains(t, m, "(mixed)")

	m = keys(t, m, "enter")
	if got := m.in[0].Value(); got != "" {
		t.Fatalf("a mixed field opens with %q, want it empty", got)
	}
	contains(t, m, "the tracks differ", "empty keeps them")
	m = keys(t, m, "enter") // empty keeps
	if m.editing || hook.manyCalls != 0 {
		t.Errorf("editing %v, saves %d: empty should keep the tracks as they are", m.editing, hook.manyCalls)
	}

	m = typed(t, keys(t, m, "enter"), "-")
	m = keys(t, m, "enter")
	for _, title := range []string{"Flow", "The Wanderer", "Along Our Way"} {
		if got := genreOf(t, hook.Library, title); got != "" {
			t.Errorf("after -, %s has the genre %q", title, got)
		}
	}
}

func TestASharedFieldOpensWithItsValueAndEmptyClearsIt(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = withEdits(t, m, hook, map[string]app.Edit{"Flow": {Genre: ptr("Folk")}, "The Wanderer": {Genre: ptr("Folk")}, "Along Our Way": {Genre: ptr("Folk")}})
	m = toArtistFields(t, m, fGenre)
	contains(t, m, "Folk")
	lacks(t, m, "(mixed)")
	m = keys(t, m, "enter")
	if got := m.in[0].Value(); got != "Folk" {
		t.Fatalf("opens with %q", got)
	}
	m = keys(t, clearField(t, m), "enter")
	for _, title := range []string{"Flow", "The Wanderer", "Along Our Way"} {
		if got := genreOf(t, hook.Library, title); got != "" {
			t.Errorf("%s still has the genre %q", title, got)
		}
	}
}

func TestTheGenreOfAnArtistIsSetOnAllItsTracks(t *testing.T) {
	m, hook := toArtist(t)
	m = keys(t, m, "right", "down", "down", "enter")
	m = typed(t, m, " Folk ")
	m = keys(t, m, "enter")
	for _, title := range []string{"Flow", "The Wanderer", "Along Our Way"} {
		if got := genreOf(t, hook.Library, title); got != "Folk" {
			t.Errorf("%s has the genre %q", title, got)
		}
	}
	if got := genreOf(t, hook.Library, "Current"); got != "" {
		t.Errorf("another artist's track has the genre %q", got)
	}
	contains(t, m, "Changed 3 tracks")
}

func TestTheLatinNameOfAnArtistIsSetOnAllItsTracksAndTheNameIsNotTouched(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	arabic := app.Edit{Artist: ptr("فيروز")}
	m = withEdits(t, m, hook, map[string]app.Edit{"Flow": arabic, "The Wanderer": arabic, "Along Our Way": arabic})
	m = toArtistFields(t, m, fLatinArtist)
	contains(t, m, "فيروز", "Firuz") // the name, and the form worked out from it
	m = keys(t, m, "enter")
	if got := m.in[0].Value(); got != "Firuz" {
		t.Fatalf("opens with %q, want the form worked out", got)
	}
	contains(t, m, "for فيروز")
	m = keys(t, m, "enter") // saving what was there changes nothing
	if hook.manyCalls != 0 {
		t.Fatal("the worked-out form was saved as if it were set by hand")
	}

	m = keys(t, m, "enter")
	m = typed(t, clearField(t, m), "Fairouz")
	m = keys(t, m, "enter")
	for _, title := range []string{"Flow", "The Wanderer", "Along Our Way"} {
		if got := trackNamed(t, hook.Library, title); got.ArtistLatin != "Fairouz" || got.Artist != "فيروز" {
			t.Errorf("%s: artist %q, form %q", title, got.Artist, got.ArtistLatin)
		}
	}
	contains(t, m, "Fairouz", "Changed 3 tracks")
}

func TestTheLatinGenreOnlyReachesTheTracksThatHaveAGenre(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = toArtistFields(t, m, fLatinGenre)
	m = keys(t, m, "enter")
	if m.editing || hook.manyCalls != 0 {
		t.Errorf("editing %v: no track has a genre to write in Latin letters", m.editing)
	}
	contains(t, m, "no genre")

	m = withEdits(t, m, hook, map[string]app.Edit{"Flow": {Genre: ptr("طرب")}})
	m = toArtistFields(t, m, fLatinGenre)
	m = keys(t, m, "enter")
	m = typed(t, clearField(t, m), "Tarab")
	m = keys(t, m, "enter")
	if got := trackNamed(t, hook.Library, "Flow").GenreLatin; got != "Tarab" {
		t.Errorf("Flow has the form %q", got)
	}
	contains(t, m, "Changed 1 track")
}

// toAlbum is the model with the cursor on the album Shadows of Shawn James.
func toAlbum(t *testing.T) (Model, *editHook) {
	t.Helper()
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "up", "up") // The Wanderer, then the album
	if got := selectedLabel(t, m); got != "Shadows" {
		t.Fatalf("setup: %q", got)
	}
	return m, hook
}

func albumOf(t *testing.T, lib Library, title string) string {
	t.Helper()
	return trackNamed(t, lib, title).Album
}

func TestAnAlbumEditOnlyTouchesTheTracksOfThatArtist(t *testing.T) {
	m, hook := toAlbum(t)
	// Another artist has an album of the same name.
	m = withEdits(t, m, hook, map[string]app.Edit{"Current": {Album: ptr("Shadows")}})
	m = selectTrack(t, m, "Flow")
	m = keys(t, m, "up", "up", "right", "enter")
	if got := m.in[0].Value(); got != "Shadows" {
		t.Fatalf("the editor starts from %q", got)
	}
	contains(t, m, "all 3 tracks")
	m = typed(t, clearField(t, m), "Shadows (Deluxe)")
	m = keys(t, m, "enter")
	for _, title := range []string{"The Wanderer", "Flow", "Along Our Way"} {
		if got := albumOf(t, hook.Library, title); got != "Shadows (Deluxe)" {
			t.Errorf("%s is on %q", title, got)
		}
	}
	if got := albumOf(t, hook.Library, "Current"); got != "Shadows" {
		t.Errorf("the album of another artist was renamed: %q", got)
	}
	contains(t, m, "Changed 3 tracks")
}

func TestTheCursorAndTheOpenStateFollowTheNewNameOfAnAlbum(t *testing.T) {
	m, _ := toAlbum(t)
	if treeLine(m, "Flow") < 0 {
		t.Fatal("setup: the album is not open")
	}
	m = keys(t, m, "right", "enter")
	m = typed(t, clearField(t, m), "Shadows II")
	m = keys(t, m, "enter")
	if got := selectedLabel(t, m); got != "Shadows II" {
		t.Errorf("the cursor is on %q", got)
	}
	if m.focus != paneInspector || treeLine(m, "Flow") < 0 {
		t.Errorf("focus %v, album open: %v", m.focus, treeLine(m, "Flow") >= 0)
	}
}

func TestTheNameOfAnAlbumCannotBeEmptyAndJoiningAnotherIsSaid(t *testing.T) {
	m, hook := toAlbum(t)
	m = withEdits(t, m, hook, map[string]app.Edit{"Along Our Way": {Album: ptr("Other")}})
	m = selectTrack(t, m, "Flow")
	m = keys(t, m, "up", "up", "right", "enter") // Shadows now holds two tracks
	m = typed(t, clearField(t, m), "Other")
	contains(t, m, "joins the album Other of Shawn James (1 track)")
	m = keys(t, m, "enter")
	for _, title := range []string{"The Wanderer", "Flow", "Along Our Way"} {
		if got := albumOf(t, hook.Library, title); got != "Other" {
			t.Errorf("%s is on %q", title, got)
		}
	}

	m = keys(t, m, "enter")
	m = keys(t, clearField(t, m), "enter")
	contains(t, m, "cannot be empty")
	if !m.editing {
		t.Error("the editor closed on an empty name")
	}
}

func TestTheOtherFieldsOfAnAlbumAreSetOnAllItsTracks(t *testing.T) {
	m, hook := toAlbum(t)
	set := func(f fieldKind, text string) {
		t.Helper()
		m = keys(t, m, "esc")
		m = selectTrack(t, m, "Flow")
		m = keys(t, m, "up", "up", "right")
		for i := 0; i < 10 && m.curField() != f; i++ {
			m = keys(t, m, "down")
		}
		if m.curField() != f {
			t.Fatalf("no field %v", f)
		}
		m = keys(t, m, "enter")
		m = typed(t, clearField(t, m), text)
		m = keys(t, m, "enter")
		if m.editing {
			t.Fatalf("%v: %s", f, screen(m))
		}
	}
	set(fReleased, "2012-03")
	set(fGenre, "طرب")
	set(fLatinGenre, "Tarab")
	set(fLatinAlbum, "Shadows (Latin)")
	set(fAlbumOf, "10")
	for _, title := range []string{"The Wanderer", "Flow", "Along Our Way"} {
		got := trackNamed(t, hook.Library, title)
		if got.Released != "2012-03" || got.Genre != "طرب" || got.GenreLatin != "Tarab" || got.AlbumLatin != "Shadows (Latin)" || got.TrackTotal != 10 {
			t.Errorf("%s = released %q, genre %q / %q, album form %q, of %d", title, got.Released, got.Genre, got.GenreLatin, got.AlbumLatin, got.TrackTotal)
		}
	}
	if got := trackNamed(t, hook.Library, "Current"); got.Released != "" || got.Genre != "" {
		t.Errorf("a track of another album was changed: %+v", got)
	}
	// Each of them was one step to undo: the last was the count.
	m = keys(t, m, "esc", "u")
	if got := trackNamed(t, hook.Library, "Flow"); got.TrackTotal != 12 || got.AlbumLatin != "Shadows (Latin)" {
		t.Errorf("after one undo: of %d, album form %q", got.TrackTotal, got.AlbumLatin)
	}
}

func TestABadDateOrCountForAnAlbumIsExplainedAndNothingIsSaved(t *testing.T) {
	m, hook := toAlbum(t)
	m = keys(t, m, "right", "down", "down", "enter") // Released
	m = typed(t, m, "next spring")
	m = keys(t, m, "enter")
	contains(t, m, "is not a date")
	if hook.manyCalls != 0 || !m.editing {
		t.Errorf("saves %d, editing %v", hook.manyCalls, m.editing)
	}
	m = keys(t, m, "esc", "down", "down", "down", "enter") // Tracks on album
	m = typed(t, m, "many")
	m = keys(t, m, "enter")
	contains(t, m, "whole number")
	if hook.manyCalls != 0 {
		t.Errorf("saves %d", hook.manyCalls)
	}
}

// toCover is the model on the Cover field of the album Shadows.
func toCover(t *testing.T, m Model) Model {
	t.Helper()
	if m.focus == paneInspector {
		m = keys(t, m, "esc")
	}
	m = selectTrack(t, m, "Flow")
	m = keys(t, m, "up", "up", "right")
	for i := 0; i < 10 && m.curField() != fCover; i++ {
		m = keys(t, m, "down")
	}
	if m.curField() != fCover {
		t.Fatal("no Cover field")
	}
	return m
}

// imageFile writes a small JPEG and returns its path.
func imageFile(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, testmp3.JPEG, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func coverOf(t *testing.T, lib Library, title string) string {
	t.Helper()
	return trackNamed(t, lib, title).CoverSHA256
}

func TestTheCoverOfAnAlbumSaysWhetherItsTracksShareOne(t *testing.T) {
	m, hook := toAlbum(t)
	m = toCover(t, m)
	contains(t, m, "Cover", "none")

	a := appOf(t, hook.Library)
	ids := []string{trackNamed(t, hook.Library, "Flow").ID, trackNamed(t, hook.Library, "The Wanderer").ID}
	if _, err := a.SetCoverMany(ids, testmp3.JPEG); err != nil {
		t.Fatal(err)
	}
	m = toCover(t, send(t, m, m.Init()))
	contains(t, m, "1 cover, 1 track without one")

	if _, err := a.SetCoverMany([]string{trackNamed(t, hook.Library, "Along Our Way").ID}, testmp3.JPEG); err != nil {
		t.Fatal(err)
	}
	m = toCover(t, send(t, m, m.Init()))
	contains(t, m, "the same on all 3 tracks")
}

func TestAnImageFileBecomesTheCoverOfEveryTrackOfTheAlbumInOneStep(t *testing.T) {
	m, hook := toAlbum(t)
	m = toCover(t, m)
	m = keys(t, m, "enter")
	contains(t, m, "a JPEG or PNG file", "empty removes")
	m = typed(t, m, imageFile(t, "front.jpg"))
	m = keys(t, m, "enter")
	sha := coverOf(t, hook.Library, "Flow")
	if sha == "" {
		t.Fatalf("no cover was set:\n%s", screen(m))
	}
	for _, title := range []string{"The Wanderer", "Along Our Way"} {
		if got := coverOf(t, hook.Library, title); got != sha {
			t.Errorf("%s has the cover %q, want %q", title, got, sha)
		}
	}
	if got := coverOf(t, hook.Library, "Current"); got != "" {
		t.Error("a track of another album got the cover")
	}
	contains(t, m, "Changed 3 tracks", "the same on all 3 tracks")

	m = keys(t, m, "esc", "u")
	for _, title := range []string{"The Wanderer", "Flow", "Along Our Way"} {
		if got := coverOf(t, hook.Library, title); got != "" {
			t.Errorf("after undo %s has the cover %q", title, got)
		}
	}
}

func TestTheCoverOfAnAlbumIsTakenAwayWithXOrWithAnEmptyPath(t *testing.T) {
	m, hook := toAlbum(t)
	a := appOf(t, hook.Library)
	var ids []string
	for _, title := range []string{"The Wanderer", "Flow", "Along Our Way"} {
		ids = append(ids, trackNamed(t, hook.Library, title).ID)
	}
	if _, err := a.SetCoverMany(ids, testmp3.JPEG); err != nil {
		t.Fatal(err)
	}
	m = toCover(t, send(t, m, m.Init()))
	m = keys(t, m, "x")
	for _, title := range []string{"The Wanderer", "Flow", "Along Our Way"} {
		if got := coverOf(t, hook.Library, title); got != "" {
			t.Errorf("x left %s the cover %q", title, got)
		}
	}
	contains(t, m, "none")

	if _, err := a.SetCoverMany(ids, testmp3.JPEG); err != nil {
		t.Fatal(err)
	}
	m = toCover(t, send(t, m, m.Init()))
	m = keys(t, m, "enter", "enter") // an empty path removes the cover
	if got := coverOf(t, hook.Library, "Flow"); got != "" {
		t.Errorf("an empty path left the cover %q", got)
	}

	// With no cover to take away there is nothing to do.
	m = toCover(t, send(t, m, m.Init()))
	m = keys(t, m, "enter", "enter")
	if m.editing {
		t.Error("an empty path with no cover to remove should just close the editor")
	}
}

func TestAMissingOrNotAnImageCoverIsExplainedAndNothingIsSaved(t *testing.T) {
	m, hook := toAlbum(t)
	m = toCover(t, m)
	m = typed(t, keys(t, m, "enter"), "/nowhere/front.png")
	m = keys(t, m, "enter")
	contains(t, m, "front.png", "no such file")
	if !m.editing || coverOf(t, hook.Library, "Flow") != "" {
		t.Errorf("editing %v, cover %q", m.editing, coverOf(t, hook.Library, "Flow"))
	}

	notImage := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(notImage, []byte("not a picture"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = typed(t, clearField(t, m), notImage)
	m = keys(t, m, "enter")
	if !m.editing || m.editErr == "" || coverOf(t, hook.Library, "Flow") != "" {
		t.Errorf("editing %v, error %q, cover %q", m.editing, m.editErr, coverOf(t, hook.Library, "Flow"))
	}
}

// toPlaylist is the model on the playlist Study (another, Gym, is there too),
// the cursor in the tree.
func toPlaylist(t *testing.T) (Model, Library) {
	t.Helper()
	lib, _ := playlistLib(t)
	if err := appOf(t, lib).CreatePlaylist("Gym"); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, lib, 120, 36)
	m = keys(t, m, "right", "down") // open the group of playlists, then Study
	if got := selectedLabel(t, m); got != "Study" {
		t.Fatalf("setup: %q", got)
	}
	return m, lib
}

func playlistsOf(t *testing.T, lib Library) []manifest.Playlist {
	t.Helper()
	pls, err := lib.Playlists()
	if err != nil {
		t.Fatal(err)
	}
	return pls
}

func TestRenamingAPlaylistInThePaneKeepsItsTracksAndSelection(t *testing.T) {
	m, lib := toPlaylist(t)
	m = keys(t, m, "enter", "right", "enter") // open Study, into its fields, edit the name
	if got := m.in[0].Value(); got != "Study" {
		t.Fatalf("the editor starts from %q", got)
	}
	m = typed(t, clearField(t, m), "Exams")
	m = keys(t, m, "enter")

	pls := playlistsOf(t, lib)
	if len(pls) != 2 || pls[0].Name != "Exams" || len(pls[0].Tracks) != 2 {
		t.Fatalf("playlists = %+v", pls)
	}
	contains(t, m, "Renamed Study to Exams")
	if m.focus != paneInspector || m.editing {
		t.Errorf("focus %v, editing %v: the pane should stay, closed", m.focus, m.editing)
	}
	if got := selectedLabel(t, m); got != "Exams" {
		t.Errorf("the cursor is on %q, want the renamed playlist", got)
	}
	if treeLine(m, "Flow") < 0 || treeLine(m, "Dream") < 0 {
		t.Errorf("the playlist closed with the rename:\n%s", screen(m))
	}
	if pls[0].NameLatin != "" {
		t.Errorf("the form worked out from the old name was kept as if set by hand: %q", pls[0].NameLatin)
	}
}

func TestOneUndoPutsTheOldNameOfAPlaylistBack(t *testing.T) {
	m, lib := toPlaylist(t)
	m = keys(t, m, "right", "enter")
	m = typed(t, clearField(t, m), "Exams")
	m = keys(t, m, "enter", "u")
	contains(t, m, "Undid")
	if pls := playlistsOf(t, lib); pls[0].Name != "Study" || len(pls[0].Tracks) != 2 {
		t.Errorf("after undo %+v", pls)
	}
	m = keys(t, m, "ctrl+r")
	if pls := playlistsOf(t, lib); pls[0].Name != "Exams" {
		t.Errorf("after redo %+v", pls)
	}
}

func TestANameThatIsAnotherPlaylistsIsRefusedUnderTheInputAndNothingChanges(t *testing.T) {
	m, lib := toPlaylist(t)
	m = keys(t, m, "right", "enter")
	m = typed(t, clearField(t, m), "gym")
	m = keys(t, m, "enter")
	if !m.editing {
		t.Fatal("the input closed although the name was refused")
	}
	contains(t, m, "already a playlist")
	if pls := playlistsOf(t, lib); pls[0].Name != "Study" || pls[1].Name != "Gym" {
		t.Errorf("playlists = %+v", pls)
	}
	m = keys(t, m, "esc")
	lacks(t, m, "already a playlist")
}

func TestThePlaylistNameCannotBeEmpty(t *testing.T) {
	m, lib := toPlaylist(t)
	m = keys(t, m, "right", "enter")
	m = keys(t, clearField(t, m), "enter")
	contains(t, m, "playlist's name cannot be empty")
	if pls := playlistsOf(t, lib); pls[0].Name != "Study" {
		t.Errorf("playlists = %+v", pls)
	}
}

func TestAPlaylistLatinNameFromThePane(t *testing.T) {
	m, lib := toPlaylist(t)
	m = keys(t, m, "right", "down") // the Latin name
	contains(t, m, "Latin name")
	m = keys(t, m, "enter")
	if got := m.in[0].Value(); got != "Study" {
		t.Fatalf("the editor starts from %q, want the form worked out", got)
	}
	m = keys(t, m, "enter") // left as it is: nothing was asked for
	if pls := playlistsOf(t, lib); pls[0].NameLatin != "" {
		t.Fatalf("the worked-out form was saved as if set by hand: %q", pls[0].NameLatin)
	}

	m = typed(t, clearField(t, keys(t, m, "enter")), "Study hall")
	m = keys(t, m, "enter")
	if pls := playlistsOf(t, lib); pls[0].Name != "Study" || pls[0].NameLatin != "Study hall" || len(pls[0].Tracks) != 2 {
		t.Fatalf("playlists = %+v", pls)
	}
	contains(t, m, "Latin name of Study is Study hall")

	// An empty text takes the form set by hand away, and undo puts it back.
	m = keys(t, m, "enter")
	m = keys(t, clearField(t, m), "enter")
	if pls := playlistsOf(t, lib); pls[0].NameLatin != "" {
		t.Errorf("an empty Latin name should be forgotten: %+v", pls[0])
	}
	m = keys(t, m, "u")
	if pls := playlistsOf(t, lib); pls[0].NameLatin != "Study hall" {
		t.Errorf("undo gave %+v", pls[0])
	}
}

func TestALatinNameThatWouldBeTheFileOfAnotherPlaylistIsRefusedInThePane(t *testing.T) {
	m, lib := toPlaylist(t)
	m = keys(t, m, "right", "down", "enter")
	m = typed(t, clearField(t, m), "gym")
	m = keys(t, m, "enter")
	contains(t, m, "same file")
	if !m.editing {
		t.Error("the input closed although the form was refused")
	}
	if pls := playlistsOf(t, lib); pls[0].NameLatin != "" {
		t.Errorf("playlists = %+v", pls)
	}
}

func TestEOnAnArtistAnAlbumAndAPlaylistGoesIntoTheirFieldsLikeRight(t *testing.T) {
	m, _ := toArtist(t)
	m = keys(t, m, "e")
	if m.overlay != nil || m.focus != paneInspector {
		t.Fatalf("on an artist: overlay %v, focus %v", m.overlay, m.focus)
	}
	contains(t, m, "Name", "Latin name", "Genre")
	lacks(t, m, "Released")

	m = keys(t, m, "esc", "down", "e") // down to its album
	if r, _ := m.selectedRow(); r.Kind != kindAlbum || m.overlay != nil || m.focus != paneInspector {
		t.Fatalf("on an album: row %v, overlay %v, focus %v", r.Kind, m.overlay, m.focus)
	}
	contains(t, m, "Released", "Tracks on album")

	p, _ := toPlaylist(t)
	p = keys(t, p, "e")
	if p.overlay != nil || p.focus != paneInspector {
		t.Fatalf("on a playlist: overlay %v, focus %v", p.overlay, p.focus)
	}
	contains(t, p, "Latin name")
	lacks(t, p, "Genre")
}

func TestEWithTracksMarkedStillEditsThemAllAtOnce(t *testing.T) {
	m, _ := toArtist(t)
	m = selectTrack(t, m, "Flow")
	m = keys(t, m, "space")
	for i := 0; i < 5; i++ { // back up to the artist
		if r, _ := m.selectedRow(); r.Kind == kindArtist {
			break
		}
		m = keys(t, m, "up")
	}
	if r, _ := m.selectedRow(); r.Kind != kindArtist {
		t.Fatalf("setup: on a %v", r.Kind)
	}
	m = keys(t, m, "e")
	if _, ok := m.overlay.(*bulkForm); !ok || m.focus != paneBrowser {
		t.Fatalf("overlay %T, focus %v: marked tracks should open the form", m.overlay, m.focus)
	}
	contains(t, m, "Edit 1 marked track")
}

// ---- the mouse

// clickRow clicks the row of the tree that has the label.
func clickRow(t *testing.T, m Model, label string) Model {
	t.Helper()
	i := treeLine(m, label)
	if i < 0 {
		t.Fatalf("no row %q in the tree:\n%s", label, screen(m))
	}
	return send(t, m, click(10, rowY(i)))
}

func TestClickingTheSelectedArtistAlbumOrPlaylistGoesIntoItsFields(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	now := time.Now()
	m.now = func() time.Time { return now }
	lib, _ := playlistLib(t)
	p := newTestModel(t, lib, 120, 36)
	p.now = m.now
	p = keys(t, p, "right") // open the group of playlists

	for _, c := range []struct {
		name string
		m    Model
		row  string
	}{{"an artist", m, "Shawn James"}, {"a playlist", p, "Study"}} {
		m := clickRow(t, c.m, c.row) // the first click selects
		if m.focus != paneBrowser {
			t.Errorf("%s: the first click went into the fields", c.name)
		}
		now = now.Add(2 * time.Second) // too slow to be a double click
		m = clickRow(t, m, c.row)
		if m.focus != paneInspector {
			t.Errorf("%s: clicking the selected row did not go into its fields", c.name)
		}
		if c.name == "an artist" && treeLine(m, "Shadows") >= 0 {
			t.Errorf("%s: the click opened the artist", c.name)
		}
	}

	a, _ := toAlbum(t)
	a.now = func() time.Time { return now }
	now = now.Add(2 * time.Second)
	a = clickRow(t, a, "Shadows")
	if a.focus != paneInspector {
		t.Error("an album: clicking the selected row did not go into its fields")
	}
	contains(t, a, "Tracks on album")
}

func TestDoubleClickStillOpensAndClosesAGroup(t *testing.T) {
	m, _ := toArtist(t) // the artist is selected, and open
	now := time.Now()
	m.now = func() time.Time { return now }
	now = now.Add(2 * time.Second)
	m = clickRow(t, m, "Shawn James")
	now = now.Add(100 * time.Millisecond)
	m = clickRow(t, m, "Shawn James")
	if m.focus != paneBrowser {
		t.Error("a double click ended in the fields")
	}
	if treeLine(m, "Shadows") >= 0 {
		t.Errorf("a double click did not close the artist:\n%s", screen(m))
	}
}

func TestClickingAFieldOfAGroupChoosesItAndAgainEditsIt(t *testing.T) {
	m, _ := toAlbum(t)
	m = keys(t, m, "right")
	l := computeLayout(m.w, m.h)
	for _, f := range []struct {
		label string
		field fieldKind
	}{{"Tracks on album", fAlbumOf}, {"Latin genre", fLatinGenre}, {"Released", fReleased}, {"Name", fAlbum}, {"Cover", fCover}} {
		y := fieldRow(t, m, f.label)
		m = send(t, m, click(l.left+8, y))
		if m.curField() != f.field {
			t.Fatalf("a click on the %s row chose %v", f.label, m.curField())
		}
		if m.editing {
			t.Errorf("the first click on %s only chooses the field", f.label)
		}
	}
	y := fieldRow(t, m, "Released")
	m = send(t, m, click(l.left+8, y))
	m = send(t, m, click(l.left+8, y))
	if !m.editing || m.curField() != fReleased {
		t.Errorf("clicking the chosen field should edit it: editing %v on %v", m.editing, m.curField())
	}
}

func TestAClickOnTheFieldsFromTheTreeGoesInWithoutEditing(t *testing.T) {
	m, _ := toArtist(t) // the cursor is in the tree
	l := computeLayout(m.w, m.h)
	y := fieldRow(t, m, "Genre")
	m = send(t, m, click(l.left+8, y))
	if m.focus != paneInspector || m.editing || m.curField() != fGenre {
		t.Errorf("focus %v, editing %v, on %v", m.focus, m.editing, m.curField())
	}
}

func TestAClickBelowTheFieldsOfAGroupDoesNotChooseOrEditAField(t *testing.T) {
	m, _ := toArtist(t)
	m = keys(t, m, "right")
	l := computeLayout(m.w, m.h)
	y := -1
	for i, line := range strings.Split(screen(m), "\n") {
		if cells := []rune(line); len(cells) > l.left && strings.Contains(string(cells[l.left:]), "Shadows") {
			y = i
		}
	}
	if y < 0 {
		t.Fatalf("no album in the list under the fields:\n%s", screen(m))
	}
	before := m.gfield
	m = send(t, m, click(l.left+8, y), click(l.left+8, y))
	if m.gfield != before || m.editing {
		t.Errorf("a click on the list under the fields chose field %d (was %d), editing %v", m.gfield, before, m.editing)
	}
}
