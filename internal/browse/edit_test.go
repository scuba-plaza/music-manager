package browse

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/testmp3"
)

type spec struct {
	title, artist, album string
	n, of                int
}

// seeded returns an editor on a library of identified tracks, one real audio
// file each, listed in the order given; a spec with no title is left unidentified.
func seeded(t *testing.T, specs ...spec) (*Editor, *app.App, []manifest.Track) {
	t.Helper()
	a, err := app.Open(filepath.Join(t.TempDir(), "lib"))
	if err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	for i := range specs {
		name := filepath.Join(src, string(rune('a'+i))+".mp3")
		if err := os.WriteFile(name, testmp3.Silent(300+i), 0o644); err != nil { // about 8 s; +i makes every file distinct
			t.Fatal(err)
		}
	}
	if _, err := a.Ingest([]string{src}); err != nil {
		t.Fatal(err)
	}
	m, _ := a.Manifest()
	for i, s := range specs {
		if s.title == "" {
			continue
		}
		e := app.Edit{Title: &s.title, Artist: &s.artist, Album: &s.album, TrackNumber: &s.n, TrackTotal: &s.of}
		if _, err := a.Edit(m.Tracks[i].ID, e); err != nil {
			t.Fatal(err)
		}
	}
	m, _ = a.Manifest()
	return NewEditor(a), a, m.Tracks
}

var shadows = []spec{
	{"The Wanderer", "Shawn James", "Shadows", 1, 3},
	{"Flow", "Shawn James", "Shadows", 2, 3},
	{"Along Our Way", "Shawn James", "Shadows", 3, 3},
	{"Sanarjiou", "Fairuz", "Jerusalem", 1, 0},
	{"", "", "", 0, 0},
}

func get(t *testing.T, a *app.App, id string) manifest.Track {
	t.Helper()
	m, err := a.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	tr, ok := m.Get(id)
	if !ok {
		t.Fatalf("no track %s", id)
	}
	return tr
}

func snap(t *testing.T, a *app.App) Snapshot {
	t.Helper()
	m, _ := a.Manifest()
	return Snapshot{Tracks: m.Tracks, Playlists: m.Playlists}
}

func TestAFieldIsSavedAndUndone(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	r, err := ed.SetField(ts[0].ID, "title", "The Wanderer (live)", "")
	if err != nil || r.Message != "Saved" || r.Reveal != ts[0].ID {
		t.Fatalf("%+v, %v", r, err)
	}
	if got := get(t, a, ts[0].ID).Title; got != "The Wanderer (live)" {
		t.Errorf("title %q", got)
	}
	if h := ed.History(); h.Undo != "change of title" || h.Redo != "" {
		t.Errorf("history %+v", h)
	}
	if r, err := ed.Undo(); err != nil || r.Message != "Undid: change of title" {
		t.Fatalf("%+v, %v", r, err)
	}
	if got := get(t, a, ts[0].ID).Title; got != "The Wanderer" {
		t.Errorf("after undo %q", got)
	}
	if _, err := ed.Redo(); err != nil || get(t, a, ts[0].ID).Title != "The Wanderer (live)" {
		t.Errorf("redo: %v", err)
	}
	if h := ed.History(); h.Redo != "" || h.Undo == "" {
		t.Errorf("history %+v", h)
	}
}

func TestATextLeftAsItWasSavesNothing(t *testing.T) {
	ed, _, ts := seeded(t, shadows...)
	r, err := ed.SetField(ts[0].ID, "title", "The Wanderer", "")
	if err != nil || !r.Same || r.Message != "" || ed.History().Undo != "" {
		t.Errorf("%+v, %v, %+v", r, err, ed.History())
	}
}

func TestWhatWouldBreakATrackIsRefused(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	for _, c := range []struct{ field, text, second, want string }{
		{"title", "  ", "", "cannot be empty"},
		{"artist", "", "", "cannot be empty"},
		{"track", "x", "", "whole number"},
		{"track", "1", "-3", "whole number"},
		{"released", "next year", "", "date"},
		{"volume", "loud", "", ""},
		{"cutStart", "soon", "", "Cut start"},
		{"nonsense", "x", "", "no field"},
		{"cover", "x", "", "no field"},
	} {
		if _, err := ed.SetField(ts[0].ID, c.field, c.text, c.second); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s=%q: %v, want %q", c.field, c.text, err, c.want)
		}
	}
	if _, err := ed.SetField("nope", "title", "x", ""); err == nil {
		t.Error("an unknown track was edited")
	}
	if got := get(t, a, ts[0].ID); got.Title != "The Wanderer" {
		t.Errorf("a refused edit changed the track: %+v", got)
	}
	if ed.History().Undo != "" {
		t.Error("a refused edit is on the undo stack")
	}
}

