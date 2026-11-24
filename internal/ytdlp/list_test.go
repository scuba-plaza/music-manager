package ytdlp_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/ytdlp"
)

const playlistJSON = `{"title":"Al Fajer The Dawn","entries":[
{"id":"F2wUzaEinfQ","title":"Halalalalayya","duration":430,"channel":"Release - Topic","uploader":"Other"},
{"id":"wFJZY_oSjQk","title":"3an Insan","duration":344,"channel":"Release - Topic"}]}`

var coverFixture = []byte("\x89PNG fake cover bytes")

func fakePlaylistBin(t *testing.T, listJSON string, listExit, coverExit int) (argsFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args.txt")
	jsonFile := filepath.Join(dir, "out.json")
	coverFile := filepath.Join(dir, "cover.bin")
	for p, data := range map[string][]byte{jsonFile: []byte(listJSON), coverFile: coverFixture} {
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	script := `#!/bin/sh
echo "---" >> "$FAKE_ARGS"
for a in "$@"; do printf '%s\n' "$a" >> "$FAKE_ARGS"; done
mode=""
out=""
for a in "$@"; do
  case "$a" in
    -J) mode=list ;;
    --write-thumbnail) mode=cover ;;
    pl_thumbnail:*) out="${a#pl_thumbnail:}" ;;
  esac
done
if [ "$mode" = "list" ]; then
  if [ "$FAKE_LIST_EXIT" != "0" ]; then echo "ERROR: Playlist does not exist" >&2; exit "$FAKE_LIST_EXIT"; fi
  cat "$FAKE_JSON"
  exit 0
fi
if [ "$FAKE_COVER_EXIT" != "0" ]; then echo "ERROR: no thumbnail" >&2; exit "$FAKE_COVER_EXIT"; fi
cp "$FAKE_COVER" "$out.jpg"
`
	if err := os.WriteFile(filepath.Join(dir, "yt-dlp"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_ARGS", argsFile)
	t.Setenv("FAKE_JSON", jsonFile)
	t.Setenv("FAKE_COVER", coverFile)
	t.Setenv("FAKE_LIST_EXIT", strconv.Itoa(listExit))
	t.Setenv("FAKE_COVER_EXIT", strconv.Itoa(coverExit))
	return argsFile
}

func TestListReadsThePlaylistAndItsCover(t *testing.T) {
	argsFile := fakePlaylistBin(t, playlistJSON, 0, 0)
	dir := t.TempDir()
	got, err := ytdlp.New().List(context.Background(), "https://example.com/playlist?list=PL1", dir, 31)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Al Fajer The Dawn" || got.Uploader != "Release - Topic" {
		t.Fatalf("got title %q uploader %q", got.Title, got.Uploader)
	}
	if len(got.Tracks) != 2 {
		t.Fatalf("got %d tracks", len(got.Tracks))
	}
	want := app.ListedTrack{URL: "https://www.youtube.com/watch?v=F2wUzaEinfQ", Title: "Halalalalayya", Length: 430 * time.Second}
	if got.Tracks[0] != want {
		t.Fatalf("got track %+v, want %+v", got.Tracks[0], want)
	}
	if string(got.Cover) != string(coverFixture) {
		t.Fatalf("got cover %q", got.Cover)
	}
	args, _ := os.ReadFile(argsFile)
	wantArgs := strings.Join([]string{
		"---",
		"--flat-playlist", "-J", "-q", "--no-warnings", "--playlist-end", "31", "--", "https://example.com/playlist?list=PL1",
		"---",
		"--flat-playlist", "--skip-download", "--write-thumbnail", "--convert-thumbnails", "jpg",
		"--playlist-items", "0", "-q", "--no-warnings",
		"-o", "pl_thumbnail:" + dir + "/cover", "-o", dir + "/%(id)s",
		"--", "https://example.com/playlist?list=PL1",
	}, "\n") + "\n"
	if string(args) != wantArgs {
		t.Fatalf("arguments:\n%s\nwant:\n%s", args, wantArgs)
	}
}

func TestListGoesOnWithoutACoverWhenTheCoverRunFails(t *testing.T) {
	fakePlaylistBin(t, playlistJSON, 0, 1)
	got, err := ytdlp.New().List(context.Background(), "https://example.com/p", t.TempDir(), 31)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cover != nil || len(got.Tracks) != 2 {
		t.Fatalf("got cover %v, %d tracks", got.Cover, len(got.Tracks))
	}
}

func TestListFailsWithTheLastLineOfWhatYtDlpSaid(t *testing.T) {
	fakePlaylistBin(t, playlistJSON, 1, 0)
	_, err := ytdlp.New().List(context.Background(), "https://example.com/p", t.TempDir(), 31)
	if err == nil || !strings.Contains(err.Error(), "ERROR: Playlist does not exist") {
		t.Fatalf("got %v", err)
	}
}

func TestListNeedsYtDlpOnThePath(t *testing.T) {
	_, err := (&ytdlp.Downloader{Bin: "no-such-yt-dlp"}).List(context.Background(), "https://example.com/p", t.TempDir(), 31)
	if err == nil || !strings.Contains(err.Error(), "not found in PATH") {
		t.Fatalf("got %v", err)
	}
}

func TestListSkipsEntriesWithoutAnID(t *testing.T) {
	fakePlaylistBin(t, `{"title":"T","entries":[{"title":"No id","duration":1},{"id":"abc","title":"Has id","duration":2,"uploader":"Up"}]}`, 0, 0)
	got, err := ytdlp.New().List(context.Background(), "https://example.com/p", t.TempDir(), 31)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tracks) != 1 || got.Tracks[0].Title != "Has id" {
		t.Fatalf("got %+v", got.Tracks)
	}
	if got.Uploader != "" {
		t.Fatalf("uploader %q", got.Uploader)
	}
}

func TestListOfASingleVideoHasNoTracks(t *testing.T) {
	fakePlaylistBin(t, `{"id":"abc","title":"A video"}`, 0, 0)
	got, err := ytdlp.New().List(context.Background(), "https://example.com/v", t.TempDir(), 31)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tracks) != 0 {
		t.Fatalf("got %+v", got.Tracks)
	}
}
