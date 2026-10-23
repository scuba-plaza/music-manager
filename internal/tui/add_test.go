package tui

import (
	"sync"

	"context"
	"errors"
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/testmp3"
)

func TestAddDefaultsComeFromTheDownloadOrTheMatch(t *testing.T) {
	d := &app.Draft{
		Downloaded: app.Info{Title: "Song (Official Video)", Uploader: "Some Band - Topic", Album: "Yt Album", TrackNumber: 4, Released: "2012"},
		Info:       app.Info{Title: "Song", Artist: "Some Band", ArtistASCII: "", Album: "Real Album", TrackNumber: 2, TrackTotal: 9, Released: "2015-09-04"},
		Match:      &app.Candidate{Title: "Song"},
	}
	got := addDefaults(d, true)
	if got.title != "Song" || got.artist != "Some Band" || got.album != "Real Album" || got.track != "2" || got.of != "9" || got.released != "2015-09-04" {
		t.Errorf("with the match: %+v", got)
	}
	got = addDefaults(d, false)
	if got.title != "Song (Official Video)" || got.artist != "Some Band" || got.album != "Yt Album" || got.track != "4" || got.of != "" || got.released != "2012" {
		t.Errorf("without it: %+v (a '- Topic' uploader is the artist)", got)
	}
	d.Match = nil
	if got := addDefaults(d, true); got.title != "Song (Official Video)" {
		t.Errorf("no match to use: %+v", got)
	}
}

// fakeDownloader "downloads" silent audio and reports the given metadata.
type fakeDownloader struct {
	info    app.Info
	frames  int
	err     error
	block   bool // wait until cancelled
	calls   []string
	started chan struct{}
}

func (f *fakeDownloader) Download(ctx context.Context, url, dir string) (app.Downloaded, error) {
	f.calls = append(f.calls, url)
	if f.started != nil {
		close(f.started)
	}
	if f.block {
		<-ctx.Done()
		return app.Downloaded{}, ctx.Err()
	}
	if f.err != nil {
		return app.Downloaded{}, f.err
	}
	frames := f.frames
	if frames == 0 {
		frames = 120
	}
	path := filepath.Join(dir, "vid.mp3")
	return app.Downloaded{Path: path, Info: f.info}, os.WriteFile(path, testmp3.Silent(frames), 0o644)
}

func newDownloader() *fakeDownloader {
	return &fakeDownloader{info: app.Info{Title: "Song Title", Uploader: "Some Band - Topic", Album: "Yt Album", TrackNumber: 4}}
}

// addModel is an empty-ish library able to add from YouTube.
func addModel(t *testing.T, dl *fakeDownloader, src *fakeSource) (Model, *editHook) {
	t.Helper()
	a := testApp(t)
	seed(t, a, seedTrack{"Anchor", "Band", "Album", 1, 1, 0})
	opts := []LibraryOption{WithDownloader(dl)}
	if src != nil {
		opts = append(opts, WithSource(src))
	}
	hook := &editHook{Library: NewLibrary(a, opts...)}
	return newTestModel(t, hook, 120, 36), hook
}

// fetched opens the dialog, types the link and waits for the form.
func fetched(t *testing.T, m Model) Model {
	t.Helper()
	m = keys(t, m, "a")
	m = typed(t, m, "https://youtu.be/abc123")
	return keys(t, m, "enter")
}

func asAdd(t *testing.T, m Model) *addDialog {
	t.Helper()
	d, ok := m.overlay.(*addDialog)
	if !ok {
		t.Fatalf("the add dialog is not open:\n%s", screen(m))
	}
	return d
}

func TestAOpensTheDialogAndEscClosesIt(t *testing.T) {
	m, _ := addModel(t, newDownloader(), nil)
	m = keys(t, m, "a")
	contains(t, m, "Add from YouTube", "Paste a link")
	m = keys(t, m, "esc")
	if m.overlay != nil {
		t.Error("esc should close it")
	}
}

func TestALinkIsNeeded(t *testing.T) {
	m, _ := addModel(t, newDownloader(), nil)
	m = keys(t, m, "a", "enter")
	contains(t, m, "Paste a link first")
	if asAdd(t, m).state != addAsking {
		t.Error("should still be asking")
	}
}

func TestTheFormStartsFromWhatTheDownloadSaid(t *testing.T) {
	dl := newDownloader()
	m, _ := addModel(t, dl, nil)
	m = fetched(t, m)
	if len(dl.calls) != 1 || dl.calls[0] != "https://youtu.be/abc123" {
		t.Fatalf("downloaded %v", dl.calls)
	}
	contains(t, m, "Title", "Artist", "Album", "Track", "Released", "Genre", "Cut start", "Cut end")
	d := asAdd(t, m)
	if d.in[addTitle].Value() != "Song Title" || d.in[addArtist].Value() != "Some Band" || d.in[addAlbum].Value() != "Yt Album" || d.in[addTrack].Value() != "4" {
		t.Errorf("fields = %q %q %q %q", d.in[addTitle].Value(), d.in[addArtist].Value(), d.in[addAlbum].Value(), d.in[addTrack].Value())
	}
	contains(t, m, "No match on MusicBrainz", "Some Band/Yt Album/04 - Song Title.mp3")
}

