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
	cover []byte
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
	return app.Downloaded{Path: path, Info: f.info, Cover: f.cover}, nil
}

func newDownloader() *fakeDownloader {
	return &fakeDownloader{
		audio: testmp3.Silent(100),
		info:  app.Info{Title: "Song Title", Uploader: "Some Band - Topic", Album: "Yt Album", TrackNumber: 4},
	}
}

func strp(s string) *string { return &s }

// add fetches a link and adds it with what the download said for every detail
// not given, the way the page's form is filled in.
func add(a *app.App, req app.AddRequest, given app.Edit, dl app.Downloader) (manifest.Track, error) {
	d, err := a.Fetch(context.Background(), req, dl)
	if err != nil {
		return manifest.Track{}, err
	}
	return a.Finish(d, withDefaults(given, d.Info))
}

func withDefaults(given app.Edit, info app.Info) app.Edit {
	e := given
	artist := info.Artist
	if artist == "" {
		artist = strings.TrimSuffix(info.Uploader, " - Topic")
	}
	text := func(dst **string, v string) {
		if *dst == nil {
			v = strings.TrimSpace(v)
			*dst = &v
		}
	}
	count := func(dst **int, n int) {
		if *dst == nil && n > 0 {
			*dst = &n
		}
	}
	text(&e.Title, info.Title)
	text(&e.Artist, artist)
	text(&e.Album, info.Album)
	count(&e.TrackNumber, info.TrackNumber)
	count(&e.TrackTotal, info.TrackTotal)
	text(&e.Released, info.Released)
	text(&e.Genre, "")
	return e
}

func TestAddRecordsTheDetailsGivenAndTheSource(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	got, err := add(a, app.AddRequest{URL: "https://youtu.be/xyz"},
		app.Edit{Title: strp("وطني"), Artist: strp("فيروز"), Album: strp("Hits"), TrackNumber: ptr(3), TrackTotal: ptr(12), Released: strp("1999-05"), Genre: strp(""), TrimStart: ptr(time.Second), TrimEnd: ptr(0 * time.Second)}, dl)
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
	got, err := add(a, app.AddRequest{URL: "u"}, app.Edit{}, dl)
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
	_, err := add(a, app.AddRequest{URL: "u"}, app.Edit{}, dl)
	if err == nil || err.Error() != "missing artist" {
		t.Fatalf("err = %v, want exactly %q", err, "missing artist")
	}
	if len(tracks(t, a)) != 0 {
		t.Error("a track was added despite the error")
	}

	if _, err := add(a, app.AddRequest{URL: "u"}, app.Edit{Artist: strp("Band")}, dl); err != nil {
		t.Errorf("with an artist given: %v", err)
	}
}

func TestAddRefusesADuplicate(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	first, err := add(a, app.AddRequest{URL: "u1"}, app.Edit{}, dl)
	if err != nil {
		t.Fatal(err)
	}
	_, err = add(a, app.AddRequest{URL: "u2"}, app.Edit{}, dl)
	var dup *app.DuplicateError
	if !errors.As(err, &dup) || dup.ID != first.ID {
		t.Fatalf("err = %v, want a DuplicateError for %s", err, first.ID)
	}
	if len(tracks(t, a)) != 1 {
		t.Errorf("%d tracks", len(tracks(t, a)))
	}
}

func TestAddFailuresLeaveTheLibraryUntouched(t *testing.T) {
	t.Run("download fails", func(t *testing.T) {
		a, dl := openLib(t), newDownloader()
		dl.err = errors.New("video unavailable")
		if _, err := add(a, app.AddRequest{URL: "u"}, app.Edit{}, dl); err == nil || !strings.Contains(err.Error(), "video unavailable") {
			t.Errorf("err = %v", err)
		}
		if len(tracks(t, a)) != 0 {
			t.Error("track added")
		}
	})
	t.Run("trims longer than the track", func(t *testing.T) {
		a, dl := openLib(t), newDownloader() // 2.6s long
		_, err := add(a, app.AddRequest{URL: "u"}, app.Edit{TrimStart: ptr(5 * time.Second)}, dl)
		if err == nil {
			t.Error("accepted a 5s trim of a 2.6s track")
		}
		if len(tracks(t, a)) != 0 {
			t.Error("track added")
		}
	})
}

func TestAddStoresAGivenCover(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	dl.cover = testmp3.JPEG
	got, err := add(a, app.AddRequest{URL: "u"}, app.Edit{}, dl)
	if err != nil || got.CoverSHA256 == "" {
		t.Errorf("track %+v, err %v", got, err)
	}
}