func TestATrackNobodyIdentifiedIsSavedEvenIfTheTextIsWhatTheTagsSaid(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	u := ts[4]
	u.Hint.Title, u.Hint.Artist = "Old", "Someone"
	if err := a.Restore(u); err != nil {
		t.Fatal(err)
	}
	if r, err := ed.SetField(u.ID, "title", "Old", ""); err != nil || r.Same {
		t.Fatalf("%+v, %v", r, err)
	}
	if got := get(t, a, u.ID); got.Title != "Old" || got.Status == manifest.Identified {
		t.Errorf("%+v", got)
	}
	if _, err := ed.SetField(u.ID, "artist", "Someone", ""); err != nil || get(t, a, u.ID).Status != manifest.Identified {
		t.Errorf("a title and an artist make it identified: %v", err)
	}
}

func TestALatinFormOfNothingCannotBeSet(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	single := ts[3] // Fairuz, album Jerusalem
	if _, err := ed.SetField(single.ID, "latinGenre", "Folk", ""); err == nil || !strings.Contains(err.Error(), "no genre to write in Latin") {
		t.Errorf("%v", err)
	}
	if _, err := ed.SetField(ts[0].ID, "latinTitle", "Al-Wanderer", ""); err != nil || get(t, a, ts[0].ID).TitleLatin != "Al-Wanderer" {
		t.Errorf("%v", err)
	}
	if _, err := ed.SetField(ts[0].ID, "latinTitle", "", ""); err != nil || get(t, a, ts[0].ID).TitleLatin != "" {
		t.Errorf("empty takes the form worked out back: %v", err)
	}
}

func TestNudgesOfOneCutAreOneStepToUndo(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	id := ts[0].ID
	for i := 0; i < 3; i++ {
		if _, err := ed.Nudge(id, "cutStart", 1, false); err != nil {
			t.Fatal(err)
		}
	}
	if got := get(t, a, id).TrimStart; got != 300*time.Millisecond {
		t.Errorf("start %v", got)
	}
	if _, err := ed.Nudge(id, "cutEnd", 1, true); err != nil {
		t.Fatal(err)
	}
	if got := get(t, a, id).TrimEnd; got != time.Second {
		t.Errorf("end %v", got)
	}
	ed.Undo() // the end
	if got := get(t, a, id); got.TrimEnd != 0 || got.TrimStart != 300*time.Millisecond {
		t.Errorf("one undo takes the end back: %v %v", got.TrimStart, got.TrimEnd)
	}
	ed.Undo() // the three nudges of the start, in one step
	if got := get(t, a, id).TrimStart; got != 0 {
		t.Errorf("start %v", got)
	}
	if _, err := ed.Nudge(id, "cutStart", -1, false); err != nil || ed.History().Undo != "" {
		t.Errorf("a cut cannot go below nothing: %v %+v", err, ed.History())
	}
}

func TestACutLeavesAtLeastABitOfTheTrack(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	id := ts[0].ID
	long := get(t, a, id).SourceLength
	for i := 0; i < 20; i++ {
		ed.Nudge(id, "cutStart", 1, true)
	}
	if got := get(t, a, id); long-got.TrimStart < minKept {
		t.Errorf("kept only %v", long-got.TrimStart)
	}
}

func TestVolumeAndFadesAreNudgedAndCleared(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	id := ts[0].ID
	ed.Nudge(id, "volume", 1, false)
	ed.Nudge(id, "volume", 1, true)
	if got := get(t, a, id).GainSteps; got != 5 {
		t.Errorf("steps %d", got)
	}
	ed.Nudge(id, "fadeIn", 1, true)
	if f := get(t, a, id).FadeIn; f.End-f.Start != time.Second {
		t.Errorf("fade in %+v", f)
	}
	ed.Nudge(id, "fadeOut", 1, false)
	if f := get(t, a, id).FadeOut; f.End-f.Start != 100*time.Millisecond {
		t.Errorf("fade out %+v", f)
	}
	for _, f := range []string{"volume", "fadeIn", "fadeOut"} {
		if _, err := ed.Clear(id, f); err != nil {
			t.Fatal(err)
		}
	}
	if got := get(t, a, id); got.GainSteps != 0 || got.FadeIn != (manifest.Span{}) || got.FadeOut != (manifest.Span{}) {
		t.Errorf("%+v", got)
	}
}

