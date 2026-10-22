package app_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/testmp3"
)

// fakeDownloader "downloads" a fixed MP3 and reports the given metadata.
type fakeDownloader struct {
	audio []byte
	info  app.Info
	err   error
	calls []string // URLs asked for
	dirs  []string // where the file was put
}

func (f *fakeDownloader) Download(ctx context.Context, url, dir string) (app.Downloaded, error) {
	f.calls = append(f.calls, url)
	f.dirs = append(f.dirs, dir)
	if f.err != nil {
		return app.Downloaded{}, f.err
	}
	path := filepath.Join(dir, "vid123.mp3")
	if err := os.WriteFile(path, f.audio, 0o644); err != nil {
		return app.Downloaded{}, err
	}
	return app.Downloaded{Path: path, Info: f.info}, nil
}

// scriptedPrompter answers by question key and records what was asked.
type scriptedPrompter struct {
	answers  map[string]string // missing key: accept the default
	asked    []string
	defaults map[string]string
	abort    bool
	choice   int      // index Choose returns
	options  []string // what Choose was last offered
}

func (p *scriptedPrompter) Choose(title string, options []string) (int, error) {
	if p.abort {
		return 0, app.ErrAborted
	}
	p.options = options
	return p.choice, nil
}

func (p *scriptedPrompter) Ask(q app.Question) (string, error) {
	if p.abort {
		return "", app.ErrAborted
	}
	p.asked = append(p.asked, q.Key)
	if p.defaults == nil {
		p.defaults = map[string]string{}
	}
	p.defaults[q.Key] = q.Default
	if a, ok := p.answers[q.Key]; ok {
		if q.Validate != nil {
			if err := q.Validate(a); err != nil {
				return "", errors.New("scripted answer rejected: " + err.Error())
			}
		}
		return a, nil
	}
	return q.Default, nil
}

func newDownloader() *fakeDownloader {
	return &fakeDownloader{
		audio: testmp3.Silent(100),
		info:  app.Info{Title: "Song Title", Uploader: "Some Band - Topic", Album: "Yt Album", TrackNumber: 4},
	}
}

func strp(s string) *string { return &s }

func TestAddWithEverythingGivenNeverAsksAndRecordsTheSource(t *testing.T) {
	a, dl, p := openLib(t), newDownloader(), &scriptedPrompter{}
	got, err := a.Add(context.Background(), app.AddRequest{
		URL:  "https://youtu.be/xyz",
		Edit: app.Edit{Title: strp("وطني"), Artist: strp("فيروز"), Album: strp("Hits"), TrackNumber: ptr(3), TrackTotal: ptr(12), TrimStart: ptr(time.Second), TrimEnd: ptr(0 * time.Second)},
	}, dl, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.asked) != 0 {
		t.Errorf("asked %v although everything was given", p.asked)
	}
	if got.Status != manifest.Identified || got.SourceURL != "https://youtu.be/xyz" ||
		got.Title != "Wtni" || got.OriginalTitle != "وطني" || got.Artist != "Firuz" || got.Album != "Hits" ||
		got.TrackNumber != 3 || got.TrackTotal != 12 || got.TrimStart != time.Second {
		t.Errorf("track = %+v", got)
	}
	if saved := tracks(t, a); len(saved) != 1 || saved[0].ID != got.ID {
		t.Errorf("manifest = %+v", saved)
	}
	if _, err := os.Stat(a.AudioPath(got.AudioSHA256)); err != nil {
		t.Errorf("audio not stored: %v", err)
	}
	if _, err := os.Stat(dl.dirs[0]); !os.IsNotExist(err) {
		t.Errorf("download directory %s was not cleaned up", dl.dirs[0])
	}
}

