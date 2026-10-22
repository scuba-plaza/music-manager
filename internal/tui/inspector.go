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
func (m Model) inspectorLines(w, h int) []string {
	r, ok := m.selectedRow()
	if !ok {
		if len(m.tracks) == 0 {
			return nil
		}
		return []string{"", styles.dim.Render(" Nothing selected")}
	}
	switch r.Kind {
	case kindTrack:
		return m.trackLines(r.Track, w, h)
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

// trackHead is everything in the inspector above the cover: the title, and
// the fields, with the one being edited open.
func (m Model) trackHead(t manifest.Track, w int) []string {
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
	return lines
}

// trackLines shows a track's fields, with the one being edited open.
func (m Model) trackLines(t manifest.Track, w, h int) []string {
	lines := m.trackHead(t, w)
	lines = append(lines, m.thumbLines(t, w, h, len(lines))...)

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
	lines = append(lines, " "+styles.dim.Render(fit(datesText(t), w-2)))
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

	if f == fCover {
		return []string{prefix + m.coverText(t)}
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

// coverText says what a track's cover is.
func (m Model) coverText(t manifest.Track) string {
	switch th, ok := m.thumbs[t.CoverSHA256]; {
	case t.CoverSHA256 == "":
		return styles.dim.Render("none")
	case !ok:
		return styles.dim.Render("loading…")
	default:
		return th.info
	}
}

// The cover sits under its row. It needs at least minCoverRows lines and
// minCoverCols columns to be worth drawing, and coverReserve lines are kept
// below it for the file's path and what is known about it.
const (
	minCoverRows = 3
	minCoverCols = 6
	coverReserve = 7
)

// thumbLines draws the cover under its row, if there is a cover and room
// for it. headLen is the number of lines above it, in an inspector w wide
// and h high.
func (m Model) thumbLines(t manifest.Track, w, h, headLen int) []string {
	th, ok := m.thumbs[t.CoverSHA256]
	if t.CoverSHA256 == "" || !ok || th.img == nil {
		return nil
	}
	room := h - headLen - coverReserve
	indent := strings.Repeat(" ", labelWidth+1)

	// In full resolution, once the terminal has the picture; the blocks stand
	// in until then, and wherever the picture is not possible.
	if m.gfx != nil && th.kitty != nil {
		cols, rows := m.coverPlan(t, th, w, h)
		if p, sent := m.pictures[t.CoverSHA256]; sent && p.cols == cols && p.rows == rows && rows <= room {
			var lines []string
			for _, l := range placeholderLines(p.id, cols, rows) {
				lines = append(lines, indent+l)
			}
			return lines
		}
	}

	cols := w - labelWidth - 4
	rows := min(room, maxCoverRows)
	if rows < minCoverRows || cols < minCoverCols {
		return nil
	}
	var lines []string
	for _, l := range renderThumb(thumbCells(th.img, cols, rows)) {
		lines = append(lines, indent+l)
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
	case m.dr.field == fCover:
		first.Width = max(vw-1, 1)
		lines = append(lines, prefix+first.View(), indent+styles.dim.Render(fit("a JPEG or PNG file · empty removes the cover", vw)))
	case m.dr.field == fReleased:
		first.Width = max(vw-1, 1)
		lines = append(lines, prefix+first.View(), indent+styles.dim.Render(fit("e.g. 1999, 1999-05 or 1999-05-17 · empty if unknown", vw)))
	default:
		first.Width = max(vw-24, 1)
		lines = append(lines, prefix+first.View()+styles.dim.Render("  e.g. 0:12 or 1:30.5"))
	}
	if m.editErr != "" {
		for _, l := range wrap(m.editErr, vw) {
			lines = append(lines, indent+styles.bad.Render(l))
		}
	}
	return lines
}

// clock formats a length as m:ss.
func clock(d time.Duration) string {
	s := int(d / time.Second)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }

// datesText says when a track was added to the library and last edited.
func datesText(t manifest.Track) string {
	var parts []string
	if d := app.SortValue(t, app.SortAdded); d != "" {
		parts = append(parts, "added "+d)
	}
	if d := app.SortValue(t, app.SortEdited); d != "" {
		parts = append(parts, "edited "+d)
	}
	return strings.Join(parts, " · ")
}