func TestACoverIsSetAndTakenAway(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	id := ts[0].ID
	if r, err := ed.SetCover([]string{id}, nil); err != nil || !r.Same {
		t.Errorf("no cover to remove: %+v %v", r, err)
	}
	if r, err := ed.SetCover([]string{id}, testmp3.PNG); err != nil || r.Message != "Saved" {
		t.Fatalf("%+v %v", r, err)
	}
	if get(t, a, id).CoverSHA256 == "" {
		t.Error("no cover")
	}
	if _, err := ed.SetCover([]string{id}, []byte("not a picture")); err == nil {
		t.Error("a file that is not a picture was taken")
	}
	if _, err := ed.SetCover([]string{id}, nil); err != nil || get(t, a, id).CoverSHA256 != "" {
		t.Errorf("%v", err)
	}
	ed.Undo()
	if get(t, a, id).CoverSHA256 == "" {
		t.Error("undo did not bring the cover back")
	}
}

func TestTheCoverOfAnAlbumGoesToAllItsTracks(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	key := albumRowKey("Shawn James", "Shadows")
	r, err := ed.SetGroupCover(key, testmp3.JPEG)
	if err != nil || r.Message != "Changed 3 tracks" {
		t.Fatalf("%+v %v", r, err)
	}
	for _, tr := range ts[:3] {
		if get(t, a, tr.ID).CoverSHA256 == "" {
			t.Errorf("%s has no cover", tr.Title)
		}
	}
	if get(t, a, ts[3].ID).CoverSHA256 != "" {
		t.Error("another artist's track got it")
	}
	if ed.History().Undo != "change of album cover" {
		t.Errorf("%+v", ed.History())
	}
	ed.Undo()
	for _, tr := range ts[:3] {
		if get(t, a, tr.ID).CoverSHA256 != "" {
			t.Error("undo is one step for the whole album")
		}
	}
}

func TestRemovingBringsTheTracksBackWithTheirPlaylists(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	if err := a.CreatePlaylist("Road", ts[0].ID, ts[3].ID); err != nil {
		t.Fatal(err)
	}
	r, err := ed.Remove([]string{ts[0].ID, ts[1].ID})
	if err != nil || !strings.HasPrefix(r.Message, "Removed 2 tracks") {
		t.Fatalf("%+v %v", r, err)
	}
	s := snap(t, a)
	if len(s.Tracks) != 3 || len(s.Playlists[0].Tracks) != 1 {
		t.Fatalf("%d tracks, playlist %v", len(s.Tracks), s.Playlists[0].Tracks)
	}
	if _, err := ed.Undo(); err != nil {
		t.Fatal(err)
	}
	s = snap(t, a)
	if len(s.Tracks) != 5 || len(s.Playlists[0].Tracks) != 2 || s.Playlists[0].Tracks[0] != ts[0].ID {
		t.Errorf("%d tracks, playlist %v", len(s.Tracks), s.Playlists[0].Tracks)
	}
	if _, err := ed.Redo(); err != nil || len(snap(t, a).Tracks) != 3 {
		t.Errorf("redo: %v", err)
	}
}

func TestAnArtistIsRenamedOnAllItsTracksAndTheRowsMove(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	key := artistKey("Shawn James")
	r, err := ed.SetGroupField(key, "artist", "Shawn Michael James")
	if err != nil || r.Message != "Changed 3 tracks" {
		t.Fatalf("%+v %v", r, err)
	}
	for _, tr := range ts[:3] {
		if get(t, a, tr.ID).Artist != "Shawn Michael James" {
			t.Errorf("%s: %s", tr.Title, get(t, a, tr.ID).Artist)
		}
	}
	if r.Moves[key] != artistKey("Shawn Michael James") || r.Moves[albumRowKey("Shawn James", "Shadows")] != albumRowKey("Shawn Michael James", "Shadows") {
		t.Errorf("moves %v", r.Moves)
	}
	u, err := ed.Undo()
	if err != nil || get(t, a, ts[2].ID).Artist != "Shawn James" {
		t.Errorf("undo is one step: %v", err)
	}
	if u.Moves[artistKey("Shawn Michael James")] != key {
		t.Errorf("the rows come back under their names: %v", u.Moves)
	}
	if r, err := ed.Redo(); err != nil || r.Moves[key] != artistKey("Shawn Michael James") {
		t.Errorf("redo: %v %v", r.Moves, err)
	}
}