func TestAddWithoutPrompterFallsBackToTheDownloadMetadata(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	got, err := a.Add(context.Background(), app.AddRequest{URL: "u"}, dl, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Song Title" || got.Artist != "Some Band" || got.Album != "Yt Album" || got.TrackNumber != 4 || got.Status != manifest.Identified {
		t.Errorf("track = %+v", got)
	}
}

func TestAddWithoutPrompterReportsWhatIsMissing(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	dl.info = app.Info{Title: "Only A Title"}
	_, err := a.Add(context.Background(), app.AddRequest{URL: "u"}, dl, nil)
	if err == nil || !strings.Contains(err.Error(), "artist") || strings.Contains(err.Error(), "title") {
		t.Fatalf("err = %v, want one naming only the missing artist", err)
	}
	if len(tracks(t, a)) != 0 {
		t.Error("a track was added despite the error")
	}

	// A flag fills the gap.
	if _, err := a.Add(context.Background(), app.AddRequest{URL: "u", Edit: app.Edit{Artist: strp("Band")}}, dl, nil); err != nil {
		t.Errorf("with --artist: %v", err)
	}
}

func TestAddAsksEachFieldWithDefaultsFromTheDownload(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	p := &scriptedPrompter{answers: map[string]string{
		"title": "وطني", "artist": "فيروز", "title-ascii": "Watani", "album": "", "track": "7", "of": "9", "trim-start": "0:01.5", "trim-end": "",
	}}
	got, err := a.Add(context.Background(), app.AddRequest{URL: "u"}, dl, p)
	if err != nil {
		t.Fatal(err)
	}
	wantAsked := []string{"title", "title-ascii", "artist", "artist-ascii", "album", "track", "of", "trim-start", "trim-end"}
	if strings.Join(p.asked, ",") != strings.Join(wantAsked, ",") {
		t.Errorf("asked %v, want %v", p.asked, wantAsked)
	}
	for k, want := range map[string]string{"title": "Song Title", "artist": "Some Band", "album": "Yt Album", "track": "4", "title-ascii": "Wtni"} {
		if p.defaults[k] != want {
			t.Errorf("default for %s = %q, want %q", k, p.defaults[k], want)
		}
	}
	if got.Title != "Watani" || got.OriginalTitle != "وطني" || got.Artist != "Firuz" || got.Album != "" ||
		got.TrackNumber != 7 || got.TrackTotal != 9 || got.TrimStart != 1500*time.Millisecond || got.TrimEnd != 0 {
		t.Errorf("track = %+v", got)
	}
}

func TestAddOnlyAsksForFieldsNotGivenAsFlags(t *testing.T) {
	a, dl, p := openLib(t), newDownloader(), &scriptedPrompter{}
	_, err := a.Add(context.Background(), app.AddRequest{URL: "u",
		Edit: app.Edit{Title: strp("Given"), Album: strp(""), TrimEnd: ptr(0 * time.Second)}}, dl, p)
	if err != nil {
		t.Fatal(err)
	}
	if want := "artist,track,of,trim-start"; strings.Join(p.asked, ",") != want {
		t.Errorf("asked %v, want %s", p.asked, want)
	}
}

func TestAddDoesNotOfferAnASCIIFormWhenTheTextIsAlreadyASCII(t *testing.T) {
	a, dl, p := openLib(t), newDownloader(), &scriptedPrompter{}
	if _, err := a.Add(context.Background(), app.AddRequest{URL: "u"}, dl, p); err != nil {
		t.Fatal(err)
	}
	for _, k := range p.asked {
		if strings.HasSuffix(k, "-ascii") {
			t.Errorf("asked for %s", k)
		}
	}
}

