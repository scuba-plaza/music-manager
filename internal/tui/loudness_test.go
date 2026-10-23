package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/mp3"
)

// The Audio tab holds the volume and the fades beside the cuts.

func TestTheAudioTabListsTheVolumeAndTheFades(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = keys(t, m, "right", "2")
	contains(t, m, "Cut start", "Cut end", "Volume", "0 dB", "Fade in", "Fade out")
	lacks(t, m, "Released")
}

func TestEditingTheVolumeInDecibels(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = focusField(t, m, fVolume)
	m = keys(t, m, "enter")
	contains(t, m, "dB", "1.5") // what is allowed is explained
	m = typed(t, m, "+3")
	m = keys(t, m, "enter")
	if got := trackNamed(t, hook.Library, "Flow"); got.GainSteps != 2 {
		t.Errorf("saved %d steps, want 2", got.GainSteps)
	}
	contains(t, m, "Saved", "+3.0 dB")

	// 2 dB is not a step: the nearest one is shown.
	m = keys(t, m, "enter")
	m = clearField(t, m)
	m = typed(t, m, "2")
	m = keys(t, m, "enter")
	contains(t, m, "+1.5 dB")

	m = keys(t, m, "enter")
	m = clearField(t, m)
	m = keys(t, m, "enter")
	if got := trackNamed(t, hook.Library, "Flow"); got.GainSteps != 0 {
		t.Errorf("an empty volume should mean no change: %d", got.GainSteps)
	}
}

func TestAVolumeThatIsNotANumberOrIsTooMuchIsExplainedAndNothingIsSaved(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = focusField(t, m, fVolume)
	m = typed(t, keys(t, m, "enter"), "loud")
	m = keys(t, m, "enter")
	if !m.editing || hook.calls != 0 {
		t.Errorf("editing %v, library calls %d", m.editing, hook.calls)
	}
	contains(t, m, "is not a number of dB")

	m = clearField(t, m)
	m = typed(t, m, "40")
	m = keys(t, m, "enter")
	if !m.editing {
		t.Error("a volume of 40 dB closed the editor")
	}
	contains(t, m, "24 dB")
}

func TestEditingAFadeAsALengthFromTheEdgeOfTheCut(t *testing.T) {
	m, hook := editingModel(t, "Flow") // about 2.6 s
	m = focusField(t, m, fFadeIn)
	m = keys(t, m, "enter")
	contains(t, m, "0:02-0:05") // the forms it takes are explained
	m = typed(t, m, "1")
	m = keys(t, m, "enter")
	got := trackNamed(t, hook.Library, "Flow")
	if got.FadeIn != (manifest.Span{End: time.Second}) {
		t.Errorf("fade-in %+v, want the first second", got.FadeIn)
	}
	contains(t, m, "Saved")

	m = keys(t, m, "down") // to Fade out
	m = typed(t, keys(t, m, "enter"), "0:00.5")
	m = keys(t, m, "enter")
	got = trackNamed(t, hook.Library, "Flow")
	if want := got.SourceLength; got.FadeOut != (manifest.Span{Start: want - 500*time.Millisecond, End: want}) {
		t.Errorf("fade-out %+v, want the last half second of %v", got.FadeOut, want)
	}
}

func TestABadFadeIsExplainedAndNothingIsSaved(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = focusField(t, m, fFadeIn)
	m = typed(t, keys(t, m, "enter"), "soon")
	m = keys(t, m, "enter")
	if !m.editing || hook.calls != 0 {
		t.Errorf("editing %v, library calls %d", m.editing, hook.calls)
	}
	contains(t, m, "invalid time")

	m = clearField(t, m)
	m = typed(t, m, "1:00") // longer than the track
	m = keys(t, m, "enter")
	if !m.editing {
		t.Error("a fade longer than the track closed the editor")
	}
	contains(t, m, "fade-in")
}