func TestAGroupFieldThatIsMixedKeepsOrClears(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	g := "Folk"
	a.Edit(ts[0].ID, app.Edit{Genre: &g})
	key := albumRowKey("Shawn James", "Shadows")
	p, _ := Peek(snap(t, a), key, "")
	var genre FieldView
	for _, f := range p.Fields {
		if f.Key == "genre" {
			genre = f
		}
	}
	if !genre.Mixed || genre.Edit != "" || genre.Empty != "(mixed)" || !strings.Contains(genre.Hint, "the tracks differ") {
		t.Errorf("%+v", genre)
	}
	if r, err := ed.SetGroupField(key, "genre", ""); err != nil || !r.Same {
		t.Errorf("empty keeps the tracks as they are: %+v %v", r, err)
	}
	if _, err := ed.SetGroupField(key, "genre", "-"); err != nil || get(t, a, ts[0].ID).Genre != "" {
		t.Errorf("- clears: %v", err)
	}
	if _, err := ed.SetGroupField(key, "genre", "Jazz"); err != nil || get(t, a, ts[2].ID).Genre != "Jazz" {
		t.Errorf("%v", err)
	}
}

func TestAGroupNameCannotBeEmptyAndSaysWhenItJoinsAnother(t *testing.T) {
	ed, a, _ := seeded(t, shadows...)
	key := artistKey("Fairuz")
	if _, err := ed.SetGroupField(key, "artist", "  "); err == nil || !strings.Contains(err.Error(), "name cannot be empty") {
		t.Errorf("%v", err)
	}
	if _, err := ed.SetGroupField(key, "latinGenre", "x"); err == nil {
		t.Error("a Latin form of no genre was set")
	}
	if h := GroupHint(snap(t, a), key, "artist", "Shawn James"); h != "joins the artist Shawn James (3 tracks)" {
		t.Errorf("hint %q", h)
	}
	if h := GroupHint(snap(t, a), key, "artist", "Nobody"); h != "" {
		t.Errorf("hint %q", h)
	}
	if h := GroupHint(snap(t, a), albumRowKey("Shawn James", "Shadows"), "album", "Shadows"); h != "" {
		t.Errorf("the name it has already: %q", h)
	}
}

func TestAPlaylistIsRenamedAndGivenALatinName(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	a.CreatePlaylist("Road", ts[0].ID)
	a.CreatePlaylist("Home", ts[1].ID)
	key := playlistRowKey("Road")
	r, err := ed.SetGroupField(key, "playlistName", "Trip")
	if err != nil || r.Moves[key] != playlistRowKey("Trip") {
		t.Fatalf("%+v %v", r, err)
	}
	if _, err := ed.SetGroupField(playlistRowKey("Trip"), "playlistName", "Home"); err == nil {
		t.Error("two playlists of one name")
	}
	if _, err := ed.SetGroupField(playlistRowKey("Trip"), "playlistLatin", "Safar"); err != nil {
		t.Fatal(err)
	}
	if i, ok := manifest.PlaylistNamed(snap(t, a).Playlists, "Trip"); !ok || snap(t, a).Playlists[i].NameLatin != "Safar" {
		t.Error("no Latin name")
	}
	ed.Undo()
	if u, _ := ed.Undo(); u.Moves[playlistRowKey("Trip")] != key {
		t.Errorf("the playlist comes back under its name: %v", u.Moves)
	}
	if _, ok := manifest.PlaylistNamed(snap(t, a).Playlists, "Road"); !ok {
		t.Error("undo brings the name back")
	}
}

