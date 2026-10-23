package app_test

import (
	"testing"

	"music-manager/internal/app"
)

// The Latin forms of the names are set by hand with Edit; "" takes one away,
// and the form worked out from the name is used again.

func TestEditSetsAndClearsTheLatinForms(t *testing.T) {
	a, tr := ingested(t)
	got, err := a.Edit(tr.ID, app.Edit{
		Title: ptr("وطني"), Artist: ptr("فيروز"), Album: ptr("روائع"), Genre: ptr("طرب"),
		TitleLatin: ptr(" Watani "), ArtistLatin: ptr("Fairouz"), AlbumLatin: ptr("Rawa'e"), GenreLatin: ptr("Tarab"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.TitleLatin != "Watani" || got.ArtistLatin != "Fairouz" || got.AlbumLatin != "Rawa'e" || got.GenreLatin != "Tarab" {
		t.Errorf("got %+v", got)
	}
	if saved := tracks(t, a)[0]; saved.ArtistLatin != "Fairouz" {
		t.Errorf("not saved: %+v", saved)
	}

	got, err = a.Edit(tr.ID, app.Edit{ArtistLatin: ptr("")})
	if err != nil {
		t.Fatal(err)
	}
	if got.ArtistLatin != "" || got.TitleLatin != "Watani" {
		t.Errorf("clearing one form changed %+v", got)
	}
	if got.LatinArtist() != "Firuz" {
		t.Errorf("after clearing, the worked-out form is %q", got.LatinArtist())
	}
}

func TestEditingTheTrueNameLeavesItsLatinFormAlone(t *testing.T) {
	a, tr := ingested(t)
	a.Edit(tr.ID, app.Edit{Title: ptr("وطني"), Artist: ptr("فيروز"), ArtistLatin: ptr("Fairouz")})
	got, err := a.Edit(tr.ID, app.Edit{Artist: ptr("فيروز ")})
	if err != nil || got.ArtistLatin != "Fairouz" {
		t.Errorf("%+v, %v", got, err)
	}
}

func TestALatinFormGoesWhenItsNameDoes(t *testing.T) {
	a, tr := ingested(t)
	a.Edit(tr.ID, app.Edit{
		Title: ptr("Song"), Artist: ptr("Band"), Album: ptr("روائع"), Genre: ptr("طرب"),
		AlbumLatin: ptr("Rawa'e"), GenreLatin: ptr("Tarab"),
	})
	got, err := a.Edit(tr.ID, app.Edit{Album: ptr(""), Genre: ptr("")})
	if err != nil {
		t.Fatal(err)
	}
	if got.AlbumLatin != "" || got.GenreLatin != "" {
		t.Errorf("Latin forms of names that are gone are kept: %+v", got)
	}
}

func TestALatinFormIsOneLine(t *testing.T) {
	a, tr := ingested(t)
	for _, e := range []app.Edit{
		{TitleLatin: ptr("two\nlines")}, {ArtistLatin: ptr("a\rb")}, {AlbumLatin: ptr("a\nb")}, {GenreLatin: ptr("a\nb")},
	} {
		if _, err := a.Edit(tr.ID, e); err == nil {
			t.Errorf("%+v was accepted", e)
		}
	}
}

func TestEditManyTakesTheLatinFormsToo(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	ts := tracks(t, a)
	for _, tr := range ts {
		identify(t, a, tr.ID, "Song "+tr.ID, "فيروز", "", 0, 0)
	}
	ts = tracks(t, a)
	got, err := a.EditMany(map[string]app.Edit{ts[0].ID: {ArtistLatin: ptr("Fairouz")}, ts[1].ID: {ArtistLatin: ptr("Fairouz")}})
	if err != nil || got[ts[0].ID].ArtistLatin != "Fairouz" || got[ts[1].ID].ArtistLatin != "Fairouz" {
		t.Errorf("%+v, %v", got, err)
	}
}

func TestChangingALatinFormIsNothingToSyncToTheLibraryFolder(t *testing.T) {
	a := libWithTracks(t, 10)
	identify(t, a, tracks(t, a)[0].ID, "وطني", "فيروز", "", 0, 0)
	mustSync(t, a, app.SyncOptions{})
	if n, _ := unsynced(t, a); n != 0 {
		t.Fatalf("%d to sync right after a sync", n)
	}
	if _, err := a.Edit(tracks(t, a)[0].ID, app.Edit{ArtistLatin: ptr("Fairouz"), TitleLatin: ptr("Watani")}); err != nil {
		t.Fatal(err)
	}
	if n, _ := unsynced(t, a); n != 0 {
		t.Errorf("a Latin form, which the library folder does not use, counts as %d to sync", n)
	}
}