func TestACoverThatIsNoImageIsLeftOutAndTheDownloadStillWorks(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	dl.cover = []byte("<html>not a picture</html>")

	d, err := a.Fetch(context.Background(), app.AddRequest{URL: "u"}, dl)

	if err != nil || d.CoverSHA() != "" {
		t.Errorf("cover %q, err %v", d.CoverSHA(), err)
	}
}

func TestTheCoverOfADraftCanBeReplacedAndTakenAwayBeforeItIsAdded(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	dl.cover = testmp3.JPEG
	d, err := a.Fetch(context.Background(), app.AddRequest{URL: "u"}, dl)
	if err != nil || d.CoverSHA() == "" {
		t.Fatalf("%q %v", d.CoverSHA(), err)
	}
	fetched := d.CoverSHA()

	if err := a.SetDraftCover(d, testmp3.PNG); err != nil || d.CoverSHA() == "" || d.CoverSHA() == fetched {
		t.Errorf("replaced: %q (was %q) %v", d.CoverSHA(), fetched, err)
	}
	if err := a.SetDraftCover(d, []byte("nope")); err == nil {
		t.Error("something that is no image became the cover")
	}
	a.RemoveDraftCover(d)

	if d.CoverSHA() != "" {
		t.Errorf("still %q", d.CoverSHA())
	}
	got, err := a.Finish(d, app.Edit{Title: strp("T"), Artist: strp("A")})
	if err != nil || got.CoverSHA256 != "" {
		t.Errorf("a track added with no cover has %q (%v)", got.CoverSHA256, err)
	}
}

func TestFetchPreparesADraftWithoutAddingAnything(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	d, err := a.Fetch(context.Background(), app.AddRequest{URL: "https://youtu.be/x"}, dl)
	if err != nil {
		t.Fatal(err)
	}
	if d.URL != "https://youtu.be/x" || d.Info != dl.info {
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

func TestFetchRefusesADuplicate(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	first, err := add(a, app.AddRequest{URL: "u1"}, app.Edit{}, dl)
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
	dl.cover = testmp3.JPEG
	d, err := a.Fetch(context.Background(), app.AddRequest{URL: "https://youtu.be/x"}, dl)
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.Finish(d, app.Edit{
		Title: strp("Dream 3"), Artist: strp("Max Richter"), Album: strp("From Sleep"), TrackNumber: ptr(1), TrackTotal: ptr(7),
		TrimStart: ptr(time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != manifest.Identified || got.SourceURL != "https://youtu.be/x" || got.TrimStart != time.Second || got.CoverSHA256 == "" {
		t.Errorf("track = %+v", got)
	}
	if saved := tracks(t, a); len(saved) != 1 || saved[0].ID != got.ID {
		t.Errorf("manifest = %+v", saved)
	}
	// A second Finish of the same draft must not add the audio twice.
	_, err = a.Finish(d, app.Edit{Title: strp("Again"), Artist: strp("Band")})
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
		if _, err := a.Finish(d, e); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if len(tracks(t, a)) != 0 {
		t.Error("a failed finish added a track")
	}
}

func TestAddTakesTheReleaseDateTheDownloadSaidUnlessOneIsGiven(t *testing.T) {
	dl := newDownloader()
	dl.info = app.Info{Title: "Dream 3", Uploader: "Some Channel", Released: "2015-09-04"}

	got, err := add(openLib(t), app.AddRequest{URL: "u"}, app.Edit{}, dl)
	if err != nil || got.Released != "2015-09-04" {
		t.Errorf("released %q, %v", got.Released, err)
	}

	got, err = add(openLib(t), app.AddRequest{URL: "u"}, app.Edit{Released: strp("1999")}, dl)
	if err != nil || got.Released != "1999" {
		t.Errorf("given: released %q, %v", got.Released, err)
	}
	if _, err := add(openLib(t), app.AddRequest{URL: "u"}, app.Edit{Released: strp("soon")}, dl); err == nil {
		t.Error("an invalid date was accepted")
	}
}

func TestAddTakesTheGenreGivenAndNoneOtherwise(t *testing.T) {
	a, dl := openLib(t), newDownloader()
	got, err := add(a, app.AddRequest{URL: "u"}, app.Edit{Genre: strp("Anthem")}, dl)
	if err != nil || got.Genre != "Anthem" {
		t.Errorf("given: genre %q, %v", got.Genre, err)
	}

	got, err = add(openLib(t), app.AddRequest{URL: "u"}, app.Edit{}, dl)
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

	_, err = a.Finish(d, app.Edit{Title: strp("Song"), Artist: strp("Band")})
	if err == nil || !strings.Contains(err.Error(), "download it again") {
		t.Fatalf("err = %v, want one that says to download it again", err)
	}
	if saved := tracks(t, a); len(saved) != 0 {
		t.Errorf("a record was added for audio that is gone: %+v", saved)
	}
}
