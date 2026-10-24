package tui

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/clipboard"
	"music-manager/internal/testmp3"
)

// v also pastes a picture that was copied outside the interface, in a file
// manager or a browser, as the cover.

// fakeDesktop is the clipboard of the desktop: it holds what a test puts in it.
type fakeDesktop struct {
	data  []byte
	name  string
	err   error
	reads int
}

func (f *fakeDesktop) Image() ([]byte, string, error) {
	f.reads++
	if f.data == nil && f.err == nil {
		return nil, "", clipboard.ErrNoPicture
	}
	return f.data, f.name, f.err
}

// shaOf is the SHA-256 that a cover with these bytes is kept under.
func shaOf(b []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

// withDesktop is the model that can read the clipboard of the desktop.
func withDesktop(t *testing.T, title string) (Model, *editHook, *fakeDesktop) {
	t.Helper()
	m, hook := editingModel(t, title)
	desk := &fakeDesktop{}
	WithClipboard(desk)(&m)
	return m, hook, desk
}

func TestAPictureCopiedOutsideIsPastedAsTheCoverOfTheTrackWithV(t *testing.T) {
	for _, c := range []struct {
		name, file string
		image      []byte
	}{{"a PNG", "cover.png", testmp3.PNG}, {"a JPEG", "photo.jpg", testmp3.JPEG}, {"a picture, not a file", "", testmp3.PNG}} {
		m, hook, desk := withDesktop(t, "Along Our Way")
		desk.data, desk.name = c.image, c.file
		m = keys(t, m, "v")
		if got := coverOf(t, hook.Library, "Along Our Way"); got != shaOf(c.image) {
			t.Errorf("%s: the cover is %q, want %q", c.name, got, shaOf(c.image))
		}
		if c.file != "" {
			contains(t, m, "Pasted the cover from "+c.file+" on 1 track", "u undoes it")
		} else {
			contains(t, m, "Pasted the cover from the clipboard on 1 track")
		}
		if got := coverOf(t, hook.Library, "Flow"); got != "" {
			t.Errorf("%s: another track got the cover", c.name)
		}

		m = keys(t, m, "u")
		if got := coverOf(t, hook.Library, "Along Our Way"); got != "" {
			t.Errorf("%s: undo left the cover %q", c.name, got)
		}
		m = keys(t, m, "ctrl+r")
		if got := coverOf(t, hook.Library, "Along Our Way"); got != shaOf(c.image) {
			t.Errorf("%s: redo gave %q", c.name, got)
		}
	}
}

func TestAPictureIsPastedOnAnAlbumAndOnMarkedTracksToo(t *testing.T) {
	m, hook, desk := withDesktop(t, "Flow")
	desk.data = testmp3.PNG
	m = keys(t, m, "up") // The Wanderer
	m = keys(t, m, "up") // the album
	if r, _ := m.selectedRow(); r.Kind != kindAlbum {
		t.Fatalf("setup: on a %v", r.Kind)
	}
	m = keys(t, m, "v")
	contains(t, m, "on 3 tracks")
	for _, title := range []string{"The Wanderer", "Flow", "Along Our Way"} {
		if got := coverOf(t, hook.Library, title); got != shaOf(testmp3.PNG) {
			t.Errorf("%s: cover %q", title, got)
		}
	}

	desk.data = testmp3.JPEG
	m = selectTrack(t, m, "Current")
	m = keys(t, m, "space")
	m = selectTrack(t, m, "Wtni")
	m = keys(t, m, "space", "v")
	for _, title := range []string{"Current", "Wtni"} {
		if got := coverOf(t, hook.Library, title); got != shaOf(testmp3.JPEG) {
			t.Errorf("%s: cover %q", title, got)
		}
	}
}

func TestAPictureTheTrackHasAlreadyChangesNothing(t *testing.T) {
	m, hook, desk := withDesktop(t, "Along Our Way")
	m = withCoverOn(t, m, hook, "Along Our Way", testmp3.PNG)
	m = selectTrack(t, m, "Along Our Way")
	desk.data = testmp3.PNG
	m = keys(t, m, "v")
	contains(t, m, "Nothing to paste", "already has this cover")
	if len(m.undoStack) != 0 {
		t.Errorf("%d steps to undo", len(m.undoStack))
	}
}

func TestWhatWasCopiedWithCBeatsAPictureThatWasOnTheClipboardBefore(t *testing.T) {
	m, hook, desk := withDesktop(t, "Flow")
	desk.data = testmp3.PNG // copied outside, long ago
	m = withEdits(t, m, hook, map[string]app.Edit{"Flow": {Genre: ptr("Soul")}})
	m = copyFrom(t, m, "Flow", fGenre)

	m = selectTrack(t, m, "Current")
	m = keys(t, m, "v")
	if got := genreOf(t, hook.Library, "Current"); got != "Soul" {
		t.Errorf("genre %q: what was copied in the interface was not pasted", got)
	}
	if got := coverOf(t, hook.Library, "Current"); got != "" {
		t.Errorf("the older picture was pasted as well: %q", got)
	}

	// A picture copied after that is the newer.
	desk.data = testmp3.JPEG
	m = selectTrack(t, m, "Wtni")
	m = keys(t, m, "v")
	if got := coverOf(t, hook.Library, "Wtni"); got != shaOf(testmp3.JPEG) {
		t.Errorf("the newer picture was not pasted: %q", got)
	}
	if got := genreOf(t, hook.Library, "Wtni"); got != "" {
		t.Errorf("the older copy was pasted as well: %q", got)
	}
}

func TestWhenThereIsNothingToPasteAnywhereVSaysWhatToCopy(t *testing.T) {
	m, hook, desk := withDesktop(t, "Flow")
	m = keys(t, m, "v")
	contains(t, m, "Nothing is copied yet", "c copies", "picture file")

	desk.err = clipboard.ErrNoTool
	m = keys(t, m, "v")
	contains(t, m, "wl-paste")
	if hook.manyCalls != 0 || len(m.undoStack) != 0 {
		t.Error("something was saved")
	}
}

func TestAPictureThatCannotBeACoverIsExplainedAndNothingIsSaved(t *testing.T) {
	m, hook, desk := withDesktop(t, "Flow")
	desk.err = fmt.Errorf("cover.webp is not a JPEG or PNG picture: %w", clipboard.ErrUnsupported)
	m = keys(t, m, "v")
	contains(t, m, "cover.webp is not a JPEG or PNG picture")
	if got := coverOf(t, hook.Library, "Flow"); got != "" {
		t.Errorf("cover %q", got)
	}
}

func TestVOnAPlaylistRowDoesNotEvenLookAtTheClipboard(t *testing.T) {
	lib, _ := playlistLib(t)
	m := newTestModel(t, lib, 120, 36)
	desk := &fakeDesktop{data: testmp3.PNG}
	WithClipboard(desk)(&m)
	m = keys(t, m, "g", "v") // the group of playlists
	contains(t, m, "Select a track, an album or an artist")
	if desk.reads != 0 {
		t.Errorf("the clipboard was read %d times", desk.reads)
	}
}

func TestTheFooterOffersVWhenThePictureCouldComeFromTheDesktop(t *testing.T) {
	m, _, _ := withDesktop(t, "Flow")
	contains(t, m, "v paste")
	plain, _ := editingModel(t, "Flow")
	lacks(t, plain, "v paste")
}
