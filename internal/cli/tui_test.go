package cli_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/cli"
)

type tuiProbe struct {
	calls    int
	root     string
	wolfgang bool
	err      error
}

func (p *tuiProbe) run(ctx context.Context, a *app.App, wolfgang bool) error {
	p.calls++
	p.root = a.Root()
	p.wolfgang = wolfgang
	return p.err
}

func TestPlainMMOpensTheInterfaceWhenThereIsATerminal(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	p := &tuiProbe{}
	code, out, errs := runWith(t, cli.Deps{RunTUI: p.run}, lib)
	if code != 0 || p.calls != 1 || p.root != lib {
		t.Errorf("code %d, calls %d, root %q, out %q, err %q", code, p.calls, p.root, out, errs)
	}
	if strings.Contains(out, "Available Commands") {
		t.Error("help was printed as well")
	}
}

func TestPlainMMPrintsHelpWithoutATerminal(t *testing.T) {
	code, out, _ := run(t, filepath.Join(t.TempDir(), "lib"))
	if code != 0 || !strings.Contains(out, "Available Commands") {
		t.Errorf("code %d, out %q", code, out)
	}
}

func TestHelpAndSubcommandsAreUnaffectedByTheInterface(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	p := &tuiProbe{}
	deps := cli.Deps{RunTUI: p.run}
	if code, out, _ := runWith(t, deps, lib, "--help"); code != 0 || !strings.Contains(out, "Available Commands") {
		t.Errorf("--help: code %d, out %q", code, out)
	}
	if code, out, _ := runWith(t, deps, lib, "ls"); code != 0 || !strings.Contains(out, "ID") {
		t.Errorf("ls: code %d, out %q", code, out)
	}
	if code, _, errs := runWith(t, deps, lib, "nonsense"); code == 0 || !strings.Contains(errs, "unknown command") {
		t.Errorf("an unknown command must still fail: code %d, err %q", code, errs)
	}
	if p.calls != 0 {
		t.Errorf("the interface was started %d times", p.calls)
	}
}

func TestMMTuiStartsTheInterfaceOrExplainsWhyNot(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	p := &tuiProbe{}
	if code, _, errs := runWith(t, cli.Deps{RunTUI: p.run}, lib, "tui"); code != 0 || p.calls != 1 {
		t.Errorf("code %d, calls %d, err %q", code, p.calls, errs)
	}
	code, _, errs := run(t, lib, "tui")
	if code == 0 || !strings.Contains(errs, "terminal") || !strings.HasPrefix(errs, "mm: ") {
		t.Errorf("without a terminal: code %d, err %q", code, errs)
	}
}

func TestAnErrorFromTheInterfaceIsReported(t *testing.T) {
	p := &tuiProbe{err: context.DeadlineExceeded}
	code, _, errs := runWith(t, cli.Deps{RunTUI: p.run}, filepath.Join(t.TempDir(), "lib"))
	if code == 0 || !strings.Contains(errs, "deadline") {
		t.Errorf("code %d, err %q", code, errs)
	}
}

func TestTheInterfaceShowsTheWolfgangPreviewOnlyWhenAsked(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	for _, c := range []struct {
		args []string
		want bool
	}{{nil, false}, {[]string{"--wolfgang"}, true}, {[]string{"tui"}, false}, {[]string{"tui", "--wolfgang"}, true}} {
		p := &tuiProbe{}
		if code, _, errs := runWith(t, cli.Deps{RunTUI: p.run}, lib, c.args...); code != 0 || p.calls != 1 || p.wolfgang != c.want {
			t.Errorf("mm %v: code %d, calls %d, wolfgang %v, err %q; want wolfgang %v", c.args, code, p.calls, p.wolfgang, errs, c.want)
		}
	}
}
