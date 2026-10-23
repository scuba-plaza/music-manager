package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"music-manager/internal/app"
)

// The inspector's Latin tab shows how the title, the artist, the album and the
// genre read in Latin letters, and is where those forms are set by hand. A form
// left alone is the one worked out from the name.

// arabicModel is the browser with "Flow" renamed in Arabic, so that its Latin
// forms are worked out (Wtni, Firuz) until someone sets them.
func arabicModel(t *testing.T) (Model, *editHook) {
	t.Helper()
	m, hook := editingModel(t, "Flow")
	tr := selectedTrack(t, m)
	if _, err := appOf(t, hook.Library).Edit(tr.ID, app.Edit{Title: ptr("وطني"), Artist: ptr("فيروز")}); err != nil {
		t.Fatal(err)
	}
	m = send(t, m, m.Init())
	return selectTrack(t, m, "وطني"), hook
}

func TestTheLatinTabShowsTheFormsWorkedOutAndTheOnesSetByHand(t *testing.T) {
	m, hook := arabicModel(t)
	tr := trackNamed(t, hook.Library, "وطني")
	if _, err := appOf(t, hook.Library).Edit(tr.ID, app.Edit{ArtistLatin: ptr("Fairouz")}); err != nil {
		t.Fatal(err)
	}
	m = send(t, m, m.Init())
	m = selectTrack(t, m, "وطني")

	m = keys(t, m, "right")
	contains(t, m, "1 Details", "2 Audio", "3 Latin")
	m = keys(t, m, "3")
	contains(t, m, "Title", "Artist", "Album", "Genre", "Wtni", "Fairouz", "Shadows")
	lacks(t, m, "Firuz", "Released", "Cut start")
	if f := trackFields[m.field]; f != fLatinTitle {
		t.Errorf("the cursor is on %v, want the Latin title", f)
	}
}

func TestAFormIsSetByHandFromTheLatinTab(t *testing.T) {
	m, hook := arabicModel(t)
	m = keys(t, m, "right", "3", "enter")
	if got := m.in[0].Value(); got != "Wtni" {
		t.Fatalf("the editor starts from %q, want the form worked out", got)
	}
	m = clearField(t, m)
	m = typed(t, m, "  Watani ")
	m = keys(t, m, "enter")

	got := trackNamed(t, hook.Library, "وطني")
	if got.TitleLatin != "Watani" || got.Title != "وطني" {
		t.Errorf("saved %q for %q", got.TitleLatin, got.Title)
	}
	contains(t, m, "Saved", "Watani")
	lacks(t, m, "Wtni")

	// The artist, one row down, is edited the same way.
	m = keys(t, m, "down", "enter")
	m = clearField(t, m)
	m = typed(t, m, "Fairouz")
	m = keys(t, m, "enter")
	if got := trackNamed(t, hook.Library, "وطني"); got.ArtistLatin != "Fairouz" || got.TitleLatin != "Watani" {
		t.Errorf("saved %+v", got)
	}
}

func TestAFormLeftAsItIsSavesNothing(t *testing.T) {
	m, hook := arabicModel(t)
	m = keys(t, m, "right", "3", "enter", "enter")
	if m.editing || hook.calls != 0 {
		t.Errorf("editing %v, library calls %d", m.editing, hook.calls)
	}
	if got := trackNamed(t, hook.Library, "وطني"); got.TitleLatin != "" {
		t.Errorf("the worked-out form was saved as if set by hand: %q", got.TitleLatin)
	}
}

func TestAnEmptyFormGoesBackToTheOneWorkedOut(t *testing.T) {
	m, hook := arabicModel(t)
	m = keys(t, m, "right", "3", "enter")
	m = typed(t, clearField(t, m), "Watani")
	m = keys(t, m, "enter")
	contains(t, m, "Watani")

	m = keys(t, m, "enter")
	m = clearField(t, m)
	m = keys(t, m, "enter")
	if got := trackNamed(t, hook.Library, "وطني"); got.TitleLatin != "" {
		t.Errorf("an empty form should be forgotten: %q", got.TitleLatin)
	}
	contains(t, m, "Wtni")
	lacks(t, m, "Watani")
}

func TestTheEditorSaysWhatTheFormIsFor(t *testing.T) {
	m, _ := arabicModel(t)
	m = keys(t, m, "right", "3", "enter")
	contains(t, m, "وطني", "empty")
}

func TestAFormOfANameThatIsEmptyCannotBeEdited(t *testing.T) {
	m, hook := arabicModel(t)
	m = keys(t, m, "right", "3", "down", "down", "down") // the genre, which this track has none of
	if f := trackFields[m.field]; f != fLatinGenre {
		t.Fatalf("the cursor is on %v, want the Latin genre", f)
	}
	contains(t, m, "—")
	m = keys(t, m, "enter")
	if m.editing || hook.calls != 0 {
		t.Errorf("editing %v, library calls %d", m.editing, hook.calls)
	}
	contains(t, m, "no genre")
}

func TestAFormSetByHandIsOneStepToUndoAndRedo(t *testing.T) {
	m, hook := arabicModel(t)
	m = keys(t, m, "right", "3", "enter")
	m = typed(t, clearField(t, m), "Watani")
	m = keys(t, m, "enter", "u")
	contains(t, m, "Undid", "Latin title")
	if got := trackNamed(t, hook.Library, "وطني"); got.TitleLatin != "" {
		t.Errorf("undo left %q", got.TitleLatin)
	}
	m = keys(t, m, "ctrl+r")
	contains(t, m, "Redid")
	if got := trackNamed(t, hook.Library, "وطني"); got.TitleLatin != "Watani" {
		t.Errorf("redo gave %q", got.TitleLatin)
	}
}

func TestClickingAFormOnTheLatinTabChoosesIt(t *testing.T) {
	m, _ := arabicModel(t)
	m = keys(t, m, "right", "3")
	l := computeLayout(m.w, m.h)
	var row int
	for y, line := range strings.Split(ansi.Strip(m.View()), "\n") {
		if strings.Contains(string([]rune(line)[min(l.left, len([]rune(line))):]), "Album") {
			row = y
		}
	}
	if row == 0 {
		t.Fatal("no Album row on the Latin tab")
	}
	m = send(t, m, click(l.left+3, row))
	if f := trackFields[m.field]; f != fLatinAlbum {
		t.Errorf("the click chose %v, want the Latin album", f)
	}
}

func TestTheFooterCountsTheLatinTab(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "right")
	contains(t, m, "1-3 tabs")

	w := wolfgangModel(t, "Flow")
	w = keys(t, w, "right")
	contains(t, w, "1-4 tabs", "4 Wolfgang")
	w = keys(t, w, "4")
	contains(t, w, "07 - Flow.mp3")
}

func TestTheLatinTabFeedsTheWolfgangTab(t *testing.T) {
	m, hook := arabicModel(t)
	WithWolfgang()(&m)
	m = keys(t, m, "right", "3", "enter")
	m = typed(t, clearField(t, m), "Watani")
	m = keys(t, m, "enter")
	if got := trackNamed(t, hook.Library, "وطني"); got.TitleLatin != "Watani" {
		t.Fatalf("saved %q", got.TitleLatin)
	}
	m = keys(t, m, "4")
	contains(t, m, "Watani.mp3")
}
