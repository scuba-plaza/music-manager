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

func TestPreviewTrimPlaysAWindowAtACutAndNeverCutAudio(t *testing.T) {
	a := libWithTracks(t, 400) // about 10.4s
	tr := tracks(t, a)[0]
	for _, c := range []struct {
		name               string
		trimStart, trimEnd time.Duration
		edge               app.Edge
		window, want       time.Duration
	}{
		{"after the start cut", 2 * time.Second, time.Second, app.EdgeStart, 3 * time.Second, 3 * time.Second},
		{"before the end cut", 2 * time.Second, time.Second, app.EdgeEnd, 3 * time.Second, 3 * time.Second},
		{"at the end of a track without cuts", 0, 0, app.EdgeEnd, 2 * time.Second, 2 * time.Second},
		{"all that is kept after the start cut", 2 * time.Second, time.Second, app.EdgeStart, time.Minute, 7449 * time.Millisecond},
		{"all that is kept before the end cut", 2 * time.Second, time.Second, app.EdgeEnd, time.Minute, 7449 * time.Millisecond},
	} {
		if _, err := a.Edit(tr.ID, app.Edit{TrimStart: &c.trimStart, TrimEnd: &c.trimEnd}); err != nil {
			t.Fatal(err)
		}
		path, cleanup, err := a.PreviewTrim(tr.ID, c.edge, c.window)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := previewLength(t, path); !near(got, c.want) {
			t.Errorf("%s: the preview is %v, want about %v", c.name, got, c.want)
		}
		cleanup()
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s: cleanup left %s behind", c.name, path)
		}
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
