package browse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"music-manager/internal/device"
	"music-manager/internal/manifest"
	"music-manager/internal/render"
)

func TestOpCounts(t *testing.T) {
	ops := []render.Op{
		{Kind: render.Create, Path: "a"}, {Kind: render.Create, Path: "b"},
		{Kind: render.Update, Path: "c"}, {Kind: render.Stray, Path: "d"}, {Kind: render.Remove, Path: "e"}, {Kind: render.Remove, Path: "f"},
	}
	c := CountOps(ops)
	if c.Create != 2 || c.Update != 1 || c.Stray != 1 || c.Remove != 2 {
		t.Errorf("= %+v", c)
	}
	for _, want := range []string{"2 to create", "1 to update", "1 stray", "2 to delete"} {
		if !strings.Contains(c.Text(), want) {
			t.Errorf("%q lacks %q", c.Text(), want)
		}
	}
	if got := (OpCounts{}).Text(); got != "Nothing to do" {
		t.Errorf("= %q", got)
	}
	if c.Writes() != 3 {
		t.Errorf("writes = %d", c.Writes())
	}
}

func TestASyncIsSummedUpInWords(t *testing.T) {
	if got := Summary(OpCounts{Create: 2, Update: 1, Remove: 3}, 4); got != "2 created · 1 updated · 3 deleted · 4 unused files cleared away" {
		t.Errorf("= %q", got)
	}
	if got := Summary(OpCounts{}, 0); got != "Nothing needed doing." {
		t.Errorf("= %q", got)
	}
}

func TestMountCandidatesAreTheFoldersUnderTheMountRoots(t *testing.T) {
	root := t.TempDir()
	userRoot := filepath.Join(root, "run", "media", "tau")
	plain := filepath.Join(root, "media")
	for _, d := range []string{filepath.Join(userRoot, "PLAYER"), filepath.Join(userRoot, "STICK"), filepath.Join(plain, "OTHER"), filepath.Join(plain, "tau")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(userRoot, "a file"), nil, 0o644)

	got := MountCandidates([]string{userRoot, plain, filepath.Join(root, "missing")}, "tau")
	want := []string{filepath.Join(userRoot, "PLAYER"), filepath.Join(userRoot, "STICK"), filepath.Join(plain, "OTHER")}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("= %v, want %v (files, missing folders and the user's own folder are not drives)", got, want)
	}
}

func TestTheLibraryFolderComesFirstAmongTheTargets(t *testing.T) {
	t.Setenv("HOME", "/home/me")
	got := Targets("/home/me/Music/mp3-player", []string{"/run/media/tau/PLAYER"})
	if len(got) != 2 || got[0] != (TargetView{Label: "Library folder", Path: "~/Music/mp3-player", Kind: "library"}) ||
		got[1] != (TargetView{Label: "PLAYER", Path: "/run/media/tau/PLAYER", Kind: "drive"}) {
		t.Errorf("= %+v", got)
	}
}

func TestATypedFolderMayBeQuotedEscapedOrUnderHome(t *testing.T) {
	t.Setenv("HOME", "/home/me")
	for typed, want := range map[string]string{
		"'/mnt/my player'":        "/mnt/my player",
		`"/mnt/my player"`:        "/mnt/my player",
		`/mnt/my\ player`:         "/mnt/my player",
		"file:///mnt/my%20player": "/mnt/my player",
		"~/Music/player":          "/home/me/Music/player",
		"~":                       "/home/me",
		"   /mnt/player  ":        "/mnt/player",
		"":                        "",
		"relative/folder":         "relative/folder",
		"/mnt/a'b":                "/mnt/a'b",
		"~someone/else":           "~someone/else",
	} {
		if got := CleanPath(typed); got != want {
			t.Errorf("CleanPath(%q) = %q, want %q", typed, got, want)
		}
	}
	for p, want := range map[string]string{"/home/me": "~", "/home/me/Music": "~/Music", "/home/meow": "/home/meow", "/mnt/x": "/mnt/x"} {
		if got := Tilde(p); got != want {
			t.Errorf("Tilde(%q) = %q, want %q", p, got, want)
		}
	}
}

func TestAPlanNamesTheTracksItWarnsAbout(t *testing.T) {
	snap := Snapshot{Tracks: []manifest.Track{{ID: "t1", Title: "Long Name"}, {ID: "t2"}}}
	ws := []device.Warning{{TrackID: "t1", Message: "too long"}, {TrackID: "t2", Message: "no title"}, {TrackID: "gone", Message: "?"}}
	v := Plan(snap, "p1", "~/player", true, []render.Op{{Kind: render.Create, Path: "A/b.mp3"}, {Kind: render.Remove, Path: "old.mp3"}}, ws)
	if v.Plan != "p1" || v.Where != "~/player" || !v.Prune || v.Text != "1 to create · 1 to delete" {
		t.Errorf("= %+v", v)
	}
	if len(v.Ops) != 2 || v.Ops[0] != (OpView{"create", "A/b.mp3"}) || v.Ops[1] != (OpView{"remove", "old.mp3"}) {
		t.Errorf("ops = %+v", v.Ops)
	}
	if len(v.Warnings) != 3 || v.Warnings[0].Track != "Long Name" || v.Warnings[1].Track != "t2" || v.Warnings[2].Track != "gone" {
		t.Errorf("warnings = %+v", v.Warnings)
	}
}
