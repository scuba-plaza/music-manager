package device_test

import (
	"strings"
	"testing"

	"music-manager/internal/device"
)

func validProfile() device.Profile {
	return device.Profile{
		Name:          "my-player",
		Label:         "My Player",
		ComponentMax:  200,
		CoverFormat:   "keep",
		ReplayGain:    true,
		PlaylistExt:   ".m3u8",
		PlaylistStyle: "relative",
	}
}

func TestValidateAcceptsTheGenericProfile(t *testing.T) {
	if err := validProfile().Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name    string
		profile string
		key     string
		change  func(p *device.Profile)
	}{
		{"an empty name", "", "name", func(p *device.Profile) { p.Name = "" }},
		{"a name with a capital and a space", "My Player", "name", func(p *device.Profile) { p.Name = "My Player" }},
		{"a name that starts with a hyphen", "-player", "name", func(p *device.Profile) { p.Name = "-player" }},
		{"a name with an underscore", "my_player", "name", func(p *device.Profile) { p.Name = "my_player" }},
		{"a zero component_max", "my-player", "component_max", func(p *device.Profile) { p.ComponentMax = 0 }},
		{"a component_max below 20", "my-player", "component_max", func(p *device.Profile) { p.ComponentMax = 19 }},
		{"a stem_max equal to component_max", "my-player", "stem_max", func(p *device.Profile) { p.StemMax = p.ComponentMax }},
		{"a stem_max above component_max", "my-player", "stem_max", func(p *device.Profile) { p.StemMax = p.ComponentMax + 1 }},
		{"a negative stem_max", "my-player", "stem_max", func(p *device.Profile) { p.StemMax = -1 }},
		{"a negative tag_width", "my-player", "tag_width", func(p *device.Profile) { p.TagWidth = -1 }},
		{"a tag_width without latin_names", "my-player", "tag_width", func(p *device.Profile) { p.TagWidth = 12 }},
		{"a cover_format that is not known", "my-player", "cover_format", func(p *device.Profile) { p.CoverFormat = "png" }},
		{"an empty cover_format", "my-player", "cover_format", func(p *device.Profile) { p.CoverFormat = "" }},
		{"baseline-jpeg without cover_max_edge", "my-player", "cover_max_edge", func(p *device.Profile) {
			p.CoverFormat = "baseline-jpeg"
			p.CoverQuality = 85
		}},
		{"baseline-jpeg without cover_quality", "my-player", "cover_quality", func(p *device.Profile) {
			p.CoverFormat = "baseline-jpeg"
			p.CoverMaxEdge = 480
		}},
		{"baseline-jpeg with a cover_quality above 100", "my-player", "cover_quality", func(p *device.Profile) {
			p.CoverFormat = "baseline-jpeg"
			p.CoverMaxEdge = 480
			p.CoverQuality = 101
		}},
		{"a playlist_ext that is not known", "my-player", "playlist_ext", func(p *device.Profile) { p.PlaylistExt = ".pls" }},
		{"a playlist_style that is not known", "my-player", "playlist_style", func(p *device.Profile) { p.PlaylistStyle = "absolute" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := validProfile()
			c.change(&p)
			err := p.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want an error")
			}
			if want := `profile "` + c.profile + `"`; !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not name the profile (%s)", err, want)
			}
			if !strings.Contains(err.Error(), c.key) {
				t.Errorf("error %q does not name the key %s", err, c.key)
			}
		})
	}
}

