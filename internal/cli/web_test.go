package cli_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/cli"
)

type webProbe struct {
	calls int
	root  string
	opts  cli.WebOptions
	err   error
}

func (p *webProbe) run(ctx context.Context, a *app.App, o cli.WebOptions) error {
	p.calls++
	p.root = a.Root()
	p.opts = o
	return p.err
}

func TestWebServesTheLibraryOnTheDefaultAddress(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	p := &webProbe{}
	code, _, errs := runWith(t, cli.Deps{RunWeb: p.run}, lib, "web")
	if code != 0 || p.calls != 1 || p.root != lib {
		t.Fatalf("code %d, calls %d, root %q, err %q", code, p.calls, p.root, errs)
	}
	if p.opts.Addr != cli.DefaultWebAddr || p.opts.AddrSet || p.opts.NoOpen || p.opts.Wolfgang || p.opts.Out == nil {
		t.Errorf("options %+v", p.opts)
	}
}

func TestWebTakesItsAddressAndFlags(t *testing.T) {
	p := &webProbe{}
	code, _, errs := runWith(t, cli.Deps{RunWeb: p.run}, filepath.Join(t.TempDir(), "lib"), "web", "--addr", "localhost:9000", "--no-open", "--wolfgang")
	if code != 0 {
		t.Fatalf("code %d, err %q", code, errs)
	}
	if p.opts.Addr != "localhost:9000" || !p.opts.AddrSet || !p.opts.NoOpen || !p.opts.Wolfgang {
		t.Errorf("options %+v", p.opts)
	}
}

func TestWebFailsWithoutAServer(t *testing.T) {
	code, _, errs := run(t, filepath.Join(t.TempDir(), "lib"), "web")
	if code == 0 || !strings.Contains(errs, "not available") {
		t.Errorf("code %d, err %q", code, errs)
	}
}

func TestWebPassesOnWhyItStopped(t *testing.T) {
	p := &webProbe{err: context.Canceled}
	code, _, errs := runWith(t, cli.Deps{RunWeb: p.run}, filepath.Join(t.TempDir(), "lib"), "web")
	if code == 0 || !strings.Contains(errs, "canceled") {
		t.Errorf("code %d, err %q", code, errs)
	}
}