func TestSavingAddsTheTrackSelectsItAndCanBeUndone(t *testing.T) {
	dl := newDownloader()
	m, hook := addModel(t, dl, nil)
	m = fetched(t, m)
	m = keys(t, m, "ctrl+u")
	m = typed(t, m, "My Title")
	contains(t, m, "Some Band/Yt Album/04 - My Title.mp3") // the path follows the typing
	m = keys(t, m, "enter")

	if m.overlay != nil {
		t.Fatalf("the dialog should close:\n%s", screen(m))
	}
	got := trackNamed(t, hook.Library, "My Title")
	if got.Status != manifest.Identified || got.SourceURL != "https://youtu.be/abc123" || got.AsciiArtist != "Some Band" || got.AsciiAlbum != "Yt Album" || got.TrackNumber != 4 {
		t.Errorf("track = %+v", got)
	}
	if r, ok := m.selectedRow(); !ok || r.Track.ID != got.ID {
		t.Errorf("the new track should be selected, got %+v", r)
	}
	contains(t, m, "Added", "My Title", "2 tracks")

	m = keys(t, m, "u")
	if len(libTracks(t, hook.Library)) != 1 {
		t.Error("undo should remove the added track again")
	}
	m = keys(t, m, "ctrl+r")
	if len(libTracks(t, hook.Library)) != 2 {
		t.Error("redo should bring it back")
	}
}

func TestEverythingInTheFormIsUsed(t *testing.T) {
	m, hook := addModel(t, newDownloader(), nil)
	m = fetched(t, m)
	d := asAdd(t, m)
	set := func(i int, v string) { d.in[i].SetValue(v) }
	set(addTitle, "Fancy")
	set(addArtist, "فيروز")
	set(addAlbum, "")
	set(addTrack, "3")
	set(addOf, "12")
	set(addReleased, "1999-05")
	set(addGenre, " Folk ")
	set(addCutStart, "0:01")
	set(addCutEnd, "0:00.5")
	m = keys(t, m, "enter")
	got := trackNamed(t, hook.Library, "Fancy")
	if got.OriginalArtist != "فيروز" || got.AsciiArtist != "Firuz" || got.AsciiAlbum != "" || got.TrackNumber != 3 || got.TrackTotal != 12 || got.Released != "1999-05" || got.Genre != "Folk" ||
		got.TrimStart != time.Second || got.TrimEnd != 500*time.Millisecond {
		t.Errorf("track = %+v", got)
	}
}

func TestMistakesInTheFormAreExplainedAndNothingIsAdded(t *testing.T) {
	m, hook := addModel(t, newDownloader(), nil)
	m = fetched(t, m)
	d := asAdd(t, m)
	d.in[addTitle].SetValue("")
	m = keys(t, m, "enter")
	contains(t, m, "Title cannot be empty")
	d.in[addTitle].SetValue("Ok")
	d.in[addReleased].SetValue("last year")
	m = keys(t, m, "enter")
	contains(t, m, "is not a date")
	d.in[addReleased].SetValue("")
	d.in[addCutStart].SetValue("1:99")
	m = keys(t, m, "enter")
	contains(t, m, "below 60")
	d.in[addCutStart].SetValue("0:30") // longer than the 3s of audio
	m = keys(t, m, "enter")
	contains(t, m, "leave nothing")
	if len(libTracks(t, hook.Library)) != 1 || asAdd(t, m).state != addForm {
		t.Error("a rejected form must stay open and add nothing")
	}
}

func TestEscInTheFormThrowsTheDownloadAway(t *testing.T) {
	m, hook := addModel(t, newDownloader(), nil)
	m = fetched(t, m)
	m = keys(t, m, "esc")
	if m.overlay != nil || len(libTracks(t, hook.Library)) != 1 {
		t.Error("esc should close the form and add nothing")
	}
}

