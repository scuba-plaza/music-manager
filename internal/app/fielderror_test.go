package app_test

import (
	"errors"
	"fmt"
	"testing"

	"music-manager/internal/app"
)

func TestInvalidNamesTheFieldAndSaysItsMessage(t *testing.T) {
	err := app.Invalid("released", "Not a date")

	if err.Error() != "Not a date" {
		t.Errorf("Error() = %q", err.Error())
	}
	var fields app.FieldErrors
	if !errors.As(fmt.Errorf("saving: %w", err), &fields) || fields["released"] != "Not a date" {
		t.Errorf("errors.As found %v", fields)
	}
}

func TestSeveralFieldErrorsAreReadAsHowManyNeedFixing(t *testing.T) {
	errs := app.FieldErrors{"title": "Title cannot be empty", "artist": "Artist cannot be empty"}

	if errs.Error() != "2 fields need fixing" {
		t.Errorf("Error() = %q", errs.Error())
	}
}

func TestFieldErrorsOfNothingIsNoError(t *testing.T) {
	var errs app.FieldErrors

	if got := errs.OrNil(); got != nil {
		t.Errorf("OrNil() = %v, want nil", got)
	}
	errs = app.FieldErrors{"title": "x"}
	var found app.FieldErrors
	if !errors.As(errs.OrNil(), &found) || found["title"] != "x" {
		t.Errorf("OrNil() = %v", errs.OrNil())
	}
}

func TestAMessageUnderAFieldStartsWithACapital(t *testing.T) {
	errs := app.FieldErrors{}

	errs.Set("released", `release date "soon" is not a date`)
	errs.Set("title", "élan is not a title")
	errs.Set("artist", "")
	errs.Set("album", "Already fine")

	want := app.FieldErrors{"released": `Release date "soon" is not a date`, "title": "Élan is not a title", "artist": "", "album": "Already fine"}
	for key, message := range want {
		if errs[key] != message {
			t.Errorf("%s: %q, want %q", key, errs[key], message)
		}
	}
}
