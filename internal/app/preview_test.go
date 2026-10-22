package app_test

import (
	"os"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/mp3"
)

func previewLength(t *testing.T, path string) time.Duration {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, d, err := mp3.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func near(got, want time.Duration) bool {
	d := got - want
	return d > -30*time.Millisecond && d < 30*time.Millisecond
}

func TestPreviewTrimPlaysTheStretchAfterTheStartCut(t *testing.T) {
	a := libWithTracks(t, 400) // about 10.4s
	tr := tracks(t, a)[0]
	if _, err := a.Edit(tr.ID, app.Edit{TrimStart: ptr(2 * time.Second), TrimEnd: ptr(time.Second)}); err != nil {
		t.Fatal(err)
	}
	path, cleanup, err := a.PreviewTrim(tr.ID, app.EdgeStart, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got := previewLength(t, path); !near(got, 3*time.Second) {
		t.Errorf("start preview is %v, want about 3s", got)
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("cleanup left %s behind", path)
	}
}

func TestPreviewTrimPlaysTheStretchBeforeTheEndCut(t *testing.T) {
	a := libWithTracks(t, 400)
	tr := tracks(t, a)[0]
	a.Edit(tr.ID, app.Edit{TrimStart: ptr(2 * time.Second), TrimEnd: ptr(time.Second)})
	path, cleanup, err := a.PreviewTrim(tr.ID, app.EdgeEnd, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if got := previewLength(t, path); !near(got, 3*time.Second) {
		t.Errorf("end preview is %v, want about 3s", got)
	}
}

func TestPreviewTrimNeverPlaysCutAudioOrMoreThanThereIs(t *testing.T) {
	a := libWithTracks(t, 400) // about 10.4s; the kept part is 2s..9.4s, 7.4s long
	tr := tracks(t, a)[0]
	a.Edit(tr.ID, app.Edit{TrimStart: ptr(2 * time.Second), TrimEnd: ptr(time.Second)})
	for _, edge := range []app.Edge{app.EdgeStart, app.EdgeEnd} {
		path, cleanup, err := a.PreviewTrim(tr.ID, edge, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if got := previewLength(t, path); !near(got, 7449*time.Millisecond) {
			t.Errorf("edge %d: %v, want the whole kept part (about 7.4s)", edge, got)
		}
		cleanup()
	}
}

func TestPreviewTrimWithoutTrimsPlaysTheEdgesOfTheTrack(t *testing.T) {
	a := libWithTracks(t, 400)
	tr := tracks(t, a)[0]
	path, cleanup, err := a.PreviewTrim(tr.ID, app.EdgeEnd, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if got := previewLength(t, path); !near(got, 2*time.Second) {
		t.Errorf("= %v", got)
	}
}

func TestPreviewTrimErrors(t *testing.T) {
	a := libWithTracks(t, 100)
	if _, _, err := a.PreviewTrim("nope", app.EdgeStart, time.Second); err == nil {
		t.Error("unknown id accepted")
	}
	tr := tracks(t, a)[0]
	if _, _, err := a.PreviewTrim(tr.ID, app.EdgeStart, 0); err == nil {
		t.Error("an empty window was accepted")
	}
}
