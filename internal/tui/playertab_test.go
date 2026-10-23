package tui

import "testing"

// The Player tab shows an artist and an album as the player will, which keeps
// 12 characters of each.

func namesModel(t *testing.T) (Model, *editHook) {
	t.Helper()
	a := testApp(t)
	seed(t, a,
		seedTrack{"Bahlam Bek", "Abdel Halim Hafez", "Abd El Halim Hafez", 1, 2, 0},
		seedTrack{"Dream of Me and You", "Max Richter", "", 0, 0, 0},
		seedTrack{"Flow", "Shawn James", "Shadows", 7, 12, 0},
	)
	lib := NewLibrary(a)
	hook := &editHook{Library: lib}
	m := newTestModel(t, lib, 120, 36)
	m.lib = hook
	return selectTrack(t, m, "Bahlam Bek"), hook
}

func TestThePlayerTabShowsWhatThePlayerWillShowForLongNames(t *testing.T) {
	m, _ := namesModel(t)
	m = keys(t, m, "right", "3")
	contains(t, m, "Player", "Abdel Halim", "Abd El Halim", `from "Abdel Halim Hafez"`, `from "Abd El Halim Hafez"`, "12 characters")
	lacks(t, m, "Released", "Cut start")
}

func TestThePlayerTabHasNothingToAddForNamesThatFit(t *testing.T) {
	m, _ := namesModel(t)
	m = selectTrack(t, m, "Flow")
	m = keys(t, m, "right", "3")
	contains(t, m, "Shawn James", "Shadows")
	lacks(t, m, "from", "your own")
}

func TestAnAlbumlessTrackShowsItsTitleAsTheAlbum(t *testing.T) {
	m, _ := namesModel(t)
	m = selectTrack(t, m, "Dream of Me and You")
	m = keys(t, m, "right", "3")
	contains(t, m, "Max Richter", "Dream of Me", "a single")
}

func TestEditingAPlayerNameCountsTheCharactersThePlayerKeeps(t *testing.T) {
	m, hook := namesModel(t)
	m = focusField(t, m, fPlayerArtist)
	m = keys(t, m, "enter")
	contains(t, m, "Abdel Halim", "11 of 12")
	m = typed(t, m, " H.")
	contains(t, m, "14 of 12", "cuts")
	m = keys(t, m, "esc")
	if hook.calls != 0 {
		t.Errorf("cancelling saved something: %d calls", hook.calls)
	}
}

func TestAPlayerNameThatDiffersFromTheAutomaticOneIsSavedAndMarkedAsOwn(t *testing.T) {
	m, hook := namesModel(t)
	m = focusField(t, m, fPlayerArtist)
	m = typed(t, keys(t, m, "enter"), " H.")
	m = keys(t, m, "enter")
	if got := trackNamed(t, hook.Library, "Bahlam Bek"); got.PlayerArtist != "Abdel Halim H." {
		t.Errorf("saved %q", got.PlayerArtist)
	}
	contains(t, m, "Abdel Halim H.", "your own", "cuts")

	// Clearing it goes back to the automatic name.
	m = keys(t, m, "enter")
	m = clearField(t, m)
	m = keys(t, m, "enter")
	if got := trackNamed(t, hook.Library, "Bahlam Bek"); got.PlayerArtist != "" {
		t.Errorf("an empty name should mean the automatic one: %q", got.PlayerArtist)
	}
	lacks(t, m, "your own")
}

func TestSavingTheAutomaticNameUnchangedKeepsItAutomatic(t *testing.T) {
	m, hook := namesModel(t)
	m = focusField(t, m, fPlayerAlbum)
	m = keys(t, m, "enter", "enter")
	if got := trackNamed(t, hook.Library, "Bahlam Bek"); got.PlayerAlbum != "" {
		t.Errorf("the automatic name was stored as an override: %q", got.PlayerAlbum)
	}
}
