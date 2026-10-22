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
	orig := s
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) > 3 {
		return 0, fmt.Errorf("invalid time %q", orig)
	}
	var total time.Duration
	for i, p := range parts {
		last := i == len(parts)-1
		if p == "" || strings.HasSuffix(p, ".") || strings.ContainsAny(p, "+-eE") {
			return 0, fmt.Errorf("invalid time %q", orig)
		}
		var unit time.Duration
		if last {
			unit = time.Second
		} else if i == len(parts)-2 {
			unit = time.Minute
		} else {
			unit = time.Hour
		}
		if !last && strings.Contains(p, ".") {
			return 0, fmt.Errorf("invalid time %q: only seconds may have a fraction", orig)
		}
		v, err := strconv.ParseFloat(p, 64)
		if err != nil || v < 0 {
			return 0, fmt.Errorf("invalid time %q", orig)
		}
		if i > 0 && v >= 60 {
			return 0, fmt.Errorf("invalid time %q: %s must be below 60", orig, map[bool]string{true: "seconds", false: "minutes"}[last])
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