func TestSeveralTracksAreEditedTogether(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	ids := idsOf(ts[:3])
	v := BulkValues{Genre: "Folk", Of: "12", Album: "-"}
	p := PreviewBulk(snap(t, a), ids, v)
	if p.Changed != 3 || p.Error != "" || p.Summary != "Album → none (a single) · Tracks on album → 12 · Genre → Folk" {
		t.Errorf("%+v", p)
	}
	if r, err := ed.Bulk(ids, v); err != nil || r.Message != "Changed 3 tracks" {
		t.Fatalf("%+v %v", r, err)
	}
	if got := get(t, a, ids[1]); got.Genre != "Folk" || got.TrackTotal != 12 || got.Album != "" {
		t.Errorf("%+v", got)
	}
	if _, err := ed.Bulk(ids, v); err == nil || !strings.Contains(err.Error(), "Nothing to change") {
		t.Errorf("%v", err)
	}
	if p := PreviewBulk(snap(t, a), ids, BulkValues{Artist: "-"}); !strings.Contains(p.Error, "cannot be cleared") {
		t.Errorf("%+v", p)
	}
	if p := PreviewBulk(snap(t, a), ids, BulkValues{CutStart: "soon"}); !strings.Contains(p.Error, "Cut start") {
		t.Errorf("%+v", p)
	}
	ed.Undo()
	if got := get(t, a, ids[1]); got.Genre != "" || got.Album != "Shadows" {
		t.Errorf("one undo step: %+v", got)
	}
}

func TestWhatWasCopiedIsPastedOnOtherTracks(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	if _, err := ed.Paste([]string{ts[3].ID}); err == nil {
		t.Error("pasted with nothing copied")
	}
	if _, err := ed.CopyField(ts[0].ID, "title"); err == nil || !strings.Contains(err.Error(), "belongs to its track") {
		t.Errorf("%v", err)
	}
	if _, err := ed.CopyField(ts[0].ID, "genre"); err == nil || !strings.Contains(err.Error(), "no genre to copy") {
		t.Errorf("%v", err)
	}
	if _, err := ed.Copy(ts[0].ID, []string{"artist", "album", "albumOf"}); err != nil {
		t.Fatal(err)
	}
	if h := ed.History(); h.Clip != "artist, album and tracks on album" {
		t.Errorf("%+v", h)
	}
	if r, err := ed.Paste([]string{ts[3].ID, ts[0].ID}); err != nil || !strings.HasPrefix(r.Message, "Pasted the artist, album and tracks on album on 1 track") {
		t.Fatalf("%+v %v", r, err)
	}
	if got := get(t, a, ts[3].ID); got.Artist != "Shawn James" || got.Album != "Shadows" || got.TrackTotal != 3 || got.Title != "Sanarjiou" {
		t.Errorf("%+v", got)
	}
	if r, err := ed.Paste([]string{ts[3].ID}); err != nil || !r.Same || !strings.Contains(r.Message, "already has the artist") {
		t.Errorf("%+v %v", r, err)
	}
}

func TestAPictureIsPastedAsTheCoverOfTracks(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	r, err := ed.PastePicture(idsOf(ts[:2]), testmp3.PNG, "front.png")
	if err != nil || r.Message != "Pasted the cover from front.png on 2 tracks · u undoes it" {
		t.Fatalf("%+v %v", r, err)
	}
	if r, err := ed.PastePicture(idsOf(ts[:2]), testmp3.PNG, ""); err != nil || !r.Same {
		t.Errorf("%+v %v", r, err)
	}
	if _, err := ed.PastePicture(idsOf(ts[:2]), []byte("no"), ""); err == nil {
		t.Error("a file that is not a picture")
	}
	if get(t, a, ts[0].ID).CoverSHA256 == "" {
		t.Error("no cover")
	}
}

func TestATrackMovesUpAndDownItsAlbumByTradingNumbers(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	if r, err := ed.MoveInAlbum(ts[1].ID, 1); err != nil || r.Message != "Moved" {
		t.Fatalf("%+v %v", r, err)
	}
	if get(t, a, ts[1].ID).TrackNumber != 3 || get(t, a, ts[2].ID).TrackNumber != 2 {
		t.Error("numbers not traded")
	}
	if r, err := ed.MoveInAlbum(ts[1].ID, 1); err != nil || r.Message != "It is the last track already" || !r.Same {
		t.Errorf("%+v %v", r, err)
	}
	if r, _ := ed.MoveInAlbum(ts[0].ID, -1); r.Message != "It is the first track already" {
		t.Errorf("%+v", r)
	}
	if _, err := ed.MoveInAlbum(ts[4].ID, 1); err == nil {
		t.Error("a track nobody identified has no album to move in")
	}
	n := 0
	a.Edit(ts[0].ID, app.Edit{TrackNumber: &n})
	if _, err := ed.MoveInAlbum(ts[0].ID, -1); err == nil || !strings.Contains(err.Error(), "Number the album") { // unnumbered tracks are listed last
		t.Errorf("%v", err)
	}
}

