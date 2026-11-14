package app_test

import (
	"errors"
	"fmt"
	"testing"

	"music-manager/internal/app"
)

func TestInvalidNamesTheFieldAndSaysItsMessage(t *testing.T) {
	err := app.Invalid("released", "not a date")

	if err.Error() != "not a date" {
		t.Errorf("Error() = %q", err.Error())
	}
	var fields app.FieldErrors
	if !errors.As(fmt.Errorf("saving: %w", err), &fields) || fields["released"] != "not a date" {
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
