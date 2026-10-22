package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"music-manager/internal/app"
	"music-manager/internal/layout"
	"music-manager/internal/manifest"
	"music-manager/internal/translit"
)

// inspectorLines draws the right pane for whatever is selected.
func (m Model) inspectorLines(w int) []string {
	r, ok := m.selectedRow()
	if !ok {
		if len(m.tracks) == 0 {
			return nil
		}
		return []string{"", styles.dim.Render(" Nothing selected")}
	}
	switch r.Kind {
	case kindTrack:
		return m.trackLines(r.Track, w)
	case kindGroup:
		return groupLines(r, w)
	}
	return m.summaryLines(r, w)
}

func rule(w int) string { return " " + styles.dim.Render(strings.Repeat("─", max(w-2, 0))) }

func groupLines(r Row, w int) []string {
	lines := []string{"", " " + styles.bold.Render(r.Label), rule(w), ""}
	text := "These tracks have not been identified yet, so they have no proper title or artist and sit in the _Inbox folder."
	for _, l := range wrap(text, w-2) {
		lines = append(lines, " "+l)
	}
	return lines
}

// summaryLines describes an artist or album: how much is in it.
func (m Model) summaryLines(r Row, w int) []string {
	lines := []string{"", " " + styles.bold.Render(r.Label), rule(w), ""}
	lines = append(lines, " "+plural(r.Count, "track"))
	if r.Kind == kindArtist {
		albums := map[string]bool{}
		for _, t := range m.tracks {
			if t.Status == manifest.Identified && t.Artist == r.Label && t.Album != "" {
				albums[t.Album] = true
			}
		}
		if len(albums) > 0 {
			lines[len(lines)-1] += " · " + plural(len(albums), "album")
		}
	}
	return lines
}

// labelWidth is the width of the label column, including the focus marker.
const labelWidth = 12

// trackLines shows a track's fields, with the one being edited open.
func (m Model) trackLines(t manifest.Track, w int) []string {
	_, title, _ := app.Display(t)
	lines := []string{"", " " + styles.bold.Render(fit(title, w-2)), rule(w)}
	if t.Status != manifest.Identified {
		lines = append(lines, " "+styles.warn.Render("Not identified yet. Dim values are from the file's old tags."))
	}

	for i, f := range trackFields {
		lines = append(lines, m.fieldLines(t, f, m.focus == paneInspector && i == m.field, w)...)
		if f == fCutEnd {
			lines = append(lines, m.barLines(t, w))
		}
	}

	lines = append(lines, rule(w))
	shown := t
	if m.editing {
		shown = previewTrack(t, m.dr)
	}
	for i, l := range wrap(layout.PathFor(shown), w-4) {
		prefix := "   "
		if i == 0 {
			prefix = " → "
		}
		lines = append(lines, prefix+styles.accent.Render(l))
	}
	about := clock(t.PlayingLength()) + " plays"
	if t.Status == manifest.Identified {
		about += " · identified"
	}
	if t.SourceURL != "" {
		about += " · from " + t.SourceURL
	}
	return append(lines, " "+styles.dim.Render(fit(about, w-2)))
}

// fieldLines is one field: its label, its value or its open inputs, and
// beneath them the ASCII form of a name and any error.
func (m Model) fieldLines(t manifest.Track, f fieldKind, current bool, w int) []string {
	marker, label := " ", styles.dim.Render(fit(f.label(), labelWidth-2))
	if current {
		marker, label = styles.accent.Render("▌"), styles.accent.Render(fit(f.label(), labelWidth-2))
	}
	prefix := " " + marker + label + " "
	indent := strings.Repeat(" ", labelWidth+1)
	vw := max(w-labelWidth-2, 1)

	if m.editing && current {
		return m.editorLines(prefix, indent, vw)
	}

	value := displayValue(t, f)
	var lines []string
	switch {
	case value == "" && f == fAlbum:
		lines = append(lines, prefix+styles.dim.Render("— (a single)"))
	case value == "":
		lines = append(lines, prefix+styles.dim.Render("—"))
	case prefilledFrom(t, f):
		lines = append(lines, prefix+styles.dim.Render(fit(value, vw)))
	default:
		lines = append(lines, prefix+fit(value, vw))
	}
	if f.isName() && t.Status == manifest.Identified {
		original, ascii := names(t, f)
		if original != "" && original != ascii {
			kind := "auto"
			if ascii != translit.ToASCII(original) {
				kind = "custom"
			}
			lines = append(lines, indent+styles.dim.Render(fit("ascii  "+ascii+" · "+kind, vw)))
		}
	}
	return lines
}

// cuts are the cuts the bar shows: the track's own, unless one is being typed.
func (m Model) cuts(t manifest.Track) (start, end time.Duration) {
	start, end = t.TrimStart, t.TrimEnd
	if m.editing && (m.dr.field == fCutStart || m.dr.field == fCutEnd) {
		if e, err := m.dr.edit(); err == nil {
			if e.TrimStart != nil {
				start = *e.TrimStart
			}
			if e.TrimEnd != nil {
				end = *e.TrimEnd
			}
		}
	}
	return start, end
}

// barLines draws what the cuts keep of the track.
func (m Model) barLines(t manifest.Track, w int) string {
	start, end := m.cuts(t)
	kept := max(t.SourceLength-start-end, 0)
	text := clock(kept) + " of " + clock(t.SourceLength)
	prefix := " " + " " + styles.dim.Render(fit("Kept", labelWidth-2)) + " "
	width := max(w-labelWidth-2-ansi.StringWidth(text)-2, 8)
	return prefix + trimBar(t.SourceLength, start, end, width) + "  " + styles.dim.Render(text)
}

// editorLines draws the open inputs of the field being edited.
func (m Model) editorLines(prefix, indent string, vw int) []string {
	first, second := m.in[0], m.in[1]
	var lines []string
	switch {
	case m.dr.field.isName():
		first.Width, second.Width = max(vw-1, 1), max(vw-8, 1)
		lines = append(lines, prefix+first.View(), indent+styles.dim.Render("ascii  ")+second.View())
	case m.dr.field == fTrack:
		first.Width, second.Width = 5, 5
		lines = append(lines, prefix+first.View()+styles.dim.Render(" of ")+second.View())
	default:
		first.Width = max(vw-24, 1)
		lines = append(lines, prefix+first.View()+styles.dim.Render("  e.g. 0:12 or 1:30.5"))
	}
	if m.editErr != "" {
		lines = append(lines, indent+styles.bad.Render(fit(m.editErr, vw)))
	}
	return lines
}

// clock formats a length as m:ss.
func clock(d time.Duration) string {
	s := int(d / time.Second)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }
