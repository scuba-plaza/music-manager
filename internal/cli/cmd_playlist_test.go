package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"music-manager/internal/testmp3"
)

// libWithSongs makes a library of identified tracks, one per title, all by
// "Band", and returns the library and the track IDs by title.
func libWithSongs(t *testing.T, titles ...string) (string, map[string]string) {
	t.Helper()
	lib, src := filepath.Join(t.TempDir(), "lib"), t.TempDir()
	for i := range titles {
		writeMP3(t, filepath.Join(src, fmt.Sprintf("f%d.mp3", i)), testmp3.Silent(50+10*i))
	}
	if code, _, errs := run(t, lib, "ingest", src); code != 0 {
		t.Fatalf("ingest: %s", errs)
	}
	_, out, _ := run(t, lib, "ls")
	rows := strings.Split(strings.TrimSpace(out), "\n")[1:]
	ids := map[string]string{}
	for i, row := range rows {
		id := strings.Fields(row)[0]
		if code, _, errs := run(t, lib, "edit", id, "--title", titles[i], "--artist", "Band"); code != 0 {
			t.Fatalf("edit: %s", errs)
		}
		ids[titles[i]] = id
	}
	return lib, ids
}

func TestCreatingListingAndShowingPlaylists(t *testing.T) {
	lib, id := libWithSongs(t, "Dream", "Atlek", "Gym")

	code, out, errs := run(t, lib, "playlist", "new", "Study", id["Dream"], id["Atlek"])
	if code != 0 || !strings.Contains(out, "Study") || !strings.Contains(out, "2 tracks") {
		t.Errorf("new: code %d, out %q, err %q", code, out, errs)
	}

	code, out, errs = run(t, lib, "playlist", "ls")
	if code != 0 || !strings.Contains(out, "Study") || !strings.Contains(out, "2") {
		t.Errorf("ls: code %d, out %q, err %q", code, out, errs)
	}

	code, out, errs = run(t, lib, "playlist", "show", "Study")
	if code != 0 {
		t.Fatalf("show: code %d, err %q", code, errs)
	}
	dream, atlek := strings.Index(out, "Dream"), strings.Index(out, "Atlek")
	if dream < 0 || atlek < dream || strings.Contains(out, "Gym") {
		t.Errorf("show should list Dream then Atlek, and not Gym:\n%s", out)
	}
	for _, want := range []string{id["Dream"], id["Atlek"], "Band"} {
		if !strings.Contains(out, want) {
			t.Errorf("show lacks %q:\n%s", want, out)
		}
	}

	if code, _, errs := run(t, lib, "playlist", "show", "Nope"); code == 0 || !strings.Contains(errs, "no playlist named") {
		t.Errorf("show of an unknown playlist: code %d, err %q", code, errs)
	}
	if code, _, errs := run(t, lib, "playlist", "new", "Gym", "zzzzzz"); code == 0 || !strings.Contains(errs, "zzzzzz") {
		t.Errorf("new with an unknown track: code %d, err %q", code, errs)
	}
}

// order is the titles of a playlist as `mm playlist show` lists them.
func order(t *testing.T, lib, name string) []string {
	t.Helper()
	code, out, errs := run(t, lib, "playlist", "show", name)
	if code != 0 {
		t.Fatalf("show %s: %s", name, errs)
	}
	var titles []string
	for _, line := range strings.Split(out, "\n")[1:] {
		if f := strings.Fields(line); len(f) >= 4 && f[0] != "" && strings.Trim(f[0], "0123456789") == "" {
			titles = append(titles, f[3]) // # ID ARTIST TITLE
		}
	}
	return titles
}

func TestAddingRemovingAndMovingTracksOfAPlaylist(t *testing.T) {
	lib, id := libWithSongs(t, "Dream", "Atlek", "Gym")
	run(t, lib, "playlist", "new", "Study", id["Dream"])

	code, out, errs := run(t, lib, "playlist", "add", "Study", id["Gym"], id["Atlek"], id["Dream"])
	if code != 0 || !strings.Contains(out, "added 2") {
		t.Errorf("add: code %d, out %q, err %q", code, out, errs)
	}
	if got := strings.Join(order(t, lib, "Study"), ","); got != "Dream,Gym,Atlek" {
		t.Errorf("order = %s", got)
	}

	// Positions are counted from 1.
	if code, _, errs := run(t, lib, "playlist", "move", "Study", id["Atlek"], "1"); code != 0 {
		t.Fatalf("move: %s", errs)
	}
	if got := strings.Join(order(t, lib, "Study"), ","); got != "Atlek,Dream,Gym" {
		t.Errorf("after move = %s", got)
	}
	for _, pos := range []string{"0", "4", "x"} {
		if code, _, _ := run(t, lib, "playlist", "move", "Study", id["Atlek"], pos); code == 0 {
			t.Errorf("move to position %q succeeded", pos)
		}
	}

	code, out, errs = run(t, lib, "playlist", "rm", "Study", id["Dream"])
	if code != 0 || !strings.Contains(out, "removed 1") {
		t.Errorf("rm: code %d, out %q, err %q", code, out, errs)
	}
	if got := strings.Join(order(t, lib, "Study"), ","); got != "Atlek,Gym" {
		t.Errorf("after rm = %s", got)
	}
	if code, _, errs := run(t, lib, "playlist", "rm", "Study", id["Dream"]); code == 0 || !strings.Contains(errs, "not in the playlist") {
		t.Errorf("rm of a track that is not in it: code %d, err %q", code, errs)
	}
}

