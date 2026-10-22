package prompt_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/prompt"
)

func ask(t *testing.T, input string, q app.Question) (string, string, error) {
	t.Helper()
	var out bytes.Buffer
	if q.Validate == nil {
		q.Validate = func(string) error { return nil }
	}
	got, err := prompt.NewPlain(strings.NewReader(input), &out).Ask(q)
	return got, out.String(), err
}

func TestPlainEmptyLineTakesTheDefault(t *testing.T) {
	got, out, err := ask(t, "\n", app.Question{Label: "Title", Default: "Song"})
	if err != nil || got != "Song" {
		t.Errorf("got %q, %v", got, err)
	}
	if !strings.Contains(out, "Title") || !strings.Contains(out, "[Song]") {
		t.Errorf("prompt %q should show the label and the default", out)
	}
}

func TestPlainTypedAnswerReplacesTheDefault(t *testing.T) {
	if got, _, _ := ask(t, "  Typed answer \n", app.Question{Label: "Title", Default: "Song"}); got != "Typed answer" {
		t.Errorf("got %q", got)
	}
}

func TestPlainDashClearsAFieldThatHasADefault(t *testing.T) {
	if got, _, err := ask(t, "-\n", app.Question{Label: "Album", Default: "Yt Album"}); err != nil || got != "" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestPlainWithoutDefaultAcceptsAnEmptyAnswer(t *testing.T) {
	if got, out, err := ask(t, "\n", app.Question{Label: "Trim"}); err != nil || got != "" || strings.Contains(out, "[") {
		t.Errorf("got %q, %v, prompt %q", got, err, out)
	}
}

func TestPlainAsksAgainUntilTheAnswerIsValid(t *testing.T) {
	q := app.Question{Label: "Track", Validate: func(s string) error {
		if s == "bad" {
			return errors.New("enter a number")
		}
		return nil
	}}
	got, out, err := ask(t, "bad\nbad\n7\n", q)
	if err != nil || got != "7" {
		t.Errorf("got %q, %v", got, err)
	}
	if strings.Count(out, "enter a number") != 2 {
		t.Errorf("the error should be shown for each bad answer:\n%s", out)
	}
}

func TestPlainValidatesTheDefaultToo(t *testing.T) {
	q := app.Question{Label: "Title", Default: "", Required: true, Validate: func(s string) error {
		if s == "" {
			return errors.New("required")
		}
		return nil
	}}
	if got, _, err := ask(t, "\n\nGiven\n", q); err != nil || got != "Given" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestPlainEndOfInputAborts(t *testing.T) {
	if _, _, err := ask(t, "", app.Question{Label: "Title"}); !errors.Is(err, app.ErrAborted) {
		t.Errorf("err = %v, want ErrAborted", err)
	}
	// Input that ends without a newline still counts as an answer.
	if got, _, err := ask(t, "last words", app.Question{Label: "Title"}); err != nil || got != "last words" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestPlainKeepsReadingFromTheSameInputAcrossQuestions(t *testing.T) {
	var out bytes.Buffer
	p := prompt.NewPlain(strings.NewReader("one\ntwo\nthree\n"), &out)
	q := app.Question{Label: "x", Validate: func(string) error { return nil }}
	for _, want := range []string{"one", "two", "three"} {
		if got, err := p.Ask(q); err != nil || got != want {
			t.Errorf("got %q, %v; want %q", got, err, want)
		}
	}
}

func TestPlainChoose(t *testing.T) {
	opts := []string{"first", "second", "skip"}
	choose := func(input string) (int, string, error) {
		var out bytes.Buffer
		i, err := prompt.NewPlain(strings.NewReader(input), &out).Choose("Which?", opts)
		return i, out.String(), err
	}
	if i, out, err := choose("2\n"); err != nil || i != 1 || !strings.Contains(out, "Which?") || !strings.Contains(out, "2) second") {
		t.Errorf("got %d, %v, %q", i, err, out)
	}
	if i, _, err := choose("\n"); err != nil || i != 0 {
		t.Errorf("an empty line should pick the first: %d, %v", i, err)
	}
	if i, out, err := choose("0\nnine\n4\n3\n"); err != nil || i != 2 || strings.Count(out, "enter a number from 1 to 3") != 3 {
		t.Errorf("bad answers should be asked again: %d, %v, %q", i, err, out)
	}
	if _, _, err := choose(""); !errors.Is(err, app.ErrAborted) {
		t.Errorf("err = %v, want ErrAborted", err)
	}
}
