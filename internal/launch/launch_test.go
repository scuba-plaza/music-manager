package launch_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"music-manager/internal/launch"
	"music-manager/internal/version"
)

const home = "/home/me"

func noEnv(string) string { return "" }

func libraryEnv(path string) func(string) string {
	return func(key string) string {
		if key == "MM_LIBRARY" {
			return path
		}
		return ""
	}
}

type probe struct {
	calls   int
	options launch.Options
	err     error
}

func (p *probe) serve(ctx context.Context, o launch.Options) error {
	p.calls++
	p.options = o
	return p.err
}

func run(p *probe, getenv func(string) string, args ...string) (code int, out, errs string) {
	var o, e bytes.Buffer
	code = launch.Run(context.Background(), launch.Env{
		Args:   args,
		Getenv: getenv,
		Home:   home,
		Out:    &o,
		Err:    &e,
		Serve:  p.serve,
	})
	return code, o.String(), e.String()
}

func TestTheDefaultLibraryIsMusicLibrary(t *testing.T) {
	if got := launch.DefaultLibrary(noEnv, home); got != "/home/me/Music/Library" {
		t.Errorf("default library %q", got)
	}
	if got := launch.DefaultLibrary(libraryEnv("/x"), home); got != "/x" {
		t.Errorf("library with MM_LIBRARY set %q", got)
	}
}

func TestNoArgumentsStartsTheServerOnTheDefaultAddress(t *testing.T) {
	p := &probe{}
	code, _, errs := run(p, noEnv)
	if code != 0 || p.calls != 1 {
		t.Fatalf("code %d, calls %d, err %q", code, p.calls, errs)
	}
	want := launch.Options{Library: "/home/me/Music/Library", Addr: "127.0.0.1:7878"}
	if p.options != want {
		t.Errorf("options %+v, want %+v", p.options, want)
	}
}

func TestAddrAndNoOpenFlagsArePassedOn(t *testing.T) {
	p := &probe{}
	code, _, errs := run(p, noEnv, "--addr", "127.0.0.1:9000", "--no-open")
	if code != 0 || p.calls != 1 {
		t.Fatalf("code %d, calls %d, err %q", code, p.calls, errs)
	}
	if p.options.Addr != "127.0.0.1:9000" || !p.options.AddrSet || !p.options.NoOpen {
		t.Errorf("options %+v", p.options)
	}
}

func TestLibraryFlagBeatsTheEnvironment(t *testing.T) {
	p := &probe{}
	if code, _, errs := run(p, libraryEnv("/from-env"), "--library", "/from-flag"); code != 0 {
		t.Fatalf("code %d, err %q", code, errs)
	}
	if p.options.Library != "/from-flag" {
		t.Errorf("library %q", p.options.Library)
	}
	if code, _, errs := run(p, libraryEnv("/from-env")); code != 0 {
		t.Fatalf("code %d, err %q", code, errs)
	}
	if p.options.Library != "/from-env" {
		t.Errorf("library without the flag %q", p.options.Library)
	}
}

func TestVersionFlagPrintsTheVersion(t *testing.T) {
	p := &probe{}
	code, out, errs := run(p, noEnv, "--version")
	if code != 0 || p.calls != 0 || errs != "" {
		t.Fatalf("code %d, calls %d, err %q", code, p.calls, errs)
	}
	if want := "mm " + version.String() + "\n"; out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}

func TestUnknownFlagFails(t *testing.T) {
	for _, flagName := range []string{"wolfgang", "bogus"} {
		p := &probe{}
		code, _, errs := run(p, noEnv, "--"+flagName)
		if code != 2 || p.calls != 0 || !strings.Contains(errs, flagName) {
			t.Errorf("--%s: code %d, calls %d, err %q", flagName, code, p.calls, errs)
		}
	}
}

func TestAnArgumentThatIsNoFlagFails(t *testing.T) {
	p := &probe{}
	code, _, errs := run(p, noEnv, "web")
	if code != 2 || p.calls != 0 || !strings.Contains(errs, "web") {
		t.Errorf("code %d, calls %d, err %q", code, p.calls, errs)
	}
}

func TestHelpListsTheFlags(t *testing.T) {
	p := &probe{}
	code, _, errs := run(p, noEnv, "--help")
	if code != 0 || p.calls != 0 {
		t.Fatalf("code %d, calls %d", code, p.calls)
	}
	for _, name := range []string{"library", "addr", "no-open", "version"} {
		if !strings.Contains(errs, name) {
			t.Errorf("usage does not mention %s: %q", name, errs)
		}
	}
}

func TestAServerErrorIsReported(t *testing.T) {
	p := &probe{err: errors.New("address in use")}
	code, _, errs := run(p, noEnv)
	if code != 1 || errs != "mm: address in use\n" {
		t.Errorf("code %d, err %q", code, errs)
	}
}