func TestAConfidentMatchFillsTheFormAndCtrlTSwitchesToYouTubesWords(t *testing.T) {
	dl := newDownloader()
	dl.info = app.Info{Title: "Dream 3 (Official Video)", Uploader: "Some Channel"}
	src := &fakeSource{results: map[string][]app.Candidate{"Dream 3 (Official Video)|Some Channel": {richterA}}}
	m, hook := addModel(t, dl, src)
	m = fetched(t, m)
	contains(t, m, "Matched on MusicBrainz", "Max Richter", "From Sleep")
	d := asAdd(t, m)
	if got := d.in[addReleased].Value(); got != "2015-09-04" {
		t.Errorf("released = %q, want the release date of the match", got)
	}
	if d.in[addTitle].Value() != "Dream 3" || d.in[addArtist].Value() != "Max Richter" || d.in[addAlbum].Value() != "From Sleep" || d.in[addTrack].Value() != "1" || d.in[addOf].Value() != "7" {
		t.Fatalf("fields = %q %q %q %q/%q", d.in[addTitle].Value(), d.in[addArtist].Value(), d.in[addAlbum].Value(), d.in[addTrack].Value(), d.in[addOf].Value())
	}

	// A field you typed in keeps your words when the match is switched off.
	m = keys(t, m, "tab")
	m = typed(t, m, "!")
	m = keys(t, m, "ctrl+t")
	contains(t, m, "Ignoring the match")
	d = asAdd(t, m)
	if d.in[addTitle].Value() != "Dream 3 (Official Video)" || d.in[addArtist].Value() != "Max Richter!" || d.in[addAlbum].Value() != "" {
		t.Errorf("after ctrl+t: %q %q %q", d.in[addTitle].Value(), d.in[addArtist].Value(), d.in[addAlbum].Value())
	}
	m = keys(t, m, "ctrl+t")
	if asAdd(t, m).in[addTitle].Value() != "Dream 3" {
		t.Error("ctrl+t again should bring the match back")
	}

	// Saving with the match's words records where it came from.
	asAdd(t, m).in[addArtist].SetValue("Max Richter")
	m = keys(t, m, "enter")
	got := trackNamed(t, hook.Library, "Dream 3")
	if got.MBRecordingID != "rec-1" || got.MBReleaseID != "rel-1" {
		t.Errorf("track = %+v", got)
	}
}

func TestTheSameAudioTwiceSaysSoAndOffersTheTrack(t *testing.T) {
	m, hook := addModel(t, newDownloader(), nil)
	m = fetched(t, m)
	m = keys(t, m, "enter")
	first := trackNamed(t, hook.Library, "Song Title")

	m = keys(t, m, "g") // look elsewhere
	m = fetched(t, m)
	contains(t, m, "already in your library", "Song Title")
	if len(libTracks(t, hook.Library)) != 3-1 { // Anchor and the first copy
		t.Errorf("%d tracks", len(libTracks(t, hook.Library)))
	}
	m = keys(t, m, "enter")
	if m.overlay != nil {
		t.Error("enter should close the dialog")
	}
	if r, _ := m.selectedRow(); r.Track.ID != first.ID {
		t.Errorf("the existing track should be selected, got %+v", r)
	}
}

func TestADownloadThatFailsSaysWhyAndCanBeRetried(t *testing.T) {
	dl := newDownloader()
	dl.err = errors.New("yt-dlp: Video unavailable")
	m, hook := addModel(t, dl, nil)
	m = fetched(t, m)
	contains(t, m, "Video unavailable", "enter tries again")

	dl.err = nil
	m = keys(t, m, "enter")
	asAdd(t, m)
	if len(dl.calls) != 2 {
		t.Errorf("downloaded %d times", len(dl.calls))
	}
	m = keys(t, m, "enter")
	trackNamed(t, hook.Library, "Song Title")
}

func TestEscStopsADownloadInProgress(t *testing.T) {
	dl := newDownloader()
	dl.block, dl.started = true, make(chan struct{})
	m, _ := addModel(t, dl, nil)
	m = keys(t, m, "a")
	m = typed(t, m, "https://youtu.be/slow")
	next, fetch := m.Update(press("enter"))
	m = next.(Model)
	contains(t, m, "Downloading")

	// Run the commands the way the program would: all of them, at once.
	var running sync.WaitGroup
	for _, c := range fetch().(tea.BatchMsg) {
		running.Add(1)
		go func() { defer running.Done(); c() }()
	}
	done := make(chan struct{})
	go func() { running.Wait(); close(done) }()
	<-dl.started
	m = keys(t, m, "esc")
	if m.overlay != nil {
		t.Fatal("esc should close the dialog at once")
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Error("the download was not told to stop")
	}
}

func TestAddNeedsADownloader(t *testing.T) {
	m, _ := browserModel(t, 120, 36)
	m = keys(t, m, "a")
	if m.overlay != nil {
		t.Error("a dialog opened")
	}
	contains(t, m, "not available")
}

func TestPastedLinksAreCleanedUp(t *testing.T) {
	dl := newDownloader()
	m, _ := addModel(t, dl, nil)
	m = keys(t, m, "a")
	m = send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("  https://youtu.be/pasted  "), Paste: true})
	m = keys(t, m, "enter")
	asAdd(t, m)
	if strings.Join(dl.calls, ",") != "https://youtu.be/pasted" {
		t.Errorf("downloaded %q", dl.calls)
	}
}
