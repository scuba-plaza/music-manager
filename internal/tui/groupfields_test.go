package tui

import (
	"reflect"
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
