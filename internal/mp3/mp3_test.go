package mp3_test

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"music-manager/internal/mp3"
	"music-manager/internal/testmp3"
)

func TestDurationOfSilentFrames(t *testing.T) {
	// 100 frames * 1152 samples / 44100 Hz = 2.612244897...s
	_, d, err := mp3.Parse(testmp3.Silent(100))
	if err != nil {
		t.Fatal(err)
	}
	want := 2612 * time.Millisecond
	if diff := d - want; diff < -time.Millisecond || diff > time.Millisecond {
		t.Errorf("duration = %v, want about %v", d, want)
	}
}

func TestParseStripsTagsLeavingIdenticalAudio(t *testing.T) {
	bare := testmp3.Silent(10)
	cases := map[string][]byte{
		"no tags":        bare,
		"id3v2.3":        testmp3.WithID3v2(bare, 3, 300),
		"id3v2.4":        testmp3.WithID3v2(bare, 4, 4096),
		"id3v1":          testmp3.WithID3v1(bare),
		"ape":            testmp3.WithAPE(bare),
		"id3v2 and v1":   testmp3.WithID3v1(testmp3.WithID3v2(bare, 4, 10)),
		"ape then id3v1": testmp3.WithID3v1(testmp3.WithAPE(bare)),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			got, _, err := mp3.Parse(in)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, bare) {
				t.Errorf("audio = %d bytes, want the %d bare bytes", len(got), len(bare))
			}
		})
	}
}

func TestParseDropsTruncatedLastFrame(t *testing.T) {
	bare := testmp3.Silent(10)
	got, _, err := mp3.Parse(bare[:len(bare)-100])
	if err != nil {
		t.Fatal(err)
	}
	if want := bare[:9*testmp3.FrameSize]; !bytes.Equal(got, want) {
		t.Errorf("audio = %d bytes, want %d", len(got), len(want))
	}
}

func TestParseRejectsNonMP3(t *testing.T) {
	for name, in := range map[string][]byte{
		"empty":        nil,
		"text":         []byte("definitely not audio, just words, repeated. " + string(make([]byte, 2000))),
		"tags only":    testmp3.WithID3v2(nil, 4, 100),
		"a lone frame": testmp3.Silent(1)[:50],
	} {
		if _, _, err := mp3.Parse(in); err == nil {
			t.Errorf("%s: Parse succeeded, want error", name)
		}
	}
}

func TestParseDropsEncoderInfoFrame(t *testing.T) {
	bare := testmp3.Silent(10)
	_, wantDur, _ := mp3.Parse(bare)
	for _, marker := range []string{"Xing", "Info", "VBRI"} {
		got, dur, err := mp3.Parse(testmp3.WithInfoFrame(bare, marker))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, bare) || dur != wantDur {
			t.Errorf("%s: audio %d bytes (want %d), duration %v (want %v)", marker, len(got), len(bare), dur, wantDur)
		}
	}
}

func TestTrimCutsWholeFramesToTheNearestFrame(t *testing.T) {
	// 100 frames of about 26.12ms each: 2.612s in all.
	audio := testmp3.Silent(100)
	frames := func(from, to int) []byte { return audio[from*testmp3.FrameSize : to*testmp3.FrameSize] }
	cases := []struct {
		name       string
		start, end time.Duration
		want       []byte
	}{
		{"no trim", 0, 0, audio},
		{"start", time.Second, 0, frames(38, 100)},         // 1s = 38.28 frames
		{"end", 0, time.Second, frames(0, 62)},             // keeps up to 1.612s = 61.7 frames
		{"both", time.Second, time.Second, frames(38, 62)}, //
		{"tiny trim rounds to nothing", 10 * time.Millisecond, 10 * time.Millisecond, audio},
		{"1.49 frames rounds down to one", 39 * time.Millisecond, 0, frames(1, 100)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := mp3.Trim(audio, c.start, c.end)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, c.want) {
				t.Errorf("got %d frames, want %d", len(got)/testmp3.FrameSize, len(c.want)/testmp3.FrameSize)
			}
		})
	}
}