func TestAddAnswerValidation(t *testing.T) {
	a, dl, p := openLib(t), newDownloader(), &scriptedPrompter{}
	var questions []app.Question
	rec := recordingPrompter{inner: p, got: &questions}
	if _, err := a.Add(context.Background(), app.AddRequest{URL: "u"}, dl, rec); err != nil {
		t.Fatal(err)
	}
	by := map[string]app.Question{}
	for _, q := range questions {
		by[q.Key] = q
	}
	check := func(key, value string, wantOK bool) {
		t.Helper()
		err := by[key].Validate(value)
		if (err == nil) != wantOK {
			t.Errorf("%s validate(%q) = %v, want ok=%v", key, value, err, wantOK)
		}
	}
	check("title", "", false)
	check("title", "x", true)
	check("artist", "  ", false)
	check("album", "", true)
	check("track", "3", true)
	check("track", "", true)
	check("track", "-1", false)
	check("track", "abc", false)
	check("trim-start", "1:30", true)
	check("trim-start", "", true)
	check("trim-start", "1:99", false)
	if !by["title"].Required || !by["artist"].Required || by["album"].Required {
		t.Error("only title and artist are required")
	}
}

type recordingPrompter struct {
	inner app.Prompter
	got   *[]app.Question
}

func (r recordingPrompter) Choose(title string, options []string) (int, error) {
	return r.inner.Choose(title, options)
}

func (r recordingPrompter) Ask(q app.Question) (string, error) {
	*r.got = append(*r.got, q)
	return r.inner.Ask(q)
}

