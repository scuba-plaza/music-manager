package mp3_test

import (
	"bytes"
	"testing"
	"time"

	"music-manager/internal/mp3"
	"music-manager/internal/testmp3"
)

// The loudness of a frame is in its side information: each granule has a
// global_gain of 8 bits, and one step more is a fourth of an octave of
// amplitude (1.505 dB). Changing it changes nothing else, so nothing is
// decoded or encoded again.

// ramp is n granules of channels channels, each at base plus its place.
func ramp(n, channels, base int) [][]int {
	out := make([][]int, n)
	for g := range out {
		out[g] = make([]int, channels)
		for c := range out[g] {
			out[g][c] = base + g + 3*c
		}
	}
	return out
}

func gainsOf(t *testing.T, audio []byte) [][]int {
	t.Helper()
	got, err := mp3.GlobalGains(audio)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestGainMovesEveryGranuleByWholeSteps(t *testing.T) {
	before := ramp(20, 2, 100)
	audio := testmp3.WithGains(testmp3.Layout{}, before)
	original := bytes.Clone(audio)

	out, err := mp3.Adjust(audio, mp3.Shape{Steps: 4})
	if err != nil {
		t.Fatal(err)
	}
	got := gainsOf(t, out)
	if len(got) != len(before) {
		t.Fatalf("%d granules, want %d", len(got), len(before))
	}
	for g := range before {
		for c := range before[g] {
			if got[g][c] != before[g][c]+4 {
				t.Errorf("granule %d channel %d: gain %d, want %d", g, c, got[g][c], before[g][c]+4)
			}
		}
	}

	if !bytes.Equal(audio, original) {
		t.Error("the audio that was given was changed")
	}
	if len(out) != len(audio) {
		t.Errorf("the length changed from %d to %d", len(audio), len(out))
	}
	// Nothing but the gains moved: undoing the steps gives the audio back.
	back, err := mp3.Adjust(out, mp3.Shape{Steps: -4})
	if err != nil || !bytes.Equal(back, audio) {
		t.Errorf("undoing the gain did not give the audio back (err %v)", err)
	}
}

func TestGainStopsAtTheEndsOfTheRangeInsteadOfWrapping(t *testing.T) {
	audio := testmp3.WithGains(testmp3.Layout{}, [][]int{{253, 255}, {2, 0}, {128, 128}, {250, 5}})
	up, err := mp3.Adjust(audio, mp3.Shape{Steps: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := gainsOf(t, up), [][]int{{255, 255}, {12, 10}, {138, 138}, {255, 15}}; !equalGains(got, want) {
		t.Errorf("up: %v, want %v", got, want)
	}
	down, err := mp3.Adjust(audio, mp3.Shape{Steps: -10})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := gainsOf(t, down), [][]int{{243, 245}, {0, 0}, {118, 118}, {240, 0}}; !equalGains(got, want) {
		t.Errorf("down: %v, want %v", got, want)
	}
}

func equalGains(a, b [][]int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

func TestGainWorksOnEveryLayoutOfFrame(t *testing.T) {
	for name, l := range map[string]testmp3.Layout{
		"MPEG-1 stereo": {},
		"MPEG-1 mono":   {Mono: true},
		"MPEG-2 stereo": {MPEG2: true},
		"MPEG-2 mono":   {MPEG2: true, Mono: true},
	} {
		t.Run(name, func(t *testing.T) {
			before := ramp(7*l.Granules(), l.Channels(), 90)
			audio := testmp3.WithGains(l, before)
			out, err := mp3.Adjust(audio, mp3.Shape{Steps: -3})
			if err != nil {
				t.Fatal(err)
			}
			want := make([][]int, len(before))
			for g := range before {
				want[g] = make([]int, len(before[g]))
				for c := range before[g] {
					want[g][c] = before[g][c] - 3
				}
			}
			if got := gainsOf(t, out); !equalGains(got, want) {
				t.Errorf("gains %v, want %v", got, want)
			}
			if back, err := mp3.Adjust(out, mp3.Shape{Steps: 3}); err != nil || !bytes.Equal(back, audio) {
				t.Errorf("something besides the gains changed (err %v)", err)
			}
		})
	}
}

func TestAProtectedFrameKeepsACrcThatFitsItsChangedSideInformation(t *testing.T) {
	for name, l := range map[string]testmp3.Layout{
		"MPEG-1 stereo": {CRC: true},
		"MPEG-2 mono":   {CRC: true, MPEG2: true, Mono: true},
	} {
		t.Run(name, func(t *testing.T) {
			audio := testmp3.WithGains(l, ramp(6*l.Granules(), l.Channels(), 120))
			out, err := mp3.Adjust(audio, mp3.Shape{Steps: 5})
			if err != nil {
				t.Fatal(err)
			}
			if want := ramp(6*l.Granules(), l.Channels(), 125); !equalGains(gainsOf(t, out), want) {
				t.Fatalf("gains %v, want %v", gainsOf(t, out), want)
			}
			size := len(out) / 6
			sideLen := map[testmp3.Layout]int{{CRC: true}: 32, {CRC: true, MPEG2: true, Mono: true}: 9}[l]
			for i := 0; i < 6; i++ {
				frame := out[i*size : (i+1)*size]
				stored := uint16(frame[4])<<8 | uint16(frame[5])
				if want := testmp3.GainCRC(frame[2:4], frame[6:6+sideLen]); stored != want {
					t.Errorf("frame %d: CRC %04x, want %04x", i, stored, want)
				}
			}
			// The frames themselves are not the ones that were given.
			if bytes.Equal(out, audio) {
				t.Error("nothing changed")
			}
		})
	}
}

// A fade is a ramp of amplitude from silence to the full volume, or back,
// across a stretch of the track: each granule gets the steps that bring its
// amplitude to the share of the fade it is at (half way: -4 steps, a half).
// Before a fade-in begins and after a fade-out ends there is silence. A
// granule is 576 samples, 13.06 ms at 44.1 kHz, and is placed at its middle.

// flat is n granules of channels channels, all at gain.
func flat(n, channels, gain int) [][]int {
	out := make([][]int, n)
	for g := range out {
		out[g] = make([]int, channels)
		for c := range out[g] {
			out[g][c] = gain
		}
	}
	return out
}

// deltas are, for each granule, how far its first channel is from base.
func deltas(t *testing.T, audio []byte, base int) []int {
	t.Helper()
	var out []int
	for _, row := range gainsOf(t, audio) {
		out = append(out, row[0]-base)
	}
	return out
}

func TestAFadeInRaisesTheVolumeFromSilenceToFullAcrossItsSpan(t *testing.T) {
	audio := testmp3.WithGains(testmp3.Layout{}, flat(200, 2, 150)) // 2.6 s
	out, err := mp3.Adjust(audio, mp3.Shape{FadeIn: mp3.Fade{Start: 0, End: time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	d := deltas(t, out, 150)
	for g := 1; g < len(d); g++ {
		if d[g] < d[g-1] {
			t.Fatalf("granule %d is quieter (%d) than the one before (%d)", g, d[g], d[g-1])
		}
	}
	for g, want := range map[int]int{
		0:  -29,        // 6.5 ms in
		18: -8, 19: -8, // a quarter of the way: amplitude 1/4
		37: -4, 38: -4, // half way: amplitude 1/2
		56: -2, 57: -2, // three quarters
		77: 0, 100: 0, 199: 0, // after the fade
	} {
		if d[g] != want {
			t.Errorf("granule %d: %d steps from the full volume, want %d", g, d[g], want)
		}
	}
}

func TestAFadeOutLowersTheVolumeToSilenceAndStaysSilent(t *testing.T) {
	audio := testmp3.WithGains(testmp3.Layout{}, flat(200, 2, 150))
	out, err := mp3.Adjust(audio, mp3.Shape{FadeOut: mp3.Fade{Start: time.Second, End: 2 * time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	d := deltas(t, out, 150)
	for g := 1; g < len(d); g++ {
		if d[g] > d[g-1] {
			t.Fatalf("granule %d is louder (%d) than the one before (%d)", g, d[g], d[g-1])
		}
	}
	for g, want := range map[int]int{0: 0, 70: 0, 76: 0, 114: -4, 115: -4, 199: -150} { // 199: silent
		if d[g] != want {
			t.Errorf("granule %d: %d steps from the full volume, want %d", g, d[g], want)
		}
	}
}

func TestAFadeInThatStartsLateLeavesSilenceBeforeIt(t *testing.T) {
	audio := testmp3.WithGains(testmp3.Layout{}, flat(200, 2, 150))
	out, err := mp3.Adjust(audio, mp3.Shape{FadeIn: mp3.Fade{Start: time.Second, End: 2 * time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	d := deltas(t, out, 150)
	if d[0] != -150 || d[70] != -150 {
		t.Errorf("before the fade: %d and %d, want silence (-150)", d[0], d[70])
	}
	if d[114] != -4 || d[160] != 0 {
		t.Errorf("granule 114: %d, want -4; granule 160: %d, want 0", d[114], d[160])
	}
}

func TestTheFirstStretchOfALongFadeIsSilentRatherThanAnInaudibleWhisper(t *testing.T) {
	audio := testmp3.WithGains(testmp3.Layout{}, flat(40, 2, 150))
	out, err := mp3.Adjust(audio, mp3.Shape{FadeIn: mp3.Fade{Start: 0, End: 100 * time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	d := deltas(t, out, 150)
	if d[0] != -150 || d[3] != -150 {
		t.Errorf("the first granules: %d and %d, want silence", d[0], d[3])
	}
	if d[20] != -34 { // 0.27 s of 100: 1/373 of the amplitude, 8.5 octaves down
		t.Errorf("granule 20: %d, want -34", d[20])
	}
}

func TestGainAndFadesAddUp(t *testing.T) {
	audio := testmp3.WithGains(testmp3.Layout{}, flat(200, 2, 150))
	out, err := mp3.Adjust(audio, mp3.Shape{Steps: 3, FadeIn: mp3.Fade{End: time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	if d := deltas(t, out, 150); d[37] != -1 || d[150] != 3 {
		t.Errorf("granule 37: %d, want -1 (3 - 4); granule 150: %d, want 3", d[37], d[150])
	}
}

func TestFadesAreInSecondsWhateverTheLayoutOfTheFrames(t *testing.T) {
	// MPEG-2 at 22.05 kHz: a granule is 26.1 ms and a frame has one.
	audio := testmp3.WithGains(testmp3.Layout{MPEG2: true, Mono: true}, flat(100, 1, 150))
	out, err := mp3.Adjust(audio, mp3.Shape{FadeIn: mp3.Fade{End: time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	if d := deltas(t, out, 150); d[18] != -4 || d[19] != -4 || d[40] != 0 {
		t.Errorf("granules 18, 19 and 40: %d, %d and %d; want -4, -4 and 0", d[18], d[19], d[40])
	}
}