func TestTrimResultIsCloseToTheRequestedLength(t *testing.T) {
	audio := testmp3.Silent(400) // about 10.4s
	got, err := mp3.Trim(audio, 2500*time.Millisecond, 1250*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	_, dur, _ := mp3.Parse(got)
	want := 10449*time.Millisecond - 3750*time.Millisecond
	if diff := dur - want; diff < -27*time.Millisecond || diff > 27*time.Millisecond {
		t.Errorf("trimmed duration = %v, want %v within one frame", dur, want)
	}
}

func TestTrimRejectsImpossibleRequests(t *testing.T) {
	audio := testmp3.Silent(100)
	for name, c := range map[string][2]time.Duration{
		"negative start": {-time.Second, 0},
		"negative end":   {0, -time.Second},
		"covers it all":  {2 * time.Second, time.Second},
		"past the end":   {time.Hour, 0},
	} {
		if _, err := mp3.Trim(audio, c[0], c[1]); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestRenderWritesATagThatReadsBackExactly(t *testing.T) {
	audio := testmp3.Silent(20)
	want := mp3.TagSet{
		Title: "Wtni", Artist: "Firuz", Album: "Hits",
		Track: 3, Total: 12,
		Date:   "1999-05-17",
		Length: 523 * time.Millisecond,
		Extra: map[string]string{
			"ORIGINAL_TITLE":  "وطني",
			"ORIGINAL_ARTIST": "فيروز",
			"SOURCE_URL":      "https://youtu.be/xyz",
			"MM_ID":           "k7f2qd",
		},
		Cover: []byte("\xff\xd8 pretend jpeg \xff\xd9"), CoverMIME: "image/jpeg",
	}
	file, err := mp3.Render(audio, want)
	if err != nil {
		t.Fatal(err)
	}
	if string(file[:3]) != "ID3" || file[3] != 4 {
		t.Errorf("file starts %q v%d, want an ID3v2.4 tag", file[:3], file[3])
	}
	got := mp3.ReadTagSet(file)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("read back %+v\nwant %+v", got, want)
	}
	back, _, err := mp3.Parse(file)
	if err != nil || !bytes.Equal(back, audio) {
		t.Errorf("audio changed by rendering: %d bytes, err %v", len(back), err)
	}
}

func TestRenderIsDeterministicAndOmitsEmptyFields(t *testing.T) {
	audio := testmp3.Silent(5)
	ts := mp3.TagSet{Title: "T", Artist: "A", Extra: map[string]string{"B": "2", "A": "1", "C": "3"}}
	first, _ := mp3.Render(audio, ts)
	for i := 0; i < 20; i++ {
		again, _ := mp3.Render(audio, ts)
		if !bytes.Equal(first, again) {
			t.Fatal("two renders of the same input differ")
		}
	}
	got := mp3.ReadTagSet(first)
	if got.Album != "" || got.Track != 0 || got.Cover != nil || got.Length != 0 || got.Date != "" {
		t.Errorf("empty fields were written: %+v", got)
	}
}

func TestReadTagSetOfUntaggedData(t *testing.T) {
	if got := mp3.ReadTagSet(testmp3.Silent(3)); !reflect.DeepEqual(got, mp3.TagSet{}) {
		t.Errorf("got %+v, want zero value", got)
	}
}

func TestReadTagSetOfForeignTags(t *testing.T) {
	audio := testmp3.Silent(5)
	cases := []struct {
		name string
		in   []byte
		want mp3.TagSet
	}{
		{"all three", testmp3.WithTags(audio, "فيروز", "Max Richter", "Sleep"), mp3.TagSet{Title: "فيروز", Artist: "Max Richter", Album: "Sleep"}},
		{"some missing", testmp3.WithTags(audio, "Only Title", "", ""), mp3.TagSet{Title: "Only Title"}},
		{"tag that is only padding", testmp3.WithID3v2(audio, 4, 200), mp3.TagSet{}},
		{"recording date", testmp3.WithTextFrames(audio, [2]string{"TIT2", "Song"}, [2]string{"TDRC", "2015-09-04"}), mp3.TagSet{Title: "Song", Date: "2015-09-04"}},
		{"year of an older tag", testmp3.WithTextFrames(audio, [2]string{"TYER", "1987"}), mp3.TagSet{Date: "1987"}},
		{"both: the newer frame wins", testmp3.WithTextFrames(audio, [2]string{"TYER", "1987"}, [2]string{"TDRC", "1987-06"}), mp3.TagSet{Date: "1987-06"}},
		{"cover only", testmp3.WithCover(audio, "image/png", testmp3.PNG), mp3.TagSet{Cover: testmp3.PNG, CoverMIME: "image/png"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mp3.ReadTagSet(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("ReadTagSet = %+v, want %+v", got, c.want)
			}
		})
	}
}

// afterTag returns the bytes of a rendered file following its ID3v2 tag.
func afterTag(file []byte) []byte {
	size := int(file[6])<<21 | int(file[7])<<14 | int(file[8])<<7 | int(file[9])
	return file[10+size:]
}

func be32(b []byte) int { return int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3]) }