func TestAnAlbumIsNumberedInTheOrderItIsListed(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	zero := 0
	for _, tr := range ts[:3] {
		a.Edit(tr.ID, app.Edit{TrackNumber: &zero})
	}
	r, err := ed.Renumber(albumRowKey("Shawn James", "Shadows"))
	if err != nil || r.Message != "Numbered 3 tracks 1–3" {
		t.Fatalf("%+v %v", r, err)
	}
	// unnumbered tracks are listed by title: Along Our Way, Flow, The Wanderer
	for i, id := range []string{ts[2].ID, ts[1].ID, ts[0].ID} {
		if got := get(t, a, id).TrackNumber; got != i+1 {
			t.Errorf("%s is %d", get(t, a, id).Title, got)
		}
	}
	if _, err := ed.Renumber(singlesKey("Fairuz")); err == nil {
		t.Error("the singles are not an album")
	}
}

func TestPlaylistsAreMadeFilledReorderedAndUndone(t *testing.T) {
	ed, a, ts := seeded(t, shadows...)
	if r, err := ed.PlaylistNew("Road", idsOf(ts[:2])); err != nil || !strings.HasPrefix(r.Message, "Made Road with 2 tracks") {
		t.Fatalf("%+v %v", r, err)
	}
	if r, err := ed.PlaylistAdd("Road", idsOf(ts[1:4])); err != nil || !strings.Contains(r.Message, "Added 2 tracks to Road (1 already there)") {
		t.Fatalf("%+v %v", r, err)
	}
	if r, err := ed.MoveInPlaylist("Road", ts[0].ID, 1); err != nil || r.Reveal != ts[0].ID {
		t.Fatalf("%+v %v", r, err)
	}
	if got := snap(t, a).Playlists[0].Tracks; got[0] != ts[1].ID || got[1] != ts[0].ID {
		t.Errorf("%v", got)
	}
	if r, _ := ed.MoveInPlaylist("Road", ts[3].ID, 1); r.Message != "It is the last track already" {
		t.Errorf("%+v", r)
	}
	if r, err := ed.PlaylistTakeOut("Road", []string{ts[2].ID}); err != nil || !strings.Contains(r.Message, "Took “Along Our Way” out of Road") {
		t.Fatalf("%+v %v", r, err)
	}
	if _, err := ed.PlaylistNew("Road", nil); err == nil {
		t.Error("a second playlist of that name")
	}
	if _, err := ed.PlaylistDelete("Road"); err != nil || len(snap(t, a).Playlists) != 0 {
		t.Fatal(err)
	}
	ed.Undo() // the deletion
	if len(snap(t, a).Playlists) != 1 || len(snap(t, a).Playlists[0].Tracks) != 3 {
		t.Errorf("%+v", snap(t, a).Playlists)
	}
}

func TestForgettingEmptiesTheHistory(t *testing.T) {
	ed, _, ts := seeded(t, shadows...)
	ed.SetField(ts[0].ID, "title", "x", "")
	ed.Forget()
	if h := ed.History(); h.Undo != "" {
		t.Errorf("%+v", h)
	}
	if r, _ := ed.Undo(); r.Message != "Nothing to undo" {
		t.Errorf("%+v", r)
	}
	if r, _ := ed.Redo(); r.Message != "Nothing to redo" {
		t.Errorf("%+v", r)
	}
}

func TestThePathShownWhileTypingIsWhereTheFileWillGo(t *testing.T) {
	_, a, ts := seeded(t, shadows...)
	p, ok := PathPreview(snap(t, a), ts[0].ID, "title", "Wanderer II", "")
	if !ok || p != "Shawn James/Shadows/01 - Wanderer II.mp3" {
		t.Errorf("%q %v", p, ok)
	}
	if _, ok := PathPreview(snap(t, a), "nope", "title", "x", ""); ok {
		t.Error("a track that is not there")
	}
}

