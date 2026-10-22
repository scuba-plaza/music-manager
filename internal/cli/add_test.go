package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/cli"
	"music-manager/internal/testmp3"
)

type fakeDownloader struct {
	info app.Info
	urls []string
}

func (f *fakeDownloader) Download(ctx context.Context, url, dir string) (app.Downloaded, error) {
	f.urls = append(f.urls, url)
	path := filepath.Join(dir, "v.mp3")
	return app.Downloaded{Path: path, Info: f.info}, os.WriteFile(path, testmp3.Silent(100), 0o644)
}

type scripted struct {
	answers map[string]string
	asked   []string
}

func (s *scripted) Choose(title string, options []string) (int, error) { return 0, nil }

func (s *scripted) Ask(q app.Question) (string, error) {
	s.asked = append(s.asked, q.Key)
	if a, ok := s.answers[q.Key]; ok {
		return a, nil
	}
	return q.Default, nil
}

func TestAddWithFlagsIsNonInteractive(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	dl := &fakeDownloader{info: app.Info{Title: "From Youtube", Uploader: "Chan"}}
	deps := cli.Deps{Downloader: dl, Prompter: &scripted{}}

	code, out, errs := runWith(t, deps, lib, "add", "https://youtu.be/x", "--no-input",
		"--title", "وطني", "--artist", "فيروز", "--album", "Hits", "--track", "3", "--of", "12", "--trim-end", "0:01")
	if code != 0 {
		t.Fatalf("code %d, err %q", code, errs)
	}
	for _, want := range []string{"added", "Firuz - Wtni", "Firuz/Hits/03 - Wtni.mp3", "mm sync"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if len(dl.urls) != 1 || dl.urls[0] != "https://youtu.be/x" {
		t.Errorf("downloaded %v", dl.urls)
	}
	if _, out, _ := run(t, lib, "show", firstID(t, lib)); !strings.Contains(out, "https://youtu.be/x") || !strings.Contains(out, "3/12") {
		t.Errorf("show:\n%s", out)
	}
}

func TestAddWithoutFlagsAsksOnlyWhenAPrompterIsAvailable(t *testing.T) {
	dl := &fakeDownloader{info: app.Info{Title: "Song", Uploader: "Band - Topic"}}

	t.Run("prompter present", func(t *testing.T) {
		p := &scripted{answers: map[string]string{"album": "Asked Album"}}
		lib := filepath.Join(t.TempDir(), "lib")
		code, _, errs := runWith(t, cli.Deps{Downloader: dl, Prompter: p}, lib, "add", "u1")
		if code != 0 {
			t.Fatalf("code %d, err %q", code, errs)
		}
		if got := strings.Join(p.asked, ","); got != "title,artist,album,track,of,released,trim-start,trim-end" {
			t.Errorf("asked %s", got)
		}
		if _, out, _ := run(t, lib, "ls"); !strings.Contains(out, "Asked Album") {
			t.Errorf("answers not applied:\n%s", out)
		}
	})

	t.Run("flags are not asked again", func(t *testing.T) {
		p := &scripted{}
		lib := filepath.Join(t.TempDir(), "lib")
		runWith(t, cli.Deps{Downloader: dl, Prompter: p}, lib, "add", "u1", "--artist", "Given", "--album", "", "--trim-start", "0")
		if got := strings.Join(p.asked, ","); got != "title,track,of,released,trim-end" {
			t.Errorf("asked %s", got)
		}
	})

	t.Run("--no-input takes the defaults", func(t *testing.T) {
		p := &scripted{}
		lib := filepath.Join(t.TempDir(), "lib")
		code, _, errs := runWith(t, cli.Deps{Downloader: dl, Prompter: p}, lib, "add", "u1", "--no-input")
		if code != 0 || len(p.asked) != 0 {
			t.Errorf("code %d, err %q, asked %v", code, errs, p.asked)
		}
	})

	t.Run("no prompter (not a terminal) takes the defaults", func(t *testing.T) {
		lib := filepath.Join(t.TempDir(), "lib")
		if code, _, errs := runWith(t, cli.Deps{Downloader: dl}, lib, "add", "u1"); code != 0 {
			t.Errorf("code %d, err %q", code, errs)
		}
	})
}

func TestAddReportsProblemsClearly(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")

	noArtist := &fakeDownloader{info: app.Info{Title: "Song"}}
	code, _, errs := runWith(t, cli.Deps{Downloader: noArtist}, lib, "add", "u")
	if code == 0 || !strings.Contains(errs, "--artist") || !strings.HasPrefix(errs, "mm: ") {
		t.Errorf("missing artist: code %d, err %q", code, errs)
	}

	deps := cli.Deps{Downloader: &fakeDownloader{info: app.Info{Title: "Song", Artist: "Band"}}}
	if code, _, errs := runWith(t, deps, lib, "add", "u"); code != 0 {
		t.Fatalf("first add: %s", errs)
	}
	id := firstID(t, lib)
	code, _, errs = runWith(t, deps, lib, "add", "u")
	if code == 0 || !strings.Contains(errs, id) || !strings.Contains(errs, "already") {
		t.Errorf("duplicate: code %d, err %q", code, errs)
	}

	if code, _, errs := runWith(t, deps, lib, "add", "u", "--trim-start", "nonsense"); code == 0 || !strings.Contains(errs, "--trim-start") {
		t.Errorf("bad trim: code %d, err %q", code, errs)
	}
	if code, _, _ := runWith(t, deps, lib, "add"); code == 0 {
		t.Error("add without a URL succeeded")
	}
	if code, _, errs := runWith(t, deps, lib, "add", "u", "--cover", filepath.Join(t.TempDir(), "nope.jpg")); code == 0 || !strings.Contains(errs, "--cover") {
		t.Errorf("missing cover file: code %d, err %q", code, errs)
	}
	if code, _, errs := runWith(t, cli.Deps{}, lib, "add", "u"); code == 0 || !strings.Contains(errs, "downloader") {
		t.Errorf("no downloader: code %d, err %q", code, errs)
	}
}

func firstID(t *testing.T, lib string) string {
	t.Helper()
	_, out, _ := run(t, lib, "ls")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return strings.Fields(lines[1])[0]
}
