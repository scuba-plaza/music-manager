package tui

import (
	"math"
	"strings"
	"time"

	"music-manager/internal/mp3"
)

// trimBar draws a track of length total as width cells: solid where the
// audio is kept and light where the cuts remove it. A cell counts as kept if
// its middle lies in the kept stretch.
func trimBar(total, start, end time.Duration, width int) string {
	if width <= 0 {
		return ""
	}
	if total <= 0 {
		return strings.Repeat(" ", width)
	}
	var b strings.Builder
	run, runKept := 0, false
	flush := func() {
		if run == 0 {
			return
		}
		if runKept {
			b.WriteString(styles.accent.Render(strings.Repeat("█", run)))
		} else {
			b.WriteString(styles.dim.Render(strings.Repeat("░", run)))
		}
		run = 0
	}
	for i := 0; i < width; i++ {
		middle := total * time.Duration(2*i+1) / time.Duration(2*width)
		kept := middle >= start && middle < total-end
		if i > 0 && kept != runKept {
			flush()
		}
		runKept = kept
		run++
	}
	flush()
	return b.String()
}

// levelBlocks are the heights a cell of the level line can have, from silence
// to the full volume.
const levelBlocks = "▁▂▃▄▅▆▇█"

// levelBar draws the volume the fades leave across a track of length total as
// width cells: a block as high as the volume, and a light cell where the cuts
// remove the audio.
func levelBar(total, start, end time.Duration, shape mp3.Shape, width int) string {
	if width <= 0 || total <= 0 {
		return ""
	}
	blocks := []rune(levelBlocks)
	var b strings.Builder
	for i := 0; i < width; i++ {
		middle := total * time.Duration(2*i+1) / time.Duration(2*width)
		if middle < start || middle >= total-end {
			b.WriteString(styles.dim.Render("░"))
			continue
		}
		b.WriteString(styles.accent.Render(string(blocks[int(math.Round(shape.Level(middle)*float64(len(blocks)-1)))])))
	}
	return b.String()
}