func TestAddRefusesADuplicateBeforeAskingAnything(t *testing.T) {
	a, dl, p := openLib(t), newDownloader(), &scriptedPrompter{}
	first, err := a.Add(context.Background(), app.AddRequest{URL: "u1"}, dl, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Add(context.Background(), app.AddRequest{URL: "u2"}, dl, p)
	var dup *app.DuplicateError
	if !errors.As(err, &dup) || dup.ID != first.ID {
		t.Fatalf("err = %v, want a DuplicateError for %s", err, first.ID)
	}
	if len(p.asked) != 0 || len(tracks(t, a)) != 1 {
		t.Errorf("asked %v, %d tracks", p.asked, len(tracks(t, a)))
	}
}

func TestAddFailuresLeaveTheLibraryUntouched(t *testing.T) {
	ctx := context.Background()
	t.Run("download fails", func(t *testing.T) {
		a, dl := openLib(t), newDownloader()
		dl.err = errors.New("video unavailable")
		if _, err := a.Add(ctx, app.AddRequest{URL: "u"}, dl, nil); err == nil || !strings.Contains(err.Error(), "video unavailable") {
			t.Errorf("err = %v", err)
		}
		if len(tracks(t, a)) != 0 {
			t.Error("track added")
		}
	})
	t.Run("user aborts", func(t *testing.T) {
		a, dl := openLib(t), newDownloader()
		if _, err := a.Add(ctx, app.AddRequest{URL: "u"}, dl, &scriptedPrompter{abort: true}); !errors.Is(err, app.ErrAborted) {
			t.Errorf("err = %v", err)
		}
		if len(tracks(t, a)) != 0 {
			t.Error("track added")
		}
	})
	t.Run("trims longer than the track", func(t *testing.T) {
		a, dl := openLib(t), newDownloader() // 2.6s long
		_, err := a.Add(ctx, app.AddRequest{URL: "u", Edit: app.Edit{TrimStart: ptr(5 * time.Second)}}, dl, nil)
		if err == nil {
			t.Error("accepted a 5s trim of a 2.6s track")
		}
		if len(tracks(t, a)) != 0 {
			t.Error("track added")
		}
	})
	t.Run("cover that is not an image", func(t *testing.T) {
		a, dl := openLib(t), newDownloader()
		if _, err := a.Add(ctx, app.AddRequest{URL: "u", Cover: []byte("text")}, dl, nil); err == nil {
			t.Error("accepted a text cover")
		}
		if len(tracks(t, a)) != 0 {
			t.Error("track added")
		}
	})
}

func TestAddStoresAGivenCover(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	got, err := a.Add(context.Background(), app.AddRequest{URL: "u", Cover: testmp3.JPEG}, dl, nil)
	if err != nil || got.CoverSHA256 == "" {
		t.Errorf("track %+v, err %v", got, err)
	}
}

func TestAddUsesAConfidentMatchAsDefaultsAndRecordsItsIDs(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	dl.info = app.Info{Title: "Dream 3 (Official Video)", Uploader: "Some Channel"}
	src := &fakeSource{
		results: map[string][]app.Candidate{"Dream 3 (Official Video)|Some Channel": {dream3}},
		covers:  map[string][]byte{"rel-1": testmp3.JPEG},
	}
	got, err := a.Add(context.Background(), app.AddRequest{URL: "u", Source: src}, dl, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Dream 3" || got.Artist != "Max Richter" || got.Album != "From Sleep" || got.TrackNumber != 1 || got.TrackTotal != 7 ||
		got.MBRecordingID != "rec-1" || got.MBReleaseID != "rel-1" || got.CoverSHA256 == "" {
		t.Errorf("track = %+v", got)
	}
}

func TestAddIgnoresWeakMatchesAndMatchesTheUserOverrode(t *testing.T) {
	ctx := context.Background()
	weak := dream3
	weak.Score = 70
	src := &fakeSource{results: map[string][]app.Candidate{"Song Title|Some Band": {weak}}}
	a, dl := openLib(t), newDownloader()
	got, err := a.Add(ctx, app.AddRequest{URL: "u", Source: src}, dl, nil)
	if err != nil || got.Title != "Song Title" || got.MBRecordingID != "" {
		t.Errorf("weak match used: %+v, %v", got, err)
	}

	// The user keeps the artist but changes the title: it is not that recording any more.
	src = &fakeSource{results: map[string][]app.Candidate{"Song Title|Some Band": {dream3}}}
	a2 := openLib(t)
	p := &scriptedPrompter{answers: map[string]string{"title": "My Own Title"}}
	got, err = a2.Add(ctx, app.AddRequest{URL: "u", Source: src}, dl, p)
	if err != nil || got.Title != "My Own Title" || got.MBRecordingID != "" {
		t.Errorf("overridden match kept its ids: %+v, %v", got, err)
	}
	if p.defaults["title"] != "Dream 3" {
		t.Errorf("the match should have been the offered default, got %q", p.defaults["title"])
	}
}

func TestAddWarnsButCarriesOnWhenTheLookupFails(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	src := &fakeSource{errs: map[string]error{"Song Title|Some Band": errors.New("offline")}}
	var warned []error
	got, err := a.Add(context.Background(), app.AddRequest{URL: "u", Source: src, OnWarning: func(e error) { warned = append(warned, e) }}, dl, nil)
	if err != nil || got.Title != "Song Title" {
		t.Fatalf("%+v, %v", got, err)
	}
	if len(warned) != 1 || !strings.Contains(warned[0].Error(), "offline") {
		t.Errorf("warnings = %v", warned)
	}
}

func TestAddCarriesTheLatinAliasOfAnArabicArtist(t *testing.T) {
	dl := newDownloader()
	dl.info = app.Info{Title: "سأرجع", Uploader: "Fairuz"}
	arabic := app.Candidate{RecordingID: "r", ReleaseID: "l", Title: "سأرجع", Artist: "فيروز", ArtistASCII: "Fairuz", Album: "Al Quds", Score: 100}
	src := &fakeSource{results: map[string][]app.Candidate{"سأرجع|Fairuz": {arabic}}}

	for name, p := range map[string]app.Prompter{"non-interactive": nil, "interactive": &scriptedPrompter{}} {
		lib := openLib(t)
		got, err := lib.Add(context.Background(), app.AddRequest{URL: "u", Source: src}, dl, p)
		if err != nil || got.Artist != "Fairuz" || got.OriginalArtist != "فيروز" {
			t.Errorf("%s: %+v, %v", name, got, err)
		}
	}
}
