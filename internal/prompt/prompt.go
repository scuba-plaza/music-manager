// Package prompt asks the user questions in the terminal.
package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"

	"music-manager/internal/app"
)

// New returns the best prompter for the current terminal: a full-screen form
// normally, plain line-based questions if ACCESSIBLE is set in the
// environment or the terminal is dumb.
func New(in io.Reader, out io.Writer) app.Prompter {
	if os.Getenv("ACCESSIBLE") != "" || os.Getenv("TERM") == "dumb" {
		return NewPlain(in, out)
	}
	return form{}
}

// form asks with an editable text field that starts out holding the default.
type form struct{}

func (form) Ask(q app.Question) (string, error) {
	answer := q.Default
	field := huh.NewInput().Title(q.Label).Value(&answer).Validate(q.Validate)
	if err := huh.NewForm(huh.NewGroup(field)).Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", app.ErrAborted
		}
		return "", err
	}
	return answer, nil
}

func (form) Choose(title string, options []string) (int, error) {
	var picked int
	opts := make([]huh.Option[int], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, i)
	}
	sel := huh.NewSelect[int]().Title(title).Options(opts...).Value(&picked)
	if err := huh.NewForm(huh.NewGroup(sel)).Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return 0, app.ErrAborted
		}
		return 0, err
	}
	return picked, nil
}

// Plain asks line-based questions: "Label [default]: ". An empty line takes
// the default, "-" clears it, and invalid answers are asked again.
type Plain struct {
	in  *bufio.Reader
	out io.Writer
}

func NewPlain(in io.Reader, out io.Writer) *Plain {
	return &Plain{in: bufio.NewReader(in), out: out}
}

// Choose lists the options by number and reads the number of the one wanted;
// an empty line means the first.
func (p *Plain) Choose(title string, options []string) (int, error) {
	fmt.Fprintln(p.out, title)
	for i, o := range options {
		fmt.Fprintf(p.out, "  %d) %s\n", i+1, o)
	}
	for {
		fmt.Fprintf(p.out, "Choose [1]: ")
		line, err := p.in.ReadString('\n')
		if err != nil && (err != io.EOF || line == "") {
			fmt.Fprintln(p.out)
			return 0, app.ErrAborted
		}
		line = strings.TrimSpace(line)
		if line == "" {
			return 0, nil
		}
		if n, err := strconv.Atoi(line); err == nil && n >= 1 && n <= len(options) {
			return n - 1, nil
		}
		fmt.Fprintf(p.out, "  enter a number from 1 to %d\n", len(options))
	}
}

func (p *Plain) Ask(q app.Question) (string, error) {
	for {
		if q.Default != "" {
			fmt.Fprintf(p.out, "%s [%s]: ", q.Label, q.Default)
		} else {
			fmt.Fprintf(p.out, "%s: ", q.Label)
		}
		line, err := p.in.ReadString('\n')
		if err != nil && (err != io.EOF || line == "") {
			fmt.Fprintln(p.out)
			return "", app.ErrAborted
		}
		answer := strings.TrimSpace(line)
		switch answer {
		case "":
			answer = q.Default
		case "-":
			answer = ""
		}
		if err := q.Validate(answer); err != nil {
			fmt.Fprintf(p.out, "  %v\n", err)
			continue
		}
		return answer, nil
	}
}