// database is a music database that knows two recordings.
type database struct {
	found []app.Candidate
	asked []app.Query
}

func (d *database) Search(_ context.Context, q app.Query) ([]app.Candidate, error) {
	d.asked = append(d.asked, q)
	return d.found, nil
}
func (d *database) CoverFront(context.Context, string) ([]byte, error) { return nil, nil }

func withDatabase(t *testing.T, found ...app.Candidate) (*Editor, *app.App, []manifest.Track, *database) {
	t.Helper()
	_, a, ts := seeded(t, shadows...)
	db := &database{found: found}
	return NewEditor(a, WithSource(db)), a, ts, db
}

func TestATrackIsLookedUpAndIdentifiedAsOneOfWhatWasFound(t *testing.T) {
	ed, a, ts, db := withDatabase(t,
		app.Candidate{Title: "Flow", Artist: "Shawn James", Album: "Through the Haze", TrackNumber: 2, TrackTotal: 9, Score: 100, Date: "2012"},
		app.Candidate{Title: "Flow (live)", Artist: "Shawn James", Album: "Live", TrackNumber: 5, TrackTotal: 12, Score: 90})
	u := ts[4] // nobody identified it
	v, err := ed.Candidates(context.Background(), u.ID, nil)
	if err != nil || len(v.Candidates) != 2 || v.Search == "" {
		t.Fatalf("%+v %v", v, err)
	}
	if v.Candidates[0].Text != "100%  Through the Haze · 2012 · 2/9 · Shawn James" || !strings.HasSuffix(v.Candidates[1].Text, "· Flow (live)") {
		t.Errorf("%q / %q", v.Candidates[0].Text, v.Candidates[1].Text)
	}
	d := v.Candidates[0].Diff
	if len(d) != 4 || d[0].Label != "Title" || d[0].New != "Flow" || d[2].New != "Through the Haze" || d[3].New != "2 of 9" {
		t.Errorf("%+v", d)
	}
	if len(db.asked) != 1 {
		t.Errorf("asked %v", db.asked)
	}
	r, err := ed.ApplyCandidate(context.Background(), v.Search, 1)
	if err != nil || r.Message != "Identified as Shawn James - Flow (live)" || r.Reveal != u.ID {
		t.Fatalf("%+v %v", r, err)
	}
	if got := get(t, a, u.ID); got.Status != manifest.Identified || got.Album != "Live" || got.TrackNumber != 5 {
		t.Errorf("%+v", got)
	}
	if ed.History().Undo != "identification of a track" {
		t.Errorf("%+v", ed.History())
	}
	ed.Undo()
	if got := get(t, a, u.ID); got.Status == manifest.Identified {
		t.Error("undo leaves it identified")
	}
}

func TestOtherWordsAreSearchedForAsTheyWereTyped(t *testing.T) {
	ed, _, ts, db := withDatabase(t, app.Candidate{Title: "Sanarjiou", Artist: "Fairuz", Score: 100})
	v, err := ed.Candidates(context.Background(), ts[4].ID, &app.Query{Title: " Sanarjiou ", Artist: "Fairuz "})
	if err != nil || v.Title != "Sanarjiou" || v.Artist != "Fairuz" {
		t.Fatalf("%+v %v", v, err)
	}
	if len(db.asked) != 1 || db.asked[0].Title != "Sanarjiou" || db.asked[0].Artist != "Fairuz" {
		t.Errorf("asked %+v", db.asked)
	}
}

func TestNothingFoundSaysWhatWasSearchedFor(t *testing.T) {
	ed, _, ts, _ := withDatabase(t)
	if _, err := ed.Candidates(context.Background(), ts[0].ID, nil); err == nil || !strings.Contains(err.Error(), "No match found for “The Wanderer – Shawn James”") {
		t.Errorf("%v", err)
	}
	if _, err := NewEditor(ed.lib).Candidates(context.Background(), ts[0].ID, nil); !errors.Is(err, ErrNoLookup) {
		t.Errorf("no database: %v", err)
	}
	if _, err := ed.ApplyCandidate(context.Background(), "nope", 0); err == nil || !strings.Contains(err.Error(), "That search is gone") {
		t.Errorf("%v", err)
	}
}