func TestRenamingAndDeletingAPlaylist(t *testing.T) {
	lib, id := libWithSongs(t, "Dream")
	run(t, lib, "playlist", "new", "Study", id["Dream"])
	run(t, lib, "playlist", "new", "Gym")

	code, out, errs := run(t, lib, "playlist", "rename", "Study", "Exams")
	if code != 0 || !strings.Contains(out, "Exams") {
		t.Errorf("rename: code %d, out %q, err %q", code, out, errs)
	}
	if got := strings.Join(order(t, lib, "Exams"), ","); got != "Dream" {
		t.Errorf("the renamed playlist holds %q", got)
	}
	if code, _, errs := run(t, lib, "playlist", "rename", "Exams", "gym"); code == 0 || !strings.Contains(errs, "already a playlist") {
		t.Errorf("rename onto another playlist: code %d, err %q", code, errs)
	}

	code, out, errs = run(t, lib, "playlist", "delete", "Exams")
	if code != 0 || !strings.Contains(out, "deleted Exams") {
		t.Errorf("delete: code %d, out %q, err %q", code, out, errs)
	}
	_, out, _ = run(t, lib, "playlist", "ls")
	if strings.Contains(out, "Exams") || !strings.Contains(out, "Gym") {
		t.Errorf("ls after delete:\n%s", out)
	}
	if code, _, _ := run(t, lib, "playlist", "delete", "Exams"); code == 0 {
		t.Error("deleting twice succeeded")
	}
	_, out, _ = run(t, lib, "ls")
	if !strings.Contains(out, "Dream") {
		t.Error("deleting a playlist removed its track")
	}
}

func TestSyncWritesThePlaylistsMadeOnTheCommandLine(t *testing.T) {
	lib, id := libWithSongs(t, "Dream", "Atlek")
	run(t, lib, "playlist", "new", "Study", id["Atlek"], id["Dream"])

	code, out, errs := run(t, lib, "sync")
	if code != 0 || !strings.Contains(out, "Playlists/Study.m3u8") {
		t.Fatalf("sync: code %d, out %q, err %q", code, out, errs)
	}
	got, err := os.ReadFile(filepath.Join(lib, "Playlists", "Study.m3u8"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "\n../Band/Atlek.mp3\n") || !strings.Contains(string(got), "\n../Band/Dream.mp3\n") || !strings.HasPrefix(string(got), "#EXTM3U\n") {
		t.Errorf("Study.m3u8 = %q, want an M3U8 with the two tracks, Atlek first", got)
	}
}

func TestImportingAPlaylistFileOfThePlayer(t *testing.T) {
	// Three entries: two whole names, and one the player cut at 26 characters.
	const file = "../app/testdata/userpl-sample.pl"
	lib, id := libWithSongs(t, "Alpha Song", "Beta Song", "Gamma Song With A Very Long Title", "Unrelated")

	code, out, errs := run(t, lib, "playlist", "import", file, "Old", "--dry-run")
	if code != 0 || !strings.Contains(out, "Alpha Song.mp3") || !strings.Contains(out, id["Gamma Song With A Very Long Title"]) {
		t.Fatalf("dry run: code %d, out %q, err %q", code, out, errs)
	}
	if _, out, _ := run(t, lib, "playlist", "ls"); strings.Contains(out, "Old") {
		t.Error("a dry run made the playlist")
	}

	code, out, errs = run(t, lib, "playlist", "import", file, "Old")
	if code != 0 || !strings.Contains(out, "created Old with 3 tracks") {
		t.Fatalf("import: code %d, out %q, err %q", code, out, errs)
	}
	if got := strings.Join(order(t, lib, "Old"), ","); got != "Alpha,Beta,Gamma" { // the third column of `show` is the first word of the title
		t.Errorf("Old = %s", got)
	}

	if code, _, errs := run(t, lib, "playlist", "import", file, "Old"); code == 0 || !strings.Contains(errs, "already a playlist") {
		t.Errorf("importing under a name that is taken: code %d, err %q", code, errs)
	}
	bad := filepath.Join(t.TempDir(), "x.pl")
	os.WriteFile(bad, []byte("nope"), 0o644)
	if code, _, errs := run(t, lib, "playlist", "import", bad, "X"); code == 0 || !strings.Contains(errs, "not a playlist file of the player") {
		t.Errorf("importing a file that is not a playlist: code %d, err %q", code, errs)
	}
}
