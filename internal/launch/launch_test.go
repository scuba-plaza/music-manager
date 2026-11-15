package launch_test

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
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

func run(t *testing.T, p *probe, getenv func(string) string, args ...string) (code int, out, errs string) {
	return runAt(t.TempDir(), p, getenv, args...)
}

func runAt(home string, p *probe, getenv func(string) string, args ...string) (code int, out, errs string) {
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
	dir := t.TempDir()
	code, _, errs := runAt(dir, p, noEnv)
	if code != 0 || p.calls != 1 {
		t.Fatalf("code %d, calls %d, err %q", code, p.calls, errs)
	}
	if p.options.Library != filepath.Join(dir, "Music", "Library") || p.options.Addr != "127.0.0.1:7878" || p.options.AddrSet || p.options.NoOpen {
		t.Errorf("options %+v", p.options)
	}
}

func TestAddrAndNoOpenFlagsArePassedOn(t *testing.T) {
	p := &probe{}
	code, _, errs := run(t, p, noEnv, "--addr", "127.0.0.1:9000", "--no-open")
	if code != 0 || p.calls != 1 {
		t.Fatalf("code %d, calls %d, err %q", code, p.calls, errs)
	}
	if p.options.Addr != "127.0.0.1:9000" || !p.options.AddrSet || !p.options.NoOpen {
		t.Errorf("options %+v", p.options)
	}
}

func TestLibraryFlagBeatsTheEnvironment(t *testing.T) {
	p := &probe{}
	if code, _, errs := run(t, p, libraryEnv("/from-env"), "--library", "/from-flag"); code != 0 {
		t.Fatalf("code %d, err %q", code, errs)
	}
	if p.options.Library != "/from-flag" {
		t.Errorf("library %q", p.options.Library)
	}
	if code, _, errs := run(t, p, libraryEnv("/from-env")); code != 0 {
		t.Fatalf("code %d, err %q", code, errs)
	}
	if p.options.Library != "/from-env" {
		t.Errorf("library without the flag %q", p.options.Library)
	}
}

func TestVersionFlagPrintsTheVersion(t *testing.T) {
	p := &probe{}
	code, out, errs := run(t, p, noEnv, "--version")
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
		code, _, errs := run(t, p, noEnv, "--"+flagName)
		if code != 2 || p.calls != 0 || !strings.Contains(errs, flagName) {
			t.Errorf("--%s: code %d, calls %d, err %q", flagName, code, p.calls, errs)
		}
	}
}

func TestAnArgumentThatIsNoFlagFails(t *testing.T) {
	p := &probe{}
	code, _, errs := run(t, p, noEnv, "extra")
	if code != 2 || p.calls != 0 || !strings.Contains(errs, `unexpected argument "extra"`) {
		t.Errorf("code %d, calls %d, err %q", code, p.calls, errs)
	}
}

func TestHelpListsTheFlags(t *testing.T) {
	p := &probe{}
	code, _, errs := run(t, p, noEnv, "--help")
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
	code, _, errs := run(t, p, noEnv)
	if code != 1 || errs != "mm: address in use\n" {
		t.Errorf("code %d, err %q", code, errs)
	}
}

func TestTheOldFlagsAndVariableForTheDeviceFileAreGone(t *testing.T) {
	for _, flag := range []string{"--config", "--check-config"} {
		p := &probe{}
		if code, _, errs := run(t, p, noEnv, flag); code != 2 || p.calls != 0 || !strings.Contains(errs, "not defined") {
			t.Errorf("%s: code %d, calls %d, %q", flag, code, p.calls, errs)
		}
	}
	p := &probe{}
	if code, _, errs := run(t, p, configEnvOf("/does/not/exist/devices.toml")); code != 0 || errs != "" || p.calls != 1 {
		t.Errorf("a MM_CONFIG that points nowhere is not read any more: code %d, calls %d, %q", code, p.calls, errs)
	}
}

func configEnvOf(path string) func(string) string {
	return func(k string) string {
		if k == "MM_CONFIG" {
			return path
		}
		return ""
	}
}

func TestHelpDoesNotPrintTheRealHomeDirectory(t *testing.T) {
	_, _, errs := runAt("/home/someone", &probe{}, noEnv, "--help")
	if strings.Contains(errs, "/home/someone") {
		t.Errorf("usage leaks the home directory: %q", errs)
	}
	if !strings.Contains(errs, "~/Music/Library") {
		t.Errorf("usage does not show the default library: %q", errs)
	}
}

func TestTheServerWritesItsMessagesWhereTheEnvironmentSaysAndKnowsTheLibrary(t *testing.T) {
	p := &probe{}
	var o bytes.Buffer
	code := launch.Run(context.Background(), launch.Env{
		Args: []string{"--library", "/music"}, Getenv: noEnv, Home: home, Out: &o, Err: &bytes.Buffer{}, Serve: p.serve,
	})
	if code != 0 || p.options.Out != &o {
		t.Errorf("code %d, the server's output is %v, want the environment's", code, p.options.Out)
	}
	if p.options.Library != "/music" {
		t.Errorf("library %q", p.options.Library)
	}
}
