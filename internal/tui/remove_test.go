package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"testing"

	"music-manager/internal/app"
)

func TestDOnATrackAsksBeforeRemovingIt(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "d")
	contains(t, m, `Remove "Flow"?`, "sync", "u brings it back")
	contains(t, m, "y/enter remove", "esc cancel")
	if len(libTracks(t, hook.Library)) != 6 {
		t.Fatal("removed before being asked")
	}
}

func TestCancellingKeepsTheTrack(t *testing.T) {
	for _, k := range []string{"esc", "n", "q"} {
		m, hook := editingModel(t, "Flow")
		m = keys(t, m, "d", k)
		if m.overlay != nil || hook.manyCalls != 0 || len(libTracks(t, hook.Library)) != 6 {
			t.Errorf("%s: overlay %v, calls %d", k, m.overlay, hook.manyCalls)
		}
	}
}

func TestOtherKeysDoNothingInTheDialog(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "d", "x", "j", "d", "space")
	if m.overlay == nil || len(libTracks(t, hook.Library)) != 6 {
		t.Error("only y and enter confirm; anything else must leave the dialog as it was")
	}
}

func TestEnterConfirmsJustLikeY(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "d", "enter")
	if m.overlay != nil || len(libTracks(t, hook.Library)) != 5 {
		t.Errorf("enter should remove the track: overlay %v, %d tracks left", m.overlay, len(libTracks(t, hook.Library)))
	}
	contains(t, m, "Removed 1 track")
}

func TestYRemovesTheTrackAndUndoBringsItBack(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	before := trackNamed(t, hook.Library, "Flow")
	m = keys(t, m, "d", "y")
	if len(libTracks(t, hook.Library)) != 5 {
		t.Fatalf("%d tracks left", len(libTracks(t, hook.Library)))
	}
	if m.overlay != nil {
		t.Error("the dialog should close")
	}
	contains(t, m, "Removed 1 track", "5 tracks")
	lacks(t, m, "07 Flow")
	if r, ok := m.selectedRow(); !ok || r.Kind != kindTrack {
		t.Errorf("the cursor should land on a neighbouring row, got %+v", r)
	}

	m = keys(t, m, "u")
	got := trackNamed(t, hook.Library, "Flow")
	if got.ID != before.ID || got.OriginalArtist != before.OriginalArtist || got.TrackNumber != 7 {
		t.Errorf("restored %+v, want %+v", got, before)
	}
	contains(t, m, "Undid", "6 tracks", "07 Flow")
	if r, _ := m.selectedRow(); r.Label != "Flow" {
		t.Errorf("the restored track should be selected, got %q", r.Label)
	}

	m = keys(t, m, "ctrl+r")
	if len(libTracks(t, hook.Library)) != 5 {
		t.Error("redo did not remove it again")
	}
}

func TestDeleteKeyWorksToo(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = send(t, m, tea.KeyMsg{Type: tea.KeyDelete})
	if m.overlay == nil {
		t.Error("delete should ask to remove")
	}
}

func TestRemovingMarkedTracks(t *testing.T) {
	m, hook := editingModel(t, "The Wanderer")
	m = keys(t, m, "space", "space") // The Wanderer, Flow
	m = keys(t, m, "d")
	contains(t, m, "Remove 2 marked tracks?", "The Wanderer", "Flow")
	m = keys(t, m, "y")
	if left := libTracks(t, hook.Library); len(left) != 4 {
		t.Fatalf("%d left", len(left))
	}
	if len(m.marks) != 0 {
		t.Errorf("marks of removed tracks must go: %v", m.marks)
	}
	m = keys(t, m, "u")
	if len(libTracks(t, hook.Library)) != 6 {
		t.Error("one undo should bring both back")
	}
}

func TestRemovingAnAlbumFromItsRow(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "left", "d")
	contains(t, m, `Remove the 3 tracks of "Shadows"?`)
	m = keys(t, m, "y")
	if left := libTracks(t, hook.Library); len(left) != 3 {
		t.Errorf("%d left", len(left))
	}
	lacks(t, m, "Shadows")
}

func TestRemovingAnArtistFromItsRow(t *testing.T) {
	m, hook := editingModel(t, "Current")
	m = keys(t, m, "left", "left", "left", "d")
	contains(t, m, `Remove "Phoria"`)
	m = keys(t, m, "y")
	if left := libTracks(t, hook.Library); len(left) != 5 {
		t.Errorf("%d left", len(left))
	}
}

func TestRemovingEverythingThatNeedsWork(t *testing.T) {
	m, hook := browserModel2(t)
	m = keys(t, m, "d")
	contains(t, m, "Needs work")
	m = keys(t, m, "y")
	for _, tr := range libTracks(t, hook.Library) {
		if tr.Status != "identified" {
			t.Errorf("%+v is still there", tr)
		}
	}
}

func browserModel2(t *testing.T) (Model, *editHook) {
	t.Helper()
	m, lib := browserModel(t, 120, 36)
	hook := &editHook{Library: lib}
	m.lib = hook
	return m, hook // the cursor starts on the Needs work group
}

func TestUndoingARemovalAfterASyncExplainsWhyItCannot(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = keys(t, m, "d", "y")

	a := appOf(t, hook.Library)
	plan, err := a.PlanSync(app.SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(); err != nil { // collects the removed track's audio
		t.Fatal(err)
	}
	m = keys(t, m, "u")
	contains(t, m, "no longer stored")
	if len(libTracks(t, hook.Library)) != 5 {
		t.Error("a failed undo changed the library")
	}
}

func TestRemoveNeedsSomethingSelected(t *testing.T) {
	a := testApp(t)
	m := newTestModel(t, NewLibrary(a), 120, 36)
	m = keys(t, m, "d")
	if m.overlay != nil {
		t.Error("a dialog opened with nothing to remove")
	}
}
