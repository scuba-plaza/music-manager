package app_test

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/device"
	"music-manager/internal/mp3"
	"music-manager/internal/testmp3"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

// fixture builds a small library that has a bit of everything the renderer
// has an opinion about: names in Arabic, titles and albums too long for the player, two tracks that collide,
// singles with and without a cover, an unidentified track and two playlists.
// It returns the library and the IDs of the tracks by letter.
func fixture(t *testing.T) (*app.App, map[string]string) {
	t.Helper()
	a := libWithTracks(t, 100, 101, 102, 103, 104, 105, 106)
	letters := []string{"a", "b", "c", "d", "e", "f", "g"}
	ids := map[string]string{}
	for i, tr := range tracks(t, a) {
		ids[letters[i]] = tr.ID
	}
	set := func(letter, title, artist, album string, n, total int, e app.Edit) {
		t.Helper()
		e.Title, e.Artist, e.Album, e.TrackNumber, e.TrackTotal = &title, &artist, &album, &n, &total
		if _, err := a.Edit(ids[letter], e); err != nil {
			t.Fatal(err)
		}
	}
	set("a", "وطني", "فيروز", "Hits", 3, 12, app.Edit{Genre: ptr("Folk"), Released: ptr("1999")})
	set("b", "بحلم بيك", "عبد الحليم حافظ", "روائع", 1, 6, app.Edit{TitleLatin: ptr("Bahlam Bek"), ArtistLatin: ptr("Abdel Halim Hafez"), AlbumLatin: ptr("Rawa'e")}) // the others have no Latin form set by hand
	set("c", "A Very Long Title Indeed That Goes On", "Max Richter", "The Long Album Name Here (Deluxe)", 5, 5, app.Edit{})
	set("d", "A Very Long Title Indeed That Goes On", "Max Richter", "The Long Album Name Here (Deluxe)", 5, 5, app.Edit{})
	set("e", "Dream", "Keaton Henson", "", 0, 0, app.Edit{Genre: ptr("Folk")})
	set("f", "Cold", "Nils Frahm", "", 0, 0, app.Edit{})
	// g stays unidentified
	for letter, img := range map[string][]byte{"a": testmp3.PNG, "c": testmp3.JPEG, "e": testmp3.JPEG2, "f": testmp3.PNG} {
		if err := a.SetCover(ids[letter], img); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.CreatePlaylist("Study", ids["a"], ids["b"], ids["c"], ids["e"]); err != nil {
		t.Fatal(err)
	}
	if err := a.CreatePlaylist("Resistance", ids["b"], ids["f"]); err != nil {
		t.Fatal(err)
	}
	return a, ids
}

// describe lists everything under root that the player or a person would
// see, one entry per file, without anything that differs from run to run
// (the track IDs, which are random, are replaced by their letters).
func describe(t *testing.T, root string, ids map[string]string) string {
	t.Helper()
	letterOf := func(s string) string {
		for letter, id := range ids {
			s = strings.ReplaceAll(s, id, "<"+letter+">")
		}
		return s
	}
	var rels []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".mm" && filepath.Dir(p) == root {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rels = append(rels, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(rels)
	short := func(b []byte) string {
		sum := sha256.Sum256(b)
		return hex.EncodeToString(sum[:])[:10]
	}
	var out strings.Builder
	for _, rel := range rels {
		data := readFile(t, root, rel)
		fmt.Fprintf(&out, "%s\n", letterOf(rel))
		switch strings.ToLower(filepath.Ext(rel)) {
		case ".mp3":
			ts := mp3.ReadTagSet(data)
			audio, length, err := mp3.Parse(data)
			if err != nil {
				t.Fatalf("%s: %v", rel, err)
			}
			fmt.Fprintf(&out, "  title=%q artist=%q album=%q genre=%q track=%d/%d date=%q tlen=%v\n",
				ts.Title, ts.Artist, ts.Album, ts.Genre, ts.Track, ts.Total, ts.Date, ts.Length)
			var keys []string
			for k := range ts.Extra {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var extra []string
			for _, k := range keys {
				v := letterOf(ts.Extra[k])
				extra = append(extra, fmt.Sprintf("%s=%q", k, v))
			}
			cover := "none"
			if len(ts.Cover) > 0 {
				cover = fmt.Sprintf("%s %s", ts.CoverMIME, short(ts.Cover))
			}
			fmt.Fprintf(&out, "  extra: %s\n  cover: %s\n  audio: %s %v\n", strings.Join(extra, " "), cover, short(audio), length)
		case ".m3u", ".m3u8":
			fmt.Fprintf(&out, "  %q\n", letterOf(string(data)))
		default:
			fmt.Fprintf(&out, "  %d bytes %s\n", len(data), short(data))
		}
	}
	return out.String()
}

// TestSyncOutputGolden pins what a sync writes: paths, every tag, the covers,
// the playlists. It is the oracle that the changes of the device plan keep the
// player's output as it was except where a slice says otherwise; when a slice
// changes the output on purpose, the golden is regenerated with -update and the
// diff is read.
func TestSyncOutputGolden(t *testing.T) {
	for name, dev := range map[string]device.Target{"generic": device.Generic, "wolfgang": device.Wolfgang} {
		t.Run(name, func(t *testing.T) { checkGolden(t, dev, "sync-"+name+".golden") })
	}
}

func checkGolden(t *testing.T, dev device.Target, file string) {
	a, ids := fixture(t)
	target := filepath.Join(t.TempDir(), "player")
	mustSync(t, a, app.SyncOptions{Target: target, Device: dev})
	got := describe(t, target, ids)

	golden := filepath.Join("testdata", file)
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if got != string(want) {
		t.Errorf("the rendered tree differs from %s (run with -update to accept, then read the diff):\n%s", golden, lineDiff(string(want), got))
	}
}

// lineDiff is the lines that differ, marked - for the golden and + for the
// new output, enough to see what moved.
func lineDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	var out strings.Builder
	n := 0
	for i := 0; i < len(w) || i < len(g); i++ {
		var a, b string
		if i < len(w) {
			a = w[i]
		}
		if i < len(g) {
			b = g[i]
		}
		if a != b {
			fmt.Fprintf(&out, "line %d\n- %s\n+ %s\n", i+1, a, b)
			if n++; n >= 12 {
				out.WriteString("...\n")
				break
			}
		}
	}
	return out.String()
}
