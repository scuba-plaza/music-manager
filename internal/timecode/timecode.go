// Package timecode parses and formats positions in a track, such as 1:30.5.
package timecode

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Parse reads [[h:]m:]s[.fraction]. Minutes and seconds must be below 60
// when a larger unit is present.
func Parse(s string) (time.Duration, error) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) > 3 {
		return 0, fmt.Errorf("invalid time %q", s)
	}
	var total time.Duration
	for i, p := range parts {
		last := i == len(parts)-1
		if p == "" || strings.HasSuffix(p, ".") || strings.ContainsAny(p, "+-eE") {
			return 0, fmt.Errorf("invalid time %q", s)
		}
		unit, name := time.Second, "seconds"
		switch len(parts) - 1 - i {
		case 1:
			unit, name = time.Minute, "minutes"
		case 2:
			unit = time.Hour
		}
		if !last && strings.Contains(p, ".") {
			return 0, fmt.Errorf("invalid time %q: only seconds may have a fraction", s)
		}
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid time %q", s)
		}
		if i > 0 && v >= 60 {
			return 0, fmt.Errorf("invalid time %q: %s must be below 60", s, name)
		}
		total += time.Duration(v * float64(unit))
	}
	return total, nil
}

// Format renders d as m:ss, h:mm:ss, with milliseconds when present.
func Format(d time.Duration) string {
	h := int(d / time.Hour)
	m := int(d%time.Hour) / int(time.Minute)
	sec := d % time.Minute
	secs := fmt.Sprintf("%02d", int(sec/time.Second))
	if ms := int(sec % time.Second / time.Millisecond); ms != 0 {
		secs += strings.TrimRight(fmt.Sprintf(".%03d", ms), "0")
	}
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%s", h, m, secs)
	}
	return fmt.Sprintf("%d:%s", m, secs)
}

// Clock renders d as m:ss, rounded down, for a column of lengths: the minutes
// go past 59 rather than turn into hours.
func Clock(d time.Duration) string {
	s := int(d / time.Second)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}
