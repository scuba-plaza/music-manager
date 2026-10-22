package timecode_test

import (
	"testing"
	"time"

	"music-manager/internal/timecode"
)

func TestParse(t *testing.T) {
	ok := map[string]time.Duration{
		"0":       0,
		"90":      90 * time.Second,
		"1:30":    90 * time.Second,
		"1:30.5":  90*time.Second + 500*time.Millisecond,
		"0:00.25": 250 * time.Millisecond,
		"1:02:03": time.Hour + 2*time.Minute + 3*time.Second,
		" 2:00 ":  2 * time.Minute,
	}
	for in, want := range ok {
		if got, err := timecode.Parse(in); err != nil || got != want {
			t.Errorf("Parse(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "-5", "1:75", "1:2:3:4", "1::5", ":30", "1:30.", "1e3"} {
		if got, err := timecode.Parse(in); err == nil {
			t.Errorf("Parse(%q) = %v, want error", in, got)
		}
	}
}

func TestFormat(t *testing.T) {
	cases := map[time.Duration]string{
		0:                                     "0:00",
		90 * time.Second:                      "1:30",
		90*time.Second + 500*time.Millisecond: "1:30.5",
		time.Hour + 2*time.Minute + 3*time.Second: "1:02:03",
		59*time.Second + 999*time.Millisecond:     "0:59.999",
	}
	for d, want := range cases {
		if got := timecode.Format(d); got != want {
			t.Errorf("Format(%v) = %q, want %q", d, got, want)
		}
	}
}