func TestValidateAcceptsTheEdgesOfEveryRule(t *testing.T) {
	cases := []struct {
		name   string
		change func(p *device.Profile)
	}{
		{"the shortest name", func(p *device.Profile) { p.Name = "a" }},
		{"a name of digits and hyphens", func(p *device.Profile) { p.Name = "0-9-z" }},
		{"the smallest component_max", func(p *device.Profile) { p.ComponentMax = 20 }},
		{"a stem_max one below component_max", func(p *device.Profile) { p.StemMax = p.ComponentMax - 1 }},
		{"a tag_width with latin_names", func(p *device.Profile) {
			p.LatinNames = true
			p.TagWidth = 12
		}},
		{"the lowest cover_quality", func(p *device.Profile) {
			p.CoverFormat = "baseline-jpeg"
			p.CoverMaxEdge = 1
			p.CoverQuality = 1
		}},
		{"the highest cover_quality", func(p *device.Profile) {
			p.CoverFormat = "baseline-jpeg"
			p.CoverMaxEdge = 480
			p.CoverQuality = 100
		}},
		{"the m3u extension and the root-backslash style", func(p *device.Profile) {
			p.PlaylistExt = ".m3u"
			p.PlaylistStyle = "root-backslash"
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := validProfile()
			c.change(&p)
			if err := p.Validate(); err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestParseAppliesTheGenericDefaults(t *testing.T) {
	profiles, err := device.ParseProfiles([]byte("[[profile]]\nname = \"x\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := device.Profile{
		Name:          "x",
		Label:         "x",
		ComponentMax:  200,
		CoverFormat:   "keep",
		ReplayGain:    true,
		PlaylistExt:   ".m3u8",
		PlaylistStyle: "relative",
	}
	if len(profiles) != 1 || profiles[0] != want {
		t.Fatalf("ParseProfiles = %+v, want [%+v]", profiles, want)
	}
}

func TestParseKeepsAStatedLabel(t *testing.T) {
	profiles, err := device.ParseProfiles([]byte("[[profile]]\nname = \"x\"\nlabel = \"The X\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := profiles[0].Label; got != "The X" {
		t.Fatalf("Label = %q, want %q", got, "The X")
	}
}

func TestParseGivesEveryProfileTheDefaults(t *testing.T) {
	document := `
[[profile]]
name = "first"
component_max = 100
replaygain = false
playlist_ext = ".m3u"
playlist_style = "root-backslash"

[[profile]]
name = "second"
`
	profiles, err := device.ParseProfiles([]byte(document))
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 {
		t.Fatalf("got %d profiles, want 2", len(profiles))
	}
	first, second := profiles[0], profiles[1]
	if first.ComponentMax != 100 || first.ReplayGain || first.PlaylistExt != ".m3u" || first.PlaylistStyle != "root-backslash" {
		t.Errorf("first profile lost what it states: %+v", first)
	}
	if first.CoverFormat != "keep" || first.Label != "first" {
		t.Errorf("first profile lacks the defaults it does not state: %+v", first)
	}
	want := device.Profile{
		Name:          "second",
		Label:         "second",
		ComponentMax:  200,
		CoverFormat:   "keep",
		ReplayGain:    true,
		PlaylistExt:   ".m3u8",
		PlaylistStyle: "relative",
	}
	if second != want {
		t.Errorf("second profile = %+v, want %+v", second, want)
	}
}

func TestParseReadsTheWolfgangShape(t *testing.T) {
	document := `
[[profile]]
name = "wolfgang-wmp01"
label = "Wolfgang WMP01"
latin_names = true
component_max = 100
stem_max = 20
tag_width = 12
latin_genre = true
cover_format = "baseline-jpeg"
cover_max_edge = 480
cover_quality = 85
single_covers = true
replaygain = false
playlist_ext = ".m3u"
playlist_style = "root-backslash"
`
	profiles, err := device.ParseProfiles([]byte(document))
	if err != nil {
		t.Fatal(err)
	}
	want := device.Profile{
		Name:          "wolfgang-wmp01",
		Label:         "Wolfgang WMP01",
		LatinNames:    true,
		ComponentMax:  100,
		StemMax:       20,
		TagWidth:      12,
		LatinGenre:    true,
		CoverFormat:   "baseline-jpeg",
		CoverMaxEdge:  480,
		CoverQuality:  85,
		SingleCovers:  true,
		ReplayGain:    false,
		PlaylistExt:   ".m3u",
		PlaylistStyle: "root-backslash",
	}
	if len(profiles) != 1 || profiles[0] != want {
		t.Fatalf("ParseProfiles = %+v, want [%+v]", profiles, want)
	}
	if err := profiles[0].Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
}

func TestParseKeepsTheOrderOfTheFile(t *testing.T) {
	document := "[[profile]]\nname = \"b\"\n[[profile]]\nname = \"a\"\n[[profile]]\nname = \"c\"\n"
	profiles, err := device.ParseProfiles([]byte(document))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range profiles {
		names = append(names, p.Name)
	}
	if got := strings.Join(names, ","); got != "b,a,c" {
		t.Fatalf("names = %s, want b,a,c", got)
	}
}

func TestParseAnEmptyDocumentHasNoProfiles(t *testing.T) {
	profiles, err := device.ParseProfiles(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 0 {
		t.Fatalf("got %d profiles, want none", len(profiles))
	}
}

func TestParseRejectsAnUnknownKey(t *testing.T) {
	document := `
[[profile]]
name = "my-player"
latin_name = true
`
	_, err := device.ParseProfiles([]byte(document))
	if err == nil {
		t.Fatal("ParseProfiles = nil error, want one")
	}
	for _, want := range []string{"latin_name", `profile "my-player"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestParseNamesTheProfileWhereTheKeyIsUnknown(t *testing.T) {
	document := `
[[profile]]
name = "first"

[[profile]]
name = "second"
cover_size = 480
`
	_, err := device.ParseProfiles([]byte(document))
	if err == nil {
		t.Fatal("ParseProfiles = nil error, want one")
	}
	for _, want := range []string{"cover_size", `profile "second"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "first") {
		t.Errorf("error %q blames the first profile", err)
	}
}

func TestParseNamesAProfileWithoutANameByItsPosition(t *testing.T) {
	document := `
[[profile]]
name = "first"

[[profile]]
latin_name = true
`
	_, err := device.ParseProfiles([]byte(document))
	if err == nil {
		t.Fatal("ParseProfiles = nil error, want one")
	}
	for _, want := range []string{"latin_name", "profile #2"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestParseRejectsAnUnknownTable(t *testing.T) {
	_, err := device.ParseProfiles([]byte("[[profiles]]\nname = \"x\"\n"))
	if err == nil {
		t.Fatal("ParseProfiles = nil error, want one")
	}
	if !strings.Contains(err.Error(), "profiles") {
		t.Errorf("error %q does not name the table", err)
	}
}

func TestParseRejectsAValueOfTheWrongType(t *testing.T) {
	document := `
[[profile]]
name = "my-player"
component_max = "wide"
`
	_, err := device.ParseProfiles([]byte(document))
	if err == nil {
		t.Fatal("ParseProfiles = nil error, want one")
	}
	for _, want := range []string{"component_max", `profile "my-player"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestParseRejectsADocumentThatIsNotTOML(t *testing.T) {
	if _, err := device.ParseProfiles([]byte("[[profile]\nname = ")); err == nil {
		t.Fatal("ParseProfiles = nil error, want one")
	}
}

func TestParseValidatesEveryProfile(t *testing.T) {
	document := `
[[profile]]
name = "fine"

[[profile]]
name = "broken"
component_max = 5
`
	_, err := device.ParseProfiles([]byte(document))
	if err == nil {
		t.Fatal("ParseProfiles = nil error, want one")
	}
	for _, want := range []string{"component_max", `profile "broken"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestParseRejectsAKeyInTheWrongCase(t *testing.T) {
	cases := []struct{ name, line, key string }{
		{"a mixed-case key", "Latin_Names = true", "Latin_Names"},
		{"an upper-case key", "COMPONENT_MAX = 50", "COMPONENT_MAX"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			document := "[[profile]]\nname = \"my-player\"\n" + c.line + "\n"
			_, err := device.ParseProfiles([]byte(document))
			if err == nil {
				t.Fatal("ParseProfiles = nil error, want one")
			}
			for _, want := range []string{c.key + " unknown key", `profile "my-player"`} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}

func TestParseRejectsTheSameKeyInTwoCases(t *testing.T) {
	document := `
[[profile]]
name = "a"
Name = "b"
`
	_, err := device.ParseProfiles([]byte(document))
	if err == nil {
		t.Fatal("ParseProfiles = nil error, want one")
	}
	for _, want := range []string{"Name unknown key", `profile "a"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestParseRejectsAKeyGivenTwice(t *testing.T) {
	document := "[[profile]]\nname = \"a\"\nname = \"b\"\n"
	_, err := device.ParseProfiles([]byte(document))
	if err == nil {
		t.Fatal("ParseProfiles = nil error, want one")
	}
	for _, want := range []string{"name", "already", "line 3"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "wrong type") {
		t.Errorf("error %q calls a duplicate a type error", err)
	}
}

func TestParseRejectsTwoProfileTables(t *testing.T) {
	document := "[profile]\nname = \"a\"\n[profile]\nname = \"b\"\n"
	_, err := device.ParseProfiles([]byte(document))
	if err == nil {
		t.Fatal("ParseProfiles = nil error, want one")
	}
	for _, want := range []string{"profile", "already", "line 3"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "wrong type") {
		t.Errorf("error %q calls a duplicate a type error", err)
	}
}

func TestParseCallsAWrongTypeByThatName(t *testing.T) {
	document := "[[profile]]\nname = \"my-player\"\nreplaygain = \"yes\"\n"
	_, err := device.ParseProfiles([]byte(document))
	if err == nil {
		t.Fatal("ParseProfiles = nil error, want one")
	}
	for _, want := range []string{`profile "my-player"`, "replaygain", "wrong type"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}
