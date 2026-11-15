package manifest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"music-manager/internal/manifest"
)

const v2Library = `{
  "version": 2,
  "tracks": [
    {"id": "aaaaaa", "audio_sha256": "x", "status": "identified", "title": "So What", "artist": "Miles Davis", "mb_recording_id": "gone"}
  ]
}
`

func writeV2(t *testing.T) (*manifest.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "library.json")
	if err := os.WriteFile(path, []byte(v2Library), 0o644); err != nil {
		t.Fatal(err)
	}
	return manifest.NewStore(path), path
}

func TestAV2LibraryGetsTheWolfgangProfileItHadBuiltIn(t *testing.T) {
	s, _ := writeV2(t)

	m, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}

	want := manifest.Profile{
		ID: "wolfgang-wmp01", Name: "Wolfgang WMP01",
		LatinNames: true, ComponentMax: 100, StemMax: 20, TagWidth: 12, LatinGenre: true,
		CoverFormat: "baseline-jpeg", CoverMaxEdge: 480, CoverQuality: 85, SingleCovers: true,
		ReplayGain: false, PlaylistExt: ".m3u", PlaylistStyle: "root-backslash",
	}
	if len(m.Profiles) != 1 || m.Profiles[0] != want {
		t.Errorf("%+v, want [%+v]", m.Profiles, want)
	}
	if len(m.Tracks) != 1 || m.Tracks[0].Title != "So What" {
		t.Errorf("the tracks came through: %+v", m.Tracks)
	}
}

func TestReadingAV2LibraryChangesNothingOnDisk(t *testing.T) {
	s, path := writeV2(t)

	if _, err := s.Load(); err != nil {
		t.Fatal(err)
	}

	if data, _ := os.ReadFile(path); string(data) != v2Library {
		t.Errorf("the file changed: %s", data)
	}
	if _, err := os.Stat(path + ".v2.bak"); !os.IsNotExist(err) {
		t.Errorf("a backup was made by reading: %v", err)
	}
}

func TestTheFirstChangeToAV2LibraryWritesVersion3AndKeepsTheOldFileAside(t *testing.T) {
	s, path := writeV2(t)

	if err := s.Update(func(m *manifest.Manifest) error { return nil }); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"version": 3`) || !strings.Contains(string(data), `"wolfgang-wmp01"`) {
		t.Errorf("not saved as version 3 with the profile: %s", data)
	}
	if kept, _ := os.ReadFile(path + ".v2.bak"); string(kept) != v2Library {
		t.Errorf("the old file is not kept as it was: %s", kept)
	}
}

func TestTheOldFileIsKeptAsideOnlyOnce(t *testing.T) {
	s, path := writeV2(t)
	if err := s.Update(func(m *manifest.Manifest) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".v2.bak", []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := s.Update(func(m *manifest.Manifest) error { return nil }); err != nil {
		t.Fatal(err)
	}

	if kept, _ := os.ReadFile(path + ".v2.bak"); string(kept) != "mine" {
		t.Errorf("the second change touched it: %s", kept)
	}
}

func TestAProfileTakenAwayDoesNotComeBack(t *testing.T) {
	s, _ := writeV2(t)
	if err := s.Update(func(m *manifest.Manifest) error { m.Profiles = nil; return nil }); err != nil {
		t.Fatal(err)
	}

	m, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}

	if len(m.Profiles) != 0 {
		t.Errorf("%+v", m.Profiles)
	}
}

func TestANewLibraryHasNoProfiles(t *testing.T) {
	m, err := manifest.NewStore(filepath.Join(t.TempDir(), "library.json")).Load()
	if err != nil || len(m.Profiles) != 0 {
		t.Errorf("%+v %v", m, err)
	}
}

func TestAnOlderVersionThanTwoIsStillRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	os.WriteFile(path, []byte(`{"version": 1, "tracks": []}`), 0o644)

	if _, err := manifest.NewStore(path).Load(); err == nil || !strings.Contains(err.Error(), "unsupported library version 1") {
		t.Errorf("%v", err)
	}
}

func TestWhatIsWrongWithAProfileIsSaidBySetting(t *testing.T) {
	good := manifest.WolfgangWMP01
	cases := []struct {
		name   string
		change func(*manifest.Profile)
		field  string
	}{
		{"empty name", func(p *manifest.Profile) { p.Name = "  " }, "name"},
		{"long name", func(p *manifest.Profile) { p.Name = strings.Repeat("x", 41) }, "name"},
		{"generic name", func(p *manifest.Profile) { p.Name = " generic " }, "name"},
		{"short component", func(p *manifest.Profile) { p.ComponentMax = 19 }, "component_max"},
		{"long component", func(p *manifest.Profile) { p.ComponentMax = 256 }, "component_max"},
		{"stem not below component", func(p *manifest.Profile) { p.StemMax = 100 }, "stem_max"},
		{"negative stem", func(p *manifest.Profile) { p.StemMax = -1 }, "stem_max"},
		{"wide tag", func(p *manifest.Profile) { p.TagWidth = 256 }, "tag_width"},
		{"tag needs latin", func(p *manifest.Profile) { p.LatinNames = false }, "tag_width"},
		{"unknown cover format", func(p *manifest.Profile) { p.CoverFormat = "png" }, "cover_format"},
		{"zero edge", func(p *manifest.Profile) { p.CoverMaxEdge = 0 }, "cover_max_edge"},
		{"huge edge", func(p *manifest.Profile) { p.CoverMaxEdge = 4097 }, "cover_max_edge"},
		{"zero quality", func(p *manifest.Profile) { p.CoverQuality = 0 }, "cover_quality"},
		{"quality over 100", func(p *manifest.Profile) { p.CoverQuality = 101 }, "cover_quality"},
		{"extension", func(p *manifest.Profile) { p.PlaylistExt = ".pls" }, "playlist_ext"},
		{"style", func(p *manifest.Profile) { p.PlaylistStyle = "absolute" }, "playlist_style"},
	}
	if problems := good.Problems(); len(problems) != 0 {
		t.Fatalf("the Wolfgang profile has problems: %v", problems)
	}
	for _, c := range cases {
		p := good
		c.change(&p)

		problems := p.Problems()

		if problems[c.field] == "" || len(problems) != 1 {
			t.Errorf("%s: %v, want only %s", c.name, problems, c.field)
		}
	}
}

func TestTheCoverSizeAndQualityAreNotCheckedWhenTheCoverIsKept(t *testing.T) {
	p := manifest.WolfgangWMP01
	p.CoverFormat, p.CoverMaxEdge, p.CoverQuality = "keep", 0, 0

	if problems := p.Problems(); len(problems) != 0 {
		t.Errorf("%v", problems)
	}
}

func TestSeveralSettingsWrongAreAllSaid(t *testing.T) {
	p := manifest.WolfgangWMP01
	p.Name, p.ComponentMax, p.PlaylistExt = "", 5, "x"

	if problems := p.Problems(); len(problems) != 3 {
		t.Errorf("%v", problems)
	}
}
