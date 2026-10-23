package tui

import (
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

// Under --wolfgang the inspector has a fourth tab, a read-only preview of what
// a sync writes for the Wolfgang WMP01. Without it there is no such tab.

func wolfgangModel(t *testing.T, title string) Model {
	t.Helper()
	m, _ := editingModel(t, title)
	WithWolfgang()(&m)
	return m
}

func TestThereIsNoFourthTabUnlessAsked(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "right", "4")
	lacks(t, m, "Wolfgang")
	contains(t, m, "1-3 tabs", "Released") // still on the details
}

func TestTheWolfgangTabShowsWhatTheCardWillGet(t *testing.T) {
	m := wolfgangModel(t, "Flow")
	m = keys(t, m, "right")
	contains(t, m, "1-4 tabs", "Wolfgang")
	m = keys(t, m, "4")
	contains(t, m, "07 - Flow.mp3", "Shawn James/Shadows", "Shawn James", "Shadows")
	lacks(t, m, "Released", "Cut start")
}

func TestTheWolfgangTabShowsTheNamesTheWolfgangWillShow(t *testing.T) {
	m := wolfgangModel(t, "Flow")
	a := appOf(t, m.lib.(*editHook).Library)
	tr := selectedTrack(t, m)
	if _, err := a.Edit(tr.ID, app.Edit{Artist: ptr("Abdel Halim Hafez"), Album: ptr("Abd El Halim Hafez")}); err != nil {
		t.Fatal(err)
	}
	m = send(t, m, m.Init())
	m = selectTrack(t, m, "Flow")
	m = keys(t, m, "right", "4")
	contains(t, m, "A. H. Hafez", "Abd El Halim")
}

func TestTheWolfgangTabMarksWhereTheFileNameIsCut(t *testing.T) {
	m := wolfgangModel(t, "Flow")
	a := appOf(t, m.lib.(*editHook).Library)
	tr := selectedTrack(t, m)
	if _, err := a.Edit(tr.ID, app.Edit{Title: ptr("A Rather Long Title That Goes On")}); err != nil {
		t.Fatal(err)
	}
	m = send(t, m, m.Init())
	m = selectTrack(t, m, "A Rather Long Title That Goes On")
	m = keys(t, m, "right", "4")
	contains(t, m, "07 - A Rather Long T.mp3", "24", "20") // what is kept, and the numbers the player works with
}

func TestTheWolfgangTabListsTheWarnings(t *testing.T) {
	m := wolfgangModel(t, "Flow")
	a := appOf(t, m.lib.(*editHook).Library)
	tr := selectedTrack(t, m)
	if _, err := a.Edit(tr.ID, app.Edit{Genre: ptr("Alternative Rock")}); err != nil {
		t.Fatal(err)
	}
	m = send(t, m, m.Init())
	m = selectTrack(t, m, "Flow")
	m = keys(t, m, "right", "4")
	contains(t, m, "Alternative Rock")
}

func TestTheWolfgangTabCannotBeEdited(t *testing.T) {
	m := wolfgangModel(t, "Flow")
	m = keys(t, m, "right", "4", "enter")
	if m.editing {
		t.Error("enter opened an editor on a preview")
	}
	m = keys(t, m, "down") // the last field: nothing below
	contains(t, m, "Wolfgang")
}

func TestTheSyncDialogStartsOnTheWolfgangFormatForADriveUnderWolfgang(t *testing.T) {
	m, _ := driveModel(t)
	WithWolfgang()(&m)
	m = keys(t, m, "s", "down")
	contains(t, m, "Format", "Wolfgang WMP01")
	m = keys(t, m, "w")
	lacks(t, m, "Wolfgang WMP01")
	m = keys(t, m, "up")
	contains(t, m, "Generic") // the library folder is always the standard way
}

func selectedTrack(t *testing.T, m Model) manifest.Track {
	t.Helper()
	r, ok := m.selectedRow()
	if !ok || r.Kind != kindTrack {
		t.Fatal("no track is selected")
	}
	return r.Track
}
