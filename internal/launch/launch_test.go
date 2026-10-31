package launch_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
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
	if p.options.Library != "/home/me/Music/Library" || p.options.Addr != "127.0.0.1:7878" || p.options.AddrSet || p.options.NoOpen {
		t.Errorf("options %+v", p.options)
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
	code, _, errs := run(p, noEnv, "extra")
	if code != 2 || p.calls != 0 || !strings.Contains(errs, `unexpected argument "extra"`) {
		t.Errorf("code %d, calls %d, err %q", code, p.calls, errs)
	}
}

func TestHelpListsTheFlags(t *testing.T) {
	p := &probe{}
	code, _, errs := run(p, noEnv, "--help")
	if code != 0 || p.calls != 0 {
		t.Fatalf("code %d, calls %d", code, p.calls)
	}
	for _, name := range []string{"library", "addr", "no-open", "config", "check-config", "version"} {
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

const userProfile = `[[profile]]
name = "my-player"
latin_names = true
`

const typoProfile = `[[profile]]
name = "my-player"
latin_nmes = true
`

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "devices.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func configEnv(key, value string) func(string) string {
	return func(k string) string {
		if k == key {
			return value
		}
		return ""
	}
}

func profileNames(o launch.Options) []string {
	if o.Profiles == nil {
		return nil
	}
	var names []string
	for _, p := range o.Profiles.Profiles() {
		names = append(names, p.Name)
	}
	return names
}

func TestTheConfigFileGivesTheServerItsProfiles(t *testing.T) {
	p := &probe{}
	path := writeConfig(t, userProfile)
	code, _, errs := run(p, noEnv, "--config", path)
	if code != 0 || p.calls != 1 {
		t.Fatalf("code %d, calls %d, err %q", code, p.calls, errs)
	}
	if got := profileNames(p.options); !slices.Equal(got, []string{"generic", "wolfgang-wmp01", "my-player"}) {
		t.Errorf("profiles %v", got)
	}
	if p.options.Config != path {
		t.Errorf("config %q, want %q", p.options.Config, path)
	}
}

func TestAnInvalidConfigRefusesToStart(t *testing.T) {
	path := writeConfig(t, typoProfile)
	for name, args := range map[string][]string{"--config": {"--config", path}, "MM_CONFIG": nil} {
		p := &probe{}
		code, _, errs := run(p, configEnv("MM_CONFIG", path), args...)
		if code != 1 || p.calls != 0 || !strings.Contains(errs, path) || !strings.Contains(errs, "latin_nmes") {
			t.Errorf("%s: code %d, calls %d, err %q", name, code, p.calls, errs)
		}
	}
}

func TestCheckConfigPrintsProfilesAndExits(t *testing.T) {
	p := &probe{}
	code, out, errs := run(p, noEnv, "--config", writeConfig(t, userProfile), "--check-config")
	if code != 0 || p.calls != 0 || errs != "" {
		t.Fatalf("code %d, calls %d, err %q", code, p.calls, errs)
	}
	if out != "generic\nwolfgang-wmp01\nmy-player\n" {
		t.Errorf("output %q", out)
	}
	path := writeConfig(t, typoProfile)
	code, out, errs = run(p, noEnv, "--config", path, "--check-config")
	if code != 1 || p.calls != 0 || out != "" || !strings.Contains(errs, path) {
		t.Errorf("invalid: code %d, calls %d, out %q, err %q", code, p.calls, out, errs)
	}
}

func TestAMissingDefaultConfigIsFine(t *testing.T) {
	dir := t.TempDir()
	for name, getenv := range map[string]func(string) string{
		"home":            noEnv,
		"MM_CONFIG":       configEnv("MM_CONFIG", filepath.Join(dir, "absent.toml")),
		"XDG_CONFIG_HOME": configEnv("XDG_CONFIG_HOME", dir),
	} {
		p := &probe{}
		code, _, errs := run(p, getenv)
		if code != 0 || p.calls != 1 {
			t.Fatalf("%s: code %d, calls %d, err %q", name, code, p.calls, errs)
		}
		if got := profileNames(p.options); !slices.Equal(got, []string{"generic", "wolfgang-wmp01"}) {
			t.Errorf("%s: profiles %v", name, got)
		}
	}
	p := &probe{}
	code, out, errs := run(p, noEnv, "--check-config")
	if code != 0 || p.calls != 0 || out != "generic\nwolfgang-wmp01\n" {
		t.Errorf("check: code %d, calls %d, out %q, err %q", code, p.calls, out, errs)
	}
}

func TestAMissingExplicitConfigFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.toml")
	for _, args := range [][]string{{"--config", path}, {"--config", path, "--check-config"}} {
		p := &probe{}
		code, _, errs := run(p, noEnv, args...)
		if code != 1 || p.calls != 0 || !strings.Contains(errs, path) {
			t.Errorf("%v: code %d, calls %d, err %q", args, code, p.calls, errs)
		}
	}
}
