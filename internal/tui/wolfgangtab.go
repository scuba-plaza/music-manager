package tui

import (
	"path"
	"strings"
	"unicode/utf8"

	"music-manager/internal/device"
	"music-manager/internal/layout"
	"music-manager/internal/manifest"
)

// WithWolfgang adds a third tab to the inspector, a read-only preview of what
// a sync for the Wolfgang WMP01 writes for the track, and starts the sync
// dialog on that format for a drive.
func WithWolfgang() Option {
	return func(m *Model) {
		m.wolfgang = true
		m.keys.Tabs.SetHelp("1-"+itoa(len(inspectorTabs)), "tabs")
	}
}

// tabCount is how many tabs the inspector has: the preview of the Wolfgang is
// only there when it was asked for.
func (m Model) tabCount() int {
	if m.wolfgang {
		return len(inspectorTabs)
	}
	return len(inspectorTabs) - 1
}

// lastField is the place in trackFields of the last field the cursor can reach.
func (m Model) lastField() int { return firstFieldOf(m.tabCount()) - 1 }

// wolfgangLines is the preview: the file, the tags the player shows, and what
// it will not show well. The names are the ones a sync derives; none is stored.
func (m Model) wolfgangLines(t manifest.Track, w int) []string {
	dev := device.Wolfgang
	vw := max(w-labelWidth-2, 1)
	var lines []string
	row := func(label, value string, tail ...string) {
		prefix := "  " + styles.dim.Render(fit(label, labelWidth-2)) + " "
		indent := strings.Repeat(" ", labelWidth+1)
		for i, l := range wrap(value, vw) {
			if i == 0 {
				lines = append(lines, prefix+l+strings.Join(tail, ""))
			} else {
				lines = append(lines, indent+l)
			}
		}
	}

	p := layout.Assign(m.tracks, dev.Layout())[t.ID]
	if t.Status != manifest.Identified {
		lines = append(lines, " "+styles.warn.Render("Not identified yet: it goes to the _Inbox folder."))
	}
	row("Folder", path.Dir(p))
	row("File", path.Base(p))
	uncut := dev.Layout()
	uncut.StemMax = 0
	whole := strings.TrimSuffix(path.Base(layout.PathFor(t, uncut)), ".mp3")
	if n := utf8.RuneCountInString(whole); n > stemShown {
		kept := string([]rune(whole)[:stemShown])
		row("Cut", kept, styles.dim.Render(string([]rune(whole)[stemShown:])))
		lines = append(lines, hintLines(strings.Repeat(" ", labelWidth+1), "The player shows "+itoa(stemShown)+" characters of a file name; the dim part is cut off.", vw)...)
	}
	if t.Status == manifest.Identified {
		tags := dev.Tags(t)
		row("Artist", tags.Artist, counted(tags.Artist, device.PlayerFieldWidth))
		row("Album", tags.Album, counted(tags.Album, device.PlayerFieldWidth))
		genre := tags.Genre
		if genre == "" {
			genre = styles.dim.Render("—")
		}
		row("Genre", genre, counted(tags.Genre, device.PlayerFieldWidth))
		row("Title", tags.Title)
		lines = append(lines, hintLines(strings.Repeat(" ", labelWidth+1), "The title is kept whole: the player shows file names, not titles.", vw)...)
		lines = appendWarnings(lines, dev, m.tracks, t, w)
	}
	return append(lines, "", " "+styles.dim.Render("A preview: nothing of it is stored."))
}

// stemShown is how many characters of a file name the Wolfgang shows.
const stemShown = 24

// counted says how much of a width a name uses, dimly.
func counted(s string, width int) string {
	if s == "" {
		return ""
	}
	return styles.dim.Render("  · " + itoa(utf8.RuneCountInString(s)) + " of " + itoa(width))
}

// appendWarnings adds what the target will not do well for the track.
func appendWarnings(lines []string, dev device.Target, all []manifest.Track, t manifest.Track, w int) []string {
	var first = true
	for _, wn := range dev.Diagnose(all) {
		if wn.TrackID != t.ID {
			continue
		}
		if first {
			lines = append(lines, "")
			first = false
		}
		for i, l := range wrap(wn.Message, w-4) {
			prefix := "   "
			if i == 0 {
				prefix = " ! "
			}
			lines = append(lines, styles.warn.Render(prefix+l))
		}
	}
	return lines
}
