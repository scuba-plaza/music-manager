package device_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"music-manager/internal/device"
)

func writeDevices(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "devices.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func targetNames(targets []device.Target) []string {
	names := make([]string, len(targets))
	for i, target := range targets {
		names[i] = target.Name()
	}
	return names
}

func TestBuiltinListsGenericThenWolfgang(t *testing.T) {
	registry := device.Builtin()
	want := []device.ProfileInfo{
		{Name: "generic", Label: "Generic", Builtin: true},
		{Name: "wolfgang-wmp01", Label: "Wolfgang WMP01", Builtin: true},
	}
	if got := registry.Profiles(); !slices.Equal(got, want) {
		t.Errorf("Profiles() = %+v, want %+v", got, want)
	}
	if got, want := targetNames(registry.Targets()), []string{"generic", "wolfgang-wmp01"}; !slices.Equal(got, want) {
		t.Errorf("Targets() names = %v, want %v", got, want)
	}
	for _, name := range []string{"generic", "wolfgang-wmp01"} {
		target, ok := registry.Get(name)
		if !ok || target.Name() != name {
			t.Errorf("Get(%q) = %v, %v, want the target of that name", name, target, ok)
		}
	}
	if _, ok := registry.Get("my-player"); ok {
		t.Error(`Get("my-player") found a profile the built-ins do not have`)
	}
}

func TestBuiltinGivesTheTargetsTheirProfiles(t *testing.T) {
	registry := device.Builtin()
	generic, _ := registry.Get("generic")
	wolfgang, _ := registry.Get("wolfgang-wmp01")
	if generic.Profile() != device.Generic.Profile() {
		t.Errorf("generic profile = %+v, want %+v", generic.Profile(), device.Generic.Profile())
	}
	if wolfgang.Profile() != device.Wolfgang.Profile() {
		t.Errorf("wolfgang profile = %+v, want %+v", wolfgang.Profile(), device.Wolfgang.Profile())
	}
}

func TestACallerCannotChangeWhatTheRegistryListsOrBuilds(t *testing.T) {
	registry := device.Builtin()
	registry.Profiles()[0].Name = "changed"
	registry.Targets()[0] = device.Wolfgang
	if got := registry.Profiles()[0].Name; got != "generic" {
		t.Errorf("Profiles()[0].Name = %q after a caller changed its copy, want generic", got)
	}
	if got := registry.Targets()[0].Name(); got != "generic" {
		t.Errorf("Targets()[0].Name() = %q after a caller changed its copy, want generic", got)
	}
	if got := device.Builtin().Profiles()[0].Name; got != "generic" {
		t.Errorf("a new Builtin() lists %q first, want generic", got)
	}
}

func TestAUserProfileComesAfterTheBuiltIns(t *testing.T) {
	registry, err := device.LoadRegistry("testdata/devices_ok.toml")
	if err != nil {
		t.Fatalf("LoadRegistry = %v", err)
	}
	want := []device.ProfileInfo{
		{Name: "generic", Label: "Generic", Builtin: true},
		{Name: "wolfgang-wmp01", Label: "Wolfgang WMP01", Builtin: true},
		{Name: "my-player", Label: "My Player", Builtin: false},
		{Name: "plain-player", Label: "plain-player", Builtin: false},
	}
	if got := registry.Profiles(); !slices.Equal(got, want) {
		t.Errorf("Profiles() = %+v, want %+v", got, want)
	}
	if got, want := targetNames(registry.Targets()), []string{"generic", "wolfgang-wmp01", "my-player", "plain-player"}; !slices.Equal(got, want) {
		t.Errorf("Targets() names = %v, want %v", got, want)
	}
	target, ok := registry.Get("my-player")
	if !ok {
		t.Fatal(`Get("my-player") found nothing`)
	}
	profile := target.Profile()
	if !profile.LatinNames || profile.TagWidth != 12 {
		t.Errorf("my-player = %+v, want latin_names true and tag_width 12", profile)
	}
	if !target.Layout().Latin {
		t.Error("the my-player target does not lay out Latin names")
	}
}

func TestAFileWithoutProfilesListsOnlyTheBuiltIns(t *testing.T) {
	registry, err := device.LoadRegistry(writeDevices(t, ""))
	if err != nil {
		t.Fatalf("LoadRegistry = %v", err)
	}
	if got, want := registry.Profiles(), device.Builtin().Profiles(); !slices.Equal(got, want) {
		t.Errorf("Profiles() = %+v, want %+v", got, want)
	}
}