func TestAnOldSearchIsForgottenAfterManyNewOnes(t *testing.T) {
	ed, _, ts, _ := withDatabase(t, app.Candidate{Title: "x", Artist: "y", Score: 100})
	first, _ := ed.Candidates(context.Background(), ts[0].ID, nil)
	for i := 0; i < maxSearches; i++ {
		ed.Candidates(context.Background(), ts[0].ID, nil)
	}
	if len(ed.searches) > maxSearches {
		t.Errorf("%d searches kept", len(ed.searches))
	}
	if _, err := ed.ApplyCandidate(context.Background(), first.Search, 0); err == nil {
		t.Error("the oldest search is still there")
	}
}

func TestEveryTrackThatNeedsWorkIsIdentifiedInOneStepToUndo(t *testing.T) {
	_, a, ts := seeded(t, spec{}, spec{}, spec{"Flow", "Shawn James", "Shadows", 2, 3})
	db := &database{found: []app.Candidate{{Title: "Found", Artist: "Someone", Album: "Somewhere", TrackNumber: 1, TrackTotal: 1, Score: 99}}}
	ed := NewEditor(a, WithSource(db))
	var seen []IdentifyProgress
	p, err := ed.IdentifyAll(context.Background(), func(p IdentifyProgress) { seen = append(seen, p) })
	if err != nil || p.Total != 2 || p.Done != 2 || p.Identified != 2 || len(seen) != 2 || seen[0].Done != 1 {
		t.Fatalf("%+v %v seen %d", p, err, len(seen))
	}
	if get(t, a, ts[0].ID).Status != manifest.Identified || get(t, a, ts[1].ID).Title != "Found" {
		t.Error("not identified")
	}
	if ed.History().Undo != "identification of 2 tracks" {
		t.Errorf("%+v", ed.History())
	}
	ed.Undo()
	if get(t, a, ts[0].ID).Status == manifest.Identified || get(t, a, ts[1].ID).Status == manifest.Identified {
		t.Error("one undo takes both back")
	}
}

func TestWhatIsNotConfidentIsLeftAndACancelledRunKeepsWhatItDid(t *testing.T) {
	_, a, ts := seeded(t, spec{}, spec{})
	db := &database{found: []app.Candidate{{Title: "Maybe", Artist: "Someone", Score: 70}}}
	ed := NewEditor(a, WithSource(db))
	p, err := ed.IdentifyAll(context.Background(), func(IdentifyProgress) {})
	if err != nil || p.Left != 2 || p.Identified != 0 || ed.History().Undo != "" {
		t.Errorf("%+v %v %+v", p, err, ed.History())
	}
	db.found[0].Score = 99
	ctx, cancel := context.WithCancel(context.Background())
	p, _ = ed.IdentifyAll(ctx, func(p IdentifyProgress) {
		if p.Done == 1 {
			cancel()
		}
	})
	if p.Identified < 1 || ed.History().Undo == "" {
		t.Errorf("what it did before it was stopped is kept: %+v %+v", p, ed.History())
	}
	_ = ts
}

func TestOldTagsAreAdoptedForTracksThatHaveThem(t *testing.T) {
	ed, a, ts := seeded(t, spec{}, spec{})
	if _, err := ed.Adopt([]string{ts[0].ID}); err == nil || !strings.Contains(err.Error(), "no old tags") {
		// the test files carry no tags: give one a hint, as ingesting a tagged file would
		t.Logf("untagged: %v", err)
	}
	u := get(t, a, ts[0].ID)
	u.Hint.Title, u.Hint.Artist, u.Hint.Album = "Old title", "Old artist", "Old album"
	if err := a.Restore(u); err != nil {
		t.Fatal(err)
	}
	r, err := ed.Adopt([]string{ts[0].ID, ts[1].ID})
	if err != nil || r.Message != "Used the old tags of 1 track · 1 had none" || r.Reveal != ts[0].ID {
		t.Fatalf("%+v %v", r, err)
	}
	if got := get(t, a, ts[0].ID); got.Title != "Old title" || got.Status != manifest.Identified {
		t.Errorf("%+v", got)
	}
	ed.Undo()
	if get(t, a, ts[0].ID).Status == manifest.Identified {
		t.Error("undo")
	}
	if _, err := ed.Adopt([]string{"nope"}); err == nil || !strings.Contains(err.Error(), "Select a track that needs work") {
		t.Errorf("%v", err)
	}
}
