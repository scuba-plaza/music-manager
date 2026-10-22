package app_test

import (
	"os"
	"strings"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/testmp3"
)

func problemsWith(ps []app.Problem, sev app.Severity, substr string) []app.Problem {
	var out []app.Problem
	for _, p := range ps {
		if p.Severity == sev && strings.Contains(p.Message, substr) {
			out = append(out, p)
		}
	}
	return out
}

func TestDoctorOnAHealthyLibrary(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	for i, tr := range tracks(t, a) {
		identify(t, a, tr.ID, "Song", "Band", "Album", i+1, 2)
	}
	a.SetCover(tracks(t, a)[0].ID, testmp3.JPEG)
	ps, err := a.Doctor()
	if err != nil || len(ps) != 0 {
		t.Errorf("problems = %+v, err %v", ps, err)
	}
}

func TestDoctorFindsDamagedStores(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30)
	ts := tracks(t, a)
	for _, tr := range ts {
		identify(t, a, tr.ID, "Song "+tr.ID, "Band", "", 0, 0)
	}
	a.SetCover(ts[2].ID, testmp3.JPEG)

	os.Remove(a.AudioPath(ts[0].AudioSHA256))
	os.WriteFile(a.AudioPath(ts[1].AudioSHA256), []byte("bit rot"), 0o644)
	m, _ := a.Manifest()
	os.Remove(a.CoverPath(m.Tracks[2].CoverSHA256))

	ps, err := a.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	if got := problemsWith(ps, app.Error, "missing"); len(got) != 2 { // one audio, one cover
		t.Errorf("missing: %+v", ps)
	}
	if got := problemsWith(ps, app.Error, "does not match"); len(got) != 1 || got[0].TrackID != ts[1].ID {
		t.Errorf("corrupt: %+v", ps)
	}
	for _, p := range ps {
		if p.TrackID == "" {
			t.Errorf("problem without a track: %+v", p)
		}
	}
}

func TestDoctorWarnsAboutUnidentifiedTracksAndClashingPositions(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "One", "Band", "Album", 1, 2)
	identify(t, a, ts[1].ID, "Two", "Band", "Album", 1, 2) // same position as One
	// ts[2] is left unidentified
	ps, _ := a.Doctor()
	if got := problemsWith(ps, app.Warning, "not identified"); len(got) != 1 || got[0].TrackID != ts[2].ID {
		t.Errorf("unidentified: %+v", ps)
	}
	if got := problemsWith(ps, app.Warning, "same position"); len(got) != 2 {
		t.Errorf("clashing positions should be reported for both tracks: %+v", ps)
	}
	if got := problemsWith(ps, app.Error, ""); len(got) != 0 {
		t.Errorf("unexpected errors: %+v", got)
	}
}
