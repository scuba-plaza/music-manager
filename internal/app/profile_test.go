package app_test

import (
	"errors"
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/device"
	"music-manager/internal/manifest"
)

func player(name string) manifest.Profile {
	return manifest.Profile{
		Name: name, ComponentMax: 100, CoverFormat: "keep", ReplayGain: true,
		PlaylistExt: ".m3u8", PlaylistStyle: "relative",
	}
}

func openEmpty(t *testing.T) *app.App {
	t.Helper()
	a, err := app.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestALibraryStartsWithNoFormatsBesidesGeneric(t *testing.T) {
	a := openEmpty(t)

	got, err := a.Profiles()

	if err != nil || len(got) != 0 {
		t.Errorf("%+v %v", got, err)
	}
}

func TestAFormatIsMadeWithAnIDOfItsOwnAndATrimmedName(t *testing.T) {
	a := openEmpty(t)

	made, err := a.CreateProfile(player("  My Player  "))

	if err != nil || made.ID != "my-player" || made.Name != "My Player" {
		t.Fatalf("%+v %v", made, err)
	}
	if got, _ := a.Profiles(); len(got) != 1 || got[0] != made {
		t.Errorf("%+v", got)
	}
}

func TestTheIDOfAFormatDoesNotChangeWhenItIsRenamed(t *testing.T) {
	a := openEmpty(t)
	made, _ := a.CreateProfile(player("My Player"))
	edited := player("Car")

	got, err := a.UpdateProfile(made.ID, edited)

	if err != nil || got.ID != made.ID || got.Name != "Car" {
		t.Errorf("%+v %v", got, err)
	}
}

func TestFormatsWhoseNamesMakeTheSameIDStillGetIDsOfTheirOwn(t *testing.T) {
	a := openEmpty(t)

	first, _ := a.CreateProfile(player("My Player"))
	second, _ := a.CreateProfile(player("my player!"))
	arabic, _ := a.CreateProfile(player("مشغل"))
	accent, _ := a.CreateProfile(player("Müller"))

	if first.ID != "my-player" || second.ID != "my-player-2" {
		t.Errorf("%q %q", first.ID, second.ID)
	}
	if arabic.ID == "" || arabic.ID == accent.ID || accent.ID != "muller" {
		t.Errorf("%q %q", arabic.ID, accent.ID)
	}
}

func TestAFormatThatIsWrongIsRefusedBySetting(t *testing.T) {
	a := openEmpty(t)
	p := player("")
	p.ComponentMax = 5
	p.PlaylistExt = ".pls"

	_, err := a.CreateProfile(p)

	var fields app.FieldErrors
	if !errors.As(err, &fields) || fields["name"] == "" || fields["component_max"] == "" || fields["playlist_ext"] == "" || len(fields) != 3 {
		t.Errorf("%v", err)
	}
	if got, _ := a.Profiles(); len(got) != 0 {
		t.Errorf("something was stored: %+v", got)
	}
}

func TestTwoFormatsCannotHaveTheSameNameWhateverTheCase(t *testing.T) {
	a := openEmpty(t)
	a.CreateProfile(player("My Player"))

	_, err := a.CreateProfile(player("my PLAYER"))

	var fields app.FieldErrors
	if !errors.As(err, &fields) || !strings.Contains(fields["name"], "already") {
		t.Errorf("%v", err)
	}
}

func TestAFormatMayBeSavedUnderItsOwnNameAgain(t *testing.T) {
	a := openEmpty(t)
	made, _ := a.CreateProfile(player("My Player"))
	changed := player("my player")
	changed.ComponentMax = 120

	got, err := a.UpdateProfile(made.ID, changed)

	if err != nil || got.ComponentMax != 120 || got.Name != "my player" {
		t.Errorf("%+v %v", got, err)
	}
}

func TestNoMoreThanEightFormatsCanBeMade(t *testing.T) {
	a := openEmpty(t)
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		if _, err := a.CreateProfile(player(name)); err != nil {
			t.Fatal(err)
		}
	}

	_, err := a.CreateProfile(player("i"))

	if err == nil || !strings.Contains(err.Error(), "8") {
		t.Errorf("%v", err)
	}
	var fields app.FieldErrors
	if errors.As(err, &fields) {
		t.Errorf("a full list is not the fault of a field: %v", fields)
	}
}

func TestAKeptCoverForgetsItsSizeAndQuality(t *testing.T) {
	a := openEmpty(t)
	p := player("My Player")
	p.CoverMaxEdge, p.CoverQuality = 480, 85

	made, err := a.CreateProfile(p)

	if err != nil || made.CoverMaxEdge != 0 || made.CoverQuality != 0 {
		t.Errorf("%+v %v", made, err)
	}
}

func TestAFormatThatIsGoneCannotBeChangedOrDeleted(t *testing.T) {
	a := openEmpty(t)

	if _, err := a.UpdateProfile("nope", player("x")); err == nil {
		t.Error("updated")
	}
	if err := a.DeleteProfile("nope"); err == nil {
		t.Error("deleted")
	}
}

func TestAFormatIsDeleted(t *testing.T) {
	a := openEmpty(t)
	made, _ := a.CreateProfile(player("My Player"))

	if err := a.DeleteProfile(made.ID); err != nil {
		t.Fatal(err)
	}

	if got, _ := a.Profiles(); len(got) != 0 {
		t.Errorf("%+v", got)
	}
}

func TestGenericIsAlwaysThereAndNothingElseIsCalledGeneric(t *testing.T) {
	a := openEmpty(t)

	if target, err := a.Format("generic"); err != nil || target.ID() != device.Generic.ID() {
		t.Errorf("%v %v", target, err)
	}
	if _, err := a.CreateProfile(player("generic")); err == nil {
		t.Error("a format called generic was made")
	}
	if _, err := a.Format("nope"); err == nil {
		t.Error("a format that is not there was found")
	}
	made, _ := a.CreateProfile(player("My Player"))
	if target, err := a.Format(made.ID); err != nil || target.Profile().Name != "My Player" {
		t.Errorf("%v %v", target, err)
	}
}
