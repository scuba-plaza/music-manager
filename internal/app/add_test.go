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

func newDownloader() *fakeDownloader {
	return &fakeDownloader{
		audio: testmp3.Silent(100),
		info:  app.Info{Title: "Song Title", Uploader: "Some Band - Topic", Album: "Yt Album", TrackNumber: 4},
	}
}

func strp(s string) *string { return &s }

func TestAddRecordsTheDetailsGivenAndTheSource(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	got, err := a.Add(context.Background(), app.AddRequest{
		URL:  "https://youtu.be/xyz",
		Edit: app.Edit{Title: strp("وطني"), Artist: strp("فيروز"), Album: strp("Hits"), TrackNumber: ptr(3), TrackTotal: ptr(12), Released: strp("1999-05"), Genre: strp(""), TrimStart: ptr(time.Second), TrimEnd: ptr(0 * time.Second)},
	}, dl)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != manifest.Identified || got.SourceURL != "https://youtu.be/xyz" ||
		got.Title != "وطني" || got.Artist != "فيروز" || got.Album != "Hits" ||
		got.TrackNumber != 3 || got.TrackTotal != 12 || got.Released != "1999-05" || got.TrimStart != time.Second {
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

func TestAddFallsBackToTheDownloadMetadata(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	got, err := a.Add(context.Background(), app.AddRequest{URL: "u"}, dl)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Song Title" || got.Artist != "Some Band" || got.Album != "Yt Album" || got.TrackNumber != 4 || got.Status != manifest.Identified {
		t.Errorf("track = %+v", got)
	}
}

func TestAddReportsWhatIsMissing(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	dl.info = app.Info{Title: "Only A Title"}
	_, err := a.Add(context.Background(), app.AddRequest{URL: "u"}, dl)
	if err == nil || err.Error() != "missing artist" {
		t.Fatalf("err = %v, want exactly %q", err, "missing artist")
	}
	if len(tracks(t, a)) != 0 {
		t.Error("a track was added despite the error")
	}

	if _, err := a.Add(context.Background(), app.AddRequest{URL: "u", Edit: app.Edit{Artist: strp("Band")}}, dl); err != nil {
		t.Errorf("with an artist given: %v", err)
	}
}

func TestAddRefusesADuplicate(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	first, err := a.Add(context.Background(), app.AddRequest{URL: "u1"}, dl)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Add(context.Background(), app.AddRequest{URL: "u2"}, dl)
	var dup *app.DuplicateError
	if !errors.As(err, &dup) || dup.ID != first.ID {
		t.Fatalf("err = %v, want a DuplicateError for %s", err, first.ID)
	}
	if len(tracks(t, a)) != 1 {
		t.Errorf("%d tracks", len(tracks(t, a)))
	}
}

func TestAddFailuresLeaveTheLibraryUntouched(t *testing.T) {
	ctx := context.Background()
	t.Run("download fails", func(t *testing.T) {
		a, dl := openLib(t), newDownloader()
		dl.err = errors.New("video unavailable")
		if _, err := a.Add(ctx, app.AddRequest{URL: "u"}, dl); err == nil || !strings.Contains(err.Error(), "video unavailable") {
			t.Errorf("err = %v", err)
		}
		if len(tracks(t, a)) != 0 {
			t.Error("track added")
		}
	})
	t.Run("trims longer than the track", func(t *testing.T) {
		a, dl := openLib(t), newDownloader() // 2.6s long
		_, err := a.Add(ctx, app.AddRequest{URL: "u", Edit: app.Edit{TrimStart: ptr(5 * time.Second)}}, dl)
		if err == nil {
			t.Error("accepted a 5s trim of a 2.6s track")
		}
		if len(tracks(t, a)) != 0 {
			t.Error("track added")
		}
	})
	t.Run("cover that is not an image", func(t *testing.T) {
		a, dl := openLib(t), newDownloader()
		if _, err := a.Add(ctx, app.AddRequest{URL: "u", Cover: []byte("text")}, dl); err == nil {
			t.Error("accepted a text cover")
		}
		if len(tracks(t, a)) != 0 {
			t.Error("track added")
		}
	})
}

func TestAddStoresAGivenCover(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	got, err := a.Add(context.Background(), app.AddRequest{URL: "u", Cover: testmp3.JPEG}, dl)
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
	got, err := a.Add(context.Background(), app.AddRequest{URL: "u", Source: src}, dl)
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
	got, err := a.Add(ctx, app.AddRequest{URL: "u", Source: src}, dl)
	if err != nil || got.Title != "Song Title" || got.MBRecordingID != "" {
		t.Errorf("weak match used: %+v, %v", got, err)
	}

	src = &fakeSource{results: map[string][]app.Candidate{"Song Title|Some Band": {dream3}}}
	a2 := openLib(t)
	d, err := a2.Fetch(ctx, app.AddRequest{URL: "u", Source: src}, dl)
	if err != nil || d.Match == nil {
		t.Fatalf("draft %+v, %v", d, err)
	}
	got, err = a2.Finish(ctx, d, app.Edit{Title: strp("My Own Title"), Artist: strp(d.Info.Artist)})
	if err != nil || got.Title != "My Own Title" || got.MBRecordingID != "" || got.MBReleaseID != "" {
		t.Errorf("overridden match kept its ids: %+v, %v", got, err)
	}
}

func TestAddWarnsButCarriesOnWhenTheLookupFails(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	src := &fakeSource{errs: map[string]error{"Song Title|Some Band": errors.New("offline")}}
	var warned []error
	got, err := a.Add(context.Background(), app.AddRequest{URL: "u", Source: src, OnWarning: func(e error) { warned = append(warned, e) }}, dl)
	if err != nil || got.Title != "Song Title" {
		t.Fatalf("%+v, %v", got, err)
	}
	if len(warned) != 1 || !strings.Contains(warned[0].Error(), "offline") {
		t.Errorf("warnings = %v", warned)
	}
}

func TestAddKeepsTheArtistAsTheMatchWritesIt(t *testing.T) {
	dl := newDownloader()
	dl.info = app.Info{Title: "سأرجع", Uploader: "Fairuz"}
	arabic := app.Candidate{RecordingID: "r", ReleaseID: "l", Title: "سأرجع", Artist: "فيروز", Album: "Al Quds", Score: 100}
	src := &fakeSource{results: map[string][]app.Candidate{"سأرجع|Fairuz": {arabic}}}

	got, err := openLib(t).Add(context.Background(), app.AddRequest{URL: "u", Source: src}, dl)
	if err != nil || got.Artist != "فيروز" {
		t.Errorf("%+v, %v", got, err)
	}
}

func TestFetchPreparesADraftWithoutAddingAnything(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	d, err := a.Fetch(context.Background(), app.AddRequest{URL: "https://youtu.be/x"}, dl)
	if err != nil {
		t.Fatal(err)
	}
	if d.URL != "https://youtu.be/x" || d.Info != dl.info || d.Downloaded != dl.info || d.Match != nil {
		t.Errorf("draft = %+v", d)
	}
	if diff := d.Length - 2612*time.Millisecond; diff < -time.Millisecond || diff > time.Millisecond {
		t.Errorf("Length = %v", d.Length)
	}
	if len(tracks(t, a)) != 0 {
		t.Error("a draft must not add a track")
	}
	if _, err := os.Stat(dl.dirs[0]); !os.IsNotExist(err) {
		t.Errorf("download directory %s was left behind", dl.dirs[0])
	}
}

func TestFetchOffersAConfidentMatchAndKeepsWhatTheDownloadSaid(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	dl.info = app.Info{Title: "Dream 3 (Official Video)", Uploader: "Some Channel"}
	src := &fakeSource{results: map[string][]app.Candidate{"Dream 3 (Official Video)|Some Channel": {dream3}}}
	d, err := a.Fetch(context.Background(), app.AddRequest{URL: "u", Source: src}, dl)
	if err != nil {
		t.Fatal(err)
	}
	if d.Match == nil || d.Match.ReleaseID != "rel-1" || d.Info.Title != "Dream 3" || d.Info.Album != "From Sleep" || d.Info.TrackNumber != 1 {
		t.Errorf("draft = %+v", d)
	}
	if d.Downloaded.Title != "Dream 3 (Official Video)" {
		t.Errorf("Downloaded = %+v; the original report should be kept so a match can be declined", d.Downloaded)
	}
}

func TestFetchRefusesADuplicate(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	first, err := a.Add(context.Background(), app.AddRequest{URL: "u1"}, dl)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Fetch(context.Background(), app.AddRequest{URL: "u2"}, dl)
	var dup *app.DuplicateError
	if !errors.As(err, &dup) || dup.ID != first.ID {
		t.Errorf("err = %v", err)
	}
}

func TestFinishSavesTheTrackWithTheGivenDetails(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	src := &fakeSource{results: map[string][]app.Candidate{"Song Title|Some Band": {dream3}}, covers: map[string][]byte{"rel-1": testmp3.JPEG}}
	d, err := a.Fetch(context.Background(), app.AddRequest{URL: "https://youtu.be/x", Source: src}, dl)
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.Finish(context.Background(), d, app.Edit{
		Title: strp("Dream 3"), Artist: strp("Max Richter"), Album: strp("From Sleep"), TrackNumber: ptr(1), TrackTotal: ptr(7),
		TrimStart: ptr(time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != manifest.Identified || got.SourceURL != "https://youtu.be/x" || got.TrimStart != time.Second ||
		got.MBRecordingID != "rec-1" || got.MBReleaseID != "rel-1" || got.CoverSHA256 == "" {
		t.Errorf("track = %+v", got)
	}
	if saved := tracks(t, a); len(saved) != 1 || saved[0].ID != got.ID {
		t.Errorf("manifest = %+v", saved)
	}
	// A second Finish of the same draft must not add the audio twice.
	_, err = a.Finish(context.Background(), d, app.Edit{Title: strp("Again"), Artist: strp("Band")})
	var dup *app.DuplicateError
	if !errors.As(err, &dup) || len(tracks(t, a)) != 1 {
		t.Errorf("second finish: err %v, %d tracks", err, len(tracks(t, a)))
	}
}

func TestFinishFailuresAddNothing(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	d, err := a.Fetch(context.Background(), app.AddRequest{URL: "u"}, dl)
	if err != nil {
		t.Fatal(err)
	}
	for name, e := range map[string]app.Edit{
		"no artist":     {Title: strp("T"), Artist: strp("")},
		"trims too big": {Title: strp("T"), Artist: strp("A"), TrimStart: ptr(time.Hour)},
	} {
		if _, err := a.Finish(context.Background(), d, e); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if len(tracks(t, a)) != 0 {
		t.Error("a failed finish added a track")
	}
}

func TestAddTakesTheReleaseDateOfTheMatchUnlessOneIsGiven(t *testing.T) {
	dl := newDownloader()
	dl.info = app.Info{Title: "Dream 3 (Official Video)", Uploader: "Some Channel"}
	src := &fakeSource{results: map[string][]app.Candidate{"Dream 3 (Official Video)|Some Channel": {dream3}}}
	ctx := context.Background()

	got, err := openLib(t).Add(ctx, app.AddRequest{URL: "u", Source: src}, dl)
	if err != nil || got.Released != "2015-09-04" {
		t.Errorf("released %q, %v", got.Released, err)
	}

	got, err = openLib(t).Add(ctx, app.AddRequest{URL: "u", Source: src, Edit: app.Edit{Released: strp("1999")}}, dl)
	if err != nil || got.Released != "1999" {
		t.Errorf("given: released %q, %v", got.Released, err)
	}
	if _, err := openLib(t).Add(ctx, app.AddRequest{URL: "u", Edit: app.Edit{Released: strp("soon")}}, dl); err == nil {
		t.Error("an invalid date was accepted")
	}
}

func TestAddTakesTheGenreGivenAndNoneOtherwise(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	got, err := a.Add(context.Background(), app.AddRequest{URL: "u", Edit: app.Edit{Genre: strp("Anthem")}}, dl)
	if err != nil || got.Genre != "Anthem" {
		t.Errorf("given: genre %q, %v", got.Genre, err)
	}

	got, err = openLib(t).Add(context.Background(), app.AddRequest{URL: "u"}, dl)
	if err != nil || got.Genre != "" {
		t.Errorf("none given: genre %q, %v", got.Genre, err)
	}
}

func TestFinishRefusesADraftWhoseAudioASyncCollectedMeanwhile(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	d, err := a.Fetch(context.Background(), app.AddRequest{URL: "https://youtu.be/x"}, dl)
	if err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, app.SyncOptions{}) // no record holds the draft's audio yet, so a sync collects it

	_, err = a.Finish(context.Background(), d, app.Edit{Title: strp("Song"), Artist: strp("Band")})
	if err == nil || !strings.Contains(err.Error(), "download it again") {
		t.Fatalf("err = %v, want one that says to download it again", err)
	}
	if saved := tracks(t, a); len(saved) != 0 {
		t.Errorf("a record was added for audio that is gone: %+v", saved)
	}
}