func TestDuplicateNamesAreRejected(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		profile string
	}{
		{"a user profile named like a built-in", writeDevices(t, "[[profile]]\nname = \"wolfgang-wmp01\"\n"), "wolfgang-wmp01"},
		{"two user profiles of one name", "testdata/devices_duplicate.toml", "my-player"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			registry, err := device.LoadRegistry(c.path)
			if err == nil {
				t.Fatal("LoadRegistry = nil error, want one")
			}
			if registry != nil {
				t.Errorf("LoadRegistry returned a registry with the error %q", err)
			}
			for _, want := range []string{c.path, `profile "` + c.profile + `"`, "name", "already"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}

func TestGenericIsReserved(t *testing.T) {
	path := writeDevices(t, "[[profile]]\nname = \"generic\"\nlabel = \"Mine\"\n")
	registry, err := device.LoadRegistry(path)
	if err == nil {
		t.Fatal("LoadRegistry = nil error, want one")
	}
	if registry != nil {
		t.Errorf("LoadRegistry returned a registry with the error %q", err)
	}
	for _, want := range []string{path, `profile "generic"`, "name", "reserved"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestTheErrorNamesTheFileTheProfileAndTheKey(t *testing.T) {
	path := "testdata/devices_typo.toml"
	registry, err := device.LoadRegistry(path)
	if err == nil {
		t.Fatal("LoadRegistry = nil error, want one")
	}
	if registry != nil {
		t.Errorf("LoadRegistry returned a registry with the error %q", err)
	}
	for _, want := range []string{path, `profile "my-player"`, "latin_name"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestEveryKindOfBadFileIsRefusedAndNamesWhatIsWrong(t *testing.T) {
	cases := []struct {
		name     string
		document string
		want     []string
	}{
		{"a key in the wrong case", "[[profile]]\nname = \"my-player\"\nLatin_Names = true\n", []string{`profile "my-player"`, "Latin_Names unknown key"}},
		{"a zero component_max", "[[profile]]\nname = \"my-player\"\ncomponent_max = 0\n", []string{`profile "my-player"`, "component_max"}},
		{"a negative tag_width", "[[profile]]\nname = \"my-player\"\nlatin_names = true\ntag_width = -1\n", []string{`profile "my-player"`, "tag_width"}},
		{"an unknown enum", "[[profile]]\nname = \"my-player\"\ncover_format = \"png\"\n", []string{`profile "my-player"`, "cover_format"}},
		{"a value of the wrong type", "[[profile]]\nname = \"my-player\"\ntag_width = \"wide\"\n", []string{`profile "my-player"`, "tag_width", "wrong type"}},
		{"a key given twice", "[[profile]]\nname = \"my-player\"\nlabel = \"a\"\nlabel = \"b\"\n", []string{"line 4", "label", "already"}},
		{"a profile without a name", "[[profile]]\nlabel = \"Nameless\"\n", []string{"profile \"\"", "name"}},
		{"text that is not TOML", "[[profile]\nname = ", []string{"line 1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := writeDevices(t, c.document)
			registry, err := device.LoadRegistry(path)
			if err == nil {
				t.Fatal("LoadRegistry = nil error, want one")
			}
			if registry != nil {
				t.Errorf("LoadRegistry returned a registry with the error %q", err)
			}
			for _, want := range append([]string{path}, c.want...) {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}

func TestMissingFileWrapsErrNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mm", "devices.toml")
	registry, err := device.LoadRegistry(path)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("LoadRegistry error = %v, want one that wraps fs.ErrNotExist", err)
	}
	if registry != nil {
		t.Errorf("LoadRegistry returned a registry with the error %q", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the file %s", err, path)
	}
}

func TestAnInvalidFileDoesNotWrapErrNotExist(t *testing.T) {
	_, err := device.LoadRegistry("testdata/devices_typo.toml")
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("LoadRegistry error = %v, want an error that is not fs.ErrNotExist", err)
	}
}

func TestConfigPathPrefersMMConfigThenXDGThenHome(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"MM_CONFIG wins over everything", map[string]string{"MM_CONFIG": "/etc/mm/mine.toml", "XDG_CONFIG_HOME": "/xdg"}, "/etc/mm/mine.toml"},
		{"MM_CONFIG alone", map[string]string{"MM_CONFIG": "/etc/mm/mine.toml"}, "/etc/mm/mine.toml"},
		{"XDG_CONFIG_HOME without MM_CONFIG", map[string]string{"XDG_CONFIG_HOME": "/xdg"}, "/xdg/mm/devices.toml"},
		{"an empty MM_CONFIG is not set", map[string]string{"MM_CONFIG": "", "XDG_CONFIG_HOME": "/xdg"}, "/xdg/mm/devices.toml"},
		{"neither variable", map[string]string{}, "/home/me/.config/mm/devices.toml"},
		{"an empty XDG_CONFIG_HOME is not set", map[string]string{"XDG_CONFIG_HOME": ""}, "/home/me/.config/mm/devices.toml"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			getenv := func(key string) string { return c.env[key] }
			if got := device.ConfigPath(getenv, "/home/me"); got != c.want {
				t.Errorf("ConfigPath = %q, want %q", got, c.want)
			}
		})
	}
}
