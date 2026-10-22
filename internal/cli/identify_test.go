package cli_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/cli"
	"music-manager/internal/testmp3"
)

type fakeMeta struct {
	cands []app.Candidate
	err   error
	asked []app.Query
}

func (f *fakeMeta) Search(ctx context.Context, q app.Query) ([]app.Candidate, error) {
	f.asked = append(f.asked, q)
	return f.cands, f.err
}
func (f *fakeMeta) CoverFront(ctx context.Context, id string) ([]byte, error) { return nil, nil }

var richter = app.Candidate{RecordingID: "r", ReleaseID: "l", Title: "Dream 3", Artist: "Max Richter", Album: "From Sleep", TrackNumber: 1, TrackTotal: 7, Score: 100, Date: "2015"}

func taggedLib(t *testing.T) string {
	t.Helper()
	lib := filepath.Join(t.TempDir(), "lib")
	f := filepath.Join(t.TempDir(), "01 - Dream 3.mp3")
	writeMP3(t, f, testmp3.WithTags(testmp3.Silent(60), "Dream 3", "Max Richter", ""))
	if code, _, errs := run(t, lib, "ingest", f); code != 0 {
		t.Fatal(errs)
	}
	return lib
}

func TestIdentifyYes(t *testing.T) {
	lib := taggedLib(t)
	meta := &fakeMeta{cands: []app.Candidate{richter}}
	code, out, errs := runWith(t, cli.Deps{Metadata: meta}, lib, "identify", "--yes")
	if code != 0 {
		t.Fatalf("code %d, err %q", code, errs)
	}
	for _, want := range []string{"Max Richter - Dream 3", "From Sleep", "identified 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if _, out, _ := run(t, lib, "show", firstID(t, lib)); !strings.Contains(out, "identified") || !strings.Contains(out, "1/7") {
		t.Errorf("show:\n%s", out)
	}
}

func TestIdentifyInteractiveChoice(t *testing.T) {
	lib := taggedLib(t)
	other := richter
	other.Album = "Voyager"
	p := &chooser{pick: 1}
	meta := &fakeMeta{cands: []app.Candidate{richter, other}}
	code, _, errs := runWith(t, cli.Deps{Metadata: meta, Prompter: p}, lib, "identify")
	if code != 0 {
		t.Fatalf("code %d, err %q", code, errs)
	}
	if _, out, _ := run(t, lib, "show", firstID(t, lib)); !strings.Contains(out, "Voyager") {
		t.Errorf("the second candidate was not applied:\n%s", out)
	}
}

type chooser struct {
	scripted
	pick int
}

func (c *chooser) Choose(title string, options []string) (int, error) { return c.pick, nil }

func TestIdentifyReportsSkipsAndFailures(t *testing.T) {
	lib := taggedLib(t)
	weak := richter
	weak.Score = 60
	code, out, _ := runWith(t, cli.Deps{Metadata: &fakeMeta{cands: []app.Candidate{weak}}}, lib, "identify", "--yes")
	if code != 0 || !strings.Contains(out, "skipped") || !strings.Contains(out, "scores only 60") || !strings.Contains(out, "identified 0") {
		t.Errorf("weak match: code %d, out %q", code, out)
	}

	code, out, _ = runWith(t, cli.Deps{Metadata: &fakeMeta{err: errors.New("network down")}}, lib, "identify", "--yes")
	if code == 0 || !strings.Contains(out, "network down") {
		t.Errorf("failed lookup: code %d, out %q", code, out)
	}
}

func TestIdentifyNeedsASourceAndAnswers(t *testing.T) {
	lib := taggedLib(t)
	if code, _, errs := runWith(t, cli.Deps{}, lib, "identify", "--yes"); code == 0 || !strings.Contains(errs, "lookup") {
		t.Errorf("no source: code %d, err %q", code, errs)
	}
	if code, _, errs := runWith(t, cli.Deps{Metadata: &fakeMeta{}}, lib, "identify"); code == 0 || !strings.Contains(errs, "--yes") {
		t.Errorf("no terminal and no --yes: code %d, err %q", code, errs)
	}
	if code, _, errs := runWith(t, cli.Deps{Metadata: &fakeMeta{}}, lib, "identify", "nope", "--yes"); code == 0 || !strings.Contains(errs, "nope") {
		t.Errorf("unknown id: code %d, err %q", code, errs)
	}
}

func TestAddLooksTheTrackUpAndUsesItAsTheDefault(t *testing.T) {
	lib := filepath.Join(t.TempDir(), "lib")
	dl := &fakeDownloader{info: app.Info{Title: "Dream 3 (Official Video)", Uploader: "Some Channel"}}
	meta := &fakeMeta{cands: []app.Candidate{richter}}
	code, out, errs := runWith(t, cli.Deps{Downloader: dl, Metadata: meta}, lib, "add", "u", "--no-input")
	if code != 0 {
		t.Fatalf("code %d, err %q", code, errs)
	}
	if len(meta.asked) != 1 || meta.asked[0].Title != "Dream 3 (Official Video)" || meta.asked[0].Artist != "Some Channel" {
		t.Errorf("asked %+v", meta.asked)
	}
	if !strings.Contains(out, "Max Richter - Dream 3") {
		t.Errorf("the match was not used:\n%s", out)
	}

	// --no-lookup keeps what the download said, and a failing lookup never blocks the add.
	for name, args := range map[string][]string{"no-lookup": {"--no-lookup"}} {
		lib := filepath.Join(t.TempDir(), name)
		meta := &fakeMeta{cands: []app.Candidate{richter}}
		code, out, _ := runWith(t, cli.Deps{Downloader: dl, Metadata: meta}, lib, append([]string{"add", "u", "--no-input", "--artist", "Given"}, args...)...)
		if code != 0 || len(meta.asked) != 0 || !strings.Contains(out, "Given - Dream 3 (Official Video)") {
			t.Errorf("%s: code %d, asked %v, out %q", name, code, meta.asked, out)
		}
	}
	lib2 := filepath.Join(t.TempDir(), "failing")
	code, out, errs = runWith(t, cli.Deps{Downloader: dl, Metadata: &fakeMeta{err: errors.New("offline")}}, lib2, "add", "u", "--no-input", "--artist", "Given")
	if code != 0 || !strings.Contains(out, "Given - Dream 3 (Official Video)") || !strings.Contains(errs, "offline") {
		t.Errorf("failing lookup: code %d, out %q, err %q", code, out, errs)
	}
}