func TestNudgeKeysMoveTheVolumeByWholeSteps(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	steps := func() int { return trackNamed(t, hook.Library, "Flow").GainSteps }
	m = focusField(t, m, fVolume)
	for _, c := range []struct {
		keys string
		want int
	}{{"]", 1}, {"]", 2}, {"[", 1}, {"}", 5}, {"{", 1}, {"{", -3}, {"0", 0}} {
		m = keys(t, m, c.keys)
		if got := steps(); got != c.want {
			t.Fatalf("after %q: %d steps, want %d", c.keys, got, c.want)
		}
	}
	contains(t, m, "0 dB")

	// It stops at 24 dB either way, and a run of nudges is one undo.
	for i := 0; i < 6; i++ {
		m = keys(t, m, "}")
	}
	if got := steps(); got != 16 {
		t.Errorf("after many nudges up: %d steps, want the limit, 16", got)
	}
	contains(t, m, "+24.1 dB")
	m = keys(t, m, "u")
	if got := steps(); got != 0 {
		t.Errorf("one undo left %d steps, want the run undone", got)
	}
}

func TestNudgeKeysLengthenAndShortenAFade(t *testing.T) {
	m, hook := editingModel(t, "Flow") // about 2.6 s
	flow := func() manifest.Track { return trackNamed(t, hook.Library, "Flow") }

	m = focusField(t, m, fFadeIn)
	m = keys(t, m, "]") // a new fade-in, 0.1 s, from where the track starts
	if got := flow().FadeIn; got != (manifest.Span{End: 100 * time.Millisecond}) {
		t.Fatalf("after one nudge: %+v", got)
	}
	m = keys(t, m, "}", "}")
	if got := flow().FadeIn; got != (manifest.Span{End: 2100 * time.Millisecond}) {
		t.Errorf("after two big nudges: %+v", got)
	}
	m = keys(t, m, "{")
	if got := flow().FadeIn; got != (manifest.Span{End: 1100 * time.Millisecond}) {
		t.Errorf("after shortening: %+v", got)
	}
	m = keys(t, m, "0")
	if got := flow().FadeIn; got != (manifest.Span{}) {
		t.Errorf("0 should take the fade away: %+v", got)
	}

	// A fade-out grows back from the end.
	m = keys(t, m, "down", "}")
	end := flow().SourceLength
	if got := flow().FadeOut; got != (manifest.Span{Start: end - time.Second, End: end}) {
		t.Errorf("fade-out %+v, want the last second of %v", got, end)
	}
	m = keys(t, m, "[")
	if got := flow().FadeOut; got != (manifest.Span{Start: end - 900*time.Millisecond, End: end}) {
		t.Errorf("fade-out %+v after shortening", got)
	}
}

func TestANudgeThatWouldCrossTheOtherFadeSaysSoAndChangesNothing(t *testing.T) {
	m, hook := editingModel(t, "Flow")
	m = focusField(t, m, fFadeIn)
	m = keys(t, m, "}")         // 1 s from the start
	m = keys(t, m, "down", "}") // 1 s back from the end of 2.6: fine
	m = keys(t, m, "}")         // 2 s back from the end: crosses the fade-in
	contains(t, m, "must not start before")
	if got := trackNamed(t, hook.Library, "Flow").FadeOut.End - trackNamed(t, hook.Library, "Flow").FadeOut.Start; got > time.Second+time.Millisecond {
		t.Errorf("the fade-out grew to %v anyway", got)
	}
}

