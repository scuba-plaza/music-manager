package ytdlp_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/testmp3"
	"music-manager/internal/ytdlp"
)

// fakeBin installs a stand-in for yt-dlp that records its arguments, writes
// the fixture MP3 where -o says, and prints the given JSON.
func fakeBin(t *testing.T, json string, exit int) (argsFile string) {
	t.Helper()
	dir := t.TempDir()
	fixture := filepath.Join(dir, "fixture.mp3")
	jsonFile := filepath.Join(dir, "out.json")
	argsFile = filepath.Join(dir, "args.txt")
	for p, data := range map[string][]byte{fixture: testmp3.Silent(10), jsonFile: []byte(json)} {
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	script := `#!/bin/sh
for a in "$@"; do printf '%s\n' "$a" >> "$FAKE_ARGS"; done
prev=""
for a in "$@"; do
  if [ "$prev" = "-o" ]; then out="$a"; fi
  prev="$a"
done
if [ "$FAKE_EXIT" != "0" ]; then echo "ERROR: Video unavailable" >&2; exit "$FAKE_EXIT"; fi
file=$(printf '%s' "$out" | sed 's/%(id)s/vid123/; s/%(ext)s/mp3/')
cp "$FAKE_FIXTURE" "$file"
cat "$FAKE_JSON"
`
	if err := os.WriteFile(filepath.Join(dir, "yt-dlp"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_ARGS", argsFile)
	t.Setenv("FAKE_FIXTURE", fixture)
	t.Setenv("FAKE_JSON", jsonFile)
	t.Setenv("FAKE_EXIT", string(rune('0'+exit)))
	return argsFile
}

const musicJSON = `{"id":"vid123","title":"Video Title (Official)","track":"Real Track","artist":"The Artist","uploader":"Channel","album":"The Album","track_number":5.0}
`

func TestDownloadRunsYtDlpForAudioAndReportsTheMetadata(t *testing.T) {
	argsFile := fakeBin(t, musicJSON, 0)
	dir := t.TempDir()

	got, err := ytdlp.New().Download(context.Background(), "https://youtu.be/vid123", dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "vid123.mp3"); got.Path != want {
		t.Errorf("Path = %q, want %q", got.Path, want)
	}
	if _, err := os.Stat(got.Path); err != nil {
		t.Error(err)
	}
	want := app.Info{Title: "Real Track", Artist: "The Artist", Uploader: "Channel", Album: "The Album", TrackNumber: 5}
	if got.Info != want {
		t.Errorf("Info = %+v, want %+v", got.Info, want)
	}

	raw, _ := os.ReadFile(argsFile)
	args := strings.Split(strings.TrimSpace(string(raw)), "\n")
	for _, must := range []string{"--no-playlist", "-x", "--print-json", "-q"} {
		if !contains(args, must) {
			t.Errorf("args %v lack %s", args, must)
		}
	}
	if i := indexOf(args, "--audio-format"); i < 0 || args[i+1] != "mp3" {
		t.Errorf("args %v do not ask for mp3", args)
	}
	if i := indexOf(args, "-o"); i < 0 || args[i+1] != filepath.Join(dir, "%(id)s.%(ext)s") {
		t.Errorf("args %v: bad output template", args)
	}
	if args[len(args)-1] != "https://youtu.be/vid123" {
		t.Errorf("the URL must come last: %v", args)
	}
}

func TestDownloadFallsBackToTheVideoTitle(t *testing.T) {
	fakeBin(t, `{"id":"vid123","title":"Just A Title","uploader":"Chan"}`+"\n", 0)
	got, err := ytdlp.New().Download(context.Background(), "u", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got.Info != (app.Info{Title: "Just A Title", Uploader: "Chan"}) {
		t.Errorf("Info = %+v", got.Info)
	}
}

func TestDownloadToleratesNoiseBeforeTheJSON(t *testing.T) {
	fakeBin(t, "[youtube] some progress\n"+musicJSON, 0)
	if _, err := ytdlp.New().Download(context.Background(), "u", t.TempDir()); err != nil {
		t.Error(err)
	}
}

func TestDownloadFailureCarriesYtDlpsMessage(t *testing.T) {
	fakeBin(t, "", 1)
	_, err := ytdlp.New().Download(context.Background(), "u", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "Video unavailable") {
		t.Errorf("err = %v, want it to include yt-dlp's message", err)
	}
}

func TestDownloadWhenYtDlpIsMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := ytdlp.New().Download(context.Background(), "u", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "yt-dlp") || !strings.Contains(err.Error(), "PATH") {
		t.Errorf("err = %v, want a hint that yt-dlp is not installed", err)
	}
}

func TestDownloadStopsWhenTheContextIsCancelled(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "yt-dlp"), []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := ytdlp.New().Download(ctx, "u", t.TempDir()); err == nil {
		t.Error("no error after cancellation")
	}
	if time.Since(start) > 5*time.Second {
		t.Error("did not stop promptly")
	}
}

func contains(s []string, v string) bool { return indexOf(s, v) >= 0 }

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}