func TestRenderWritesAnAccurateXingHeader(t *testing.T) {
	audio := testmp3.Silent(200)
	file, err := mp3.Render(audio, mp3.TagSet{Title: "T"})
	if err != nil {
		t.Fatal(err)
	}
	frame := afterTag(file)
	// The header sits after the 4-byte frame header and 32 bytes of side info.
	if string(frame[36:40]) != "Xing" {
		t.Fatalf("first frame holds %q at the Xing offset, want Xing", frame[36:40])
	}
	if flags := be32(frame[40:44]); flags != 0x7 {
		t.Errorf("flags = %#x, want frames+bytes+toc (0x7)", flags)
	}
	if frames := be32(frame[44:48]); frames != 200 {
		t.Errorf("frame count = %d, want 200", frames)
	}
	wantBytes := len(frame) // everything after the tag: the header frame plus the audio
	if got := be32(frame[48:52]); got != wantBytes {
		t.Errorf("byte count = %d, want %d", got, wantBytes)
	}
	toc := frame[52:152]
	if toc[0] != 0 {
		t.Errorf("toc[0] = %d, want 0", toc[0])
	}
	for i := 1; i < 100; i++ {
		if toc[i] < toc[i-1] {
			t.Fatalf("toc not monotonic at %d: %v", i, toc)
		}
	}
	// Constant bitrate: half way in time is about half way in bytes.
	if toc[50] < 124 || toc[50] > 132 {
		t.Errorf("toc[50] = %d, want about 128", toc[50])
	}

	// The header frame is not audio: reading it back gives the original audio.
	back, dur, _ := mp3.Parse(file)
	_, wantDur, _ := mp3.Parse(audio)
	if !bytes.Equal(back, audio) || dur != wantDur {
		t.Errorf("the header frame leaked into the audio: %d bytes, %v", len(back), dur)
	}
}

func TestRenderXingHeaderForTrimmedAudio(t *testing.T) {
	trimmed, _ := mp3.Trim(testmp3.Silent(200), time.Second, 0)
	file, _ := mp3.Render(trimmed, mp3.TagSet{})
	frame := afterTag(file)
	if got, want := be32(frame[44:48]), len(trimmed)/testmp3.FrameSize; got != want {
		t.Errorf("frame count = %d, want %d (the trimmed count, not the original)", got, want)
	}
}

func TestRenderWritesTheXingHeaderEvenWhenTheFirstFrameIsTiny(t *testing.T) {
	audio := testmp3.SilentLow(60) // 32 kbps: 104-byte frames
	file, err := mp3.Render(audio, mp3.TagSet{})
	if err != nil {
		t.Fatal(err)
	}
	frame := afterTag(file)
	if string(frame[36:40]) != "Xing" {
		t.Fatalf("no Xing header: %q", frame[36:40])
	}
	if frames := be32(frame[44:48]); frames != 60 {
		t.Errorf("frame count = %d, want 60", frames)
	}
	back, dur, err := mp3.Parse(file)
	_, wantDur, _ := mp3.Parse(audio)
	if err != nil || !bytes.Equal(back, audio) || dur != wantDur {
		t.Errorf("header leaked into the audio: %d bytes, %v, %v", len(back), dur, err)
	}
}