func TestPreviewOnTheVolumeAndTheFadesPlaysWhatSyncWillWrite(t *testing.T) {
	p := newFakePlayer()
	m, hook := playerModel(t, "Flow", p)
	tr := trackNamed(t, hook.Library, "Flow")
	db := 10 * 1.5051499783199058 // ten steps
	if _, err := hook.Library.Edit(tr.ID, app.Edit{GainDB: &db, FadeOut: &manifest.Span{Start: tr.SourceLength - time.Second, End: tr.SourceLength}}); err != nil {
		t.Fatal(err)
	}
	m = send(t, m, m.load())

	gains := func(path string) []int {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		audio, _, err := mp3.Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := mp3.GlobalGains(audio)
		if err != nil {
			t.Fatal(err)
		}
		var out []int
		for _, r := range rows {
			out = append(out, r[0])
		}
		return out
	}

	m = focusField(t, m, fFadeOut)
	m = keys(t, m, "p")
	if len(p.played) != 1 {
		t.Fatalf("played %v", p.played)
	}
	g := gains(p.played[0])
	if g[0] != 10 || g[len(g)-1] != 0 { // the gain at first, and silence at the end of the fade-out
		t.Errorf("the preview of the fade-out went from gain %d to %d, want 10 to 0", g[0], g[len(g)-1])
	}
	m = keys(t, m, "p") // stop

	m = keys(t, m, "up", "up", "p") // on Volume
	if len(p.played) != 2 {
		t.Fatalf("played %v", p.played)
	}
	if g := gains(p.played[1]); g[0] != 10 {
		t.Errorf("the preview for the volume starts at gain %d, want 10", g[0])
	}
}

func TestTheFooterOffersTheNudgeKeysOnTheVolumeAndTheFades(t *testing.T) {
	m, _ := editingModel(t, "Flow")
	m = focusField(t, m, fVolume)
	contains(t, m, "nudge", "hear")
	m = keys(t, m, "down")
	contains(t, m, "nudge", "hear")
}

// levelOf is the row of blocks that the Level line of the Audio tab shows.
func levelOf(t *testing.T, m Model) []rune {
	t.Helper()
	for _, line := range strings.Split(screen(m), "\n") {
		if _, after, ok := strings.Cut(line, "Level"); ok {
			for _, part := range strings.Split(after, "│") { // the right edge of the pane
				if strings.ContainsAny(part, "▁▂▃▄▅▆▇█░") {
					return []rune(strings.TrimSpace(part))
				}
			}
		}
	}
	return nil
}

func TestTheFadesAreDrawnAsRampsOnTheAudioTab(t *testing.T) {
	m, hook := editingModel(t, "Flow") // about 2.6 s
	m = keys(t, m, "right", "2")
	lacks(t, m, "▁") // no line of blocks without a fade

	tr := trackNamed(t, hook.Library, "Flow")
	if _, err := hook.Library.Edit(tr.ID, app.Edit{
		TrimStart: ptr(500 * time.Millisecond),
		FadeIn:    &manifest.Span{Start: 500 * time.Millisecond, End: 1500 * time.Millisecond},
		FadeOut:   &manifest.Span{Start: tr.SourceLength - time.Second, End: tr.SourceLength},
	}); err != nil {
		t.Fatal(err)
	}
	m = send(t, m, m.load())
	cells := levelOf(t, m)
	if len(cells) < 20 {
		t.Fatalf("no level line (%q)", string(cells))
	}
	if cells[0] != '░' { // the cut-off start
		t.Errorf("the level starts with %q, want the cut-off mark", cells[0])
	}
	rank := func(r rune) int { return strings.IndexRune("▁▂▃▄▅▆▇█", r) }
	var kept []rune
	for _, r := range cells {
		if r != '░' {
			kept = append(kept, r)
		}
	}
	peak := 0
	for i, r := range kept {
		if rank(r) < 0 {
			t.Fatalf("unexpected %q in %q", r, string(kept))
		}
		if rank(r) > rank(kept[peak]) {
			peak = i
		}
	}
	if kept[0] != '▁' || kept[peak] != '█' || kept[len(kept)-1] != '▁' {
		t.Errorf("the ramps should start low, reach full and end low: %q", string(kept))
	}
	for i := 1; i <= peak; i++ {
		if rank(kept[i]) < rank(kept[i-1]) {
			t.Errorf("the fade-in is not a rising ramp: %q", string(kept))
			break
		}
	}
}
