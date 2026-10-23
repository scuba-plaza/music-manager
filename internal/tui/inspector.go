package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

	"music-manager/internal/app"
	"music-manager/internal/layout"
	"music-manager/internal/manifest"
	"music-manager/internal/mp3"
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
	if r.Kind == kindTrack {
		return m.trackLines(r.Track, w, h)
	}
	return m.peekLines(r, w, h)
}

func rule(w int) string { return " " + styles.dim.Render(strings.Repeat("─", max(w-2, 0))) }

// labelWidth is the width of the label column, including the focus marker.
const labelWidth = 12

// trackHead is everything in the inspector above the cover: the title, and
// the fields, with the one being edited open.
func (m Model) trackHead(t manifest.Track, w, h int) []string {
	return m.headLines(t, w, m.compact(t, w, h))
}

// compact says whether the inspector leaves out the ASCII forms under the
// names, because the cover would be smaller than comfortCoverRows with them
// and fits (at least minCoverRows) without. The answer depends only on the
// size of the pane and the track, never on what is being typed, so the
// layout does not jump about.
func (m Model) compact(t manifest.Track, w, h int) bool {
	if t.CoverSHA256 == "" {
		return false // nothing to make room for
	}
	quiet := m.quiet()
	room := func(compact bool) int { return quiet.coverRoom(t, w, h, len(quiet.headLines(t, w, compact))) }
	return room(false) < comfortCoverRows && room(true) >= minCoverRows
}

// headLines is trackHead with the ASCII forms of the names left out if compact.
func (m Model) headLines(t manifest.Track, w int, compact bool) []string {
	_, title, _ := app.Display(t)
	tab := m.tab()
	strip, _ := m.tabStrip()
	lines := []string{strip, " " + styles.bold.Render(fit(title, w-2)), rule(w)}
	if t.Status != manifest.Identified && tab == 0 {
		lines = append(lines, " "+styles.warn.Render("Not identified yet. Dim values are from the file's old tags."))
	}

	for i, f := range trackFields {
		if tabOf(f) != tab {
			continue
		}
		lines = append(lines, m.fieldLines(t, f, m.focus == paneInspector && i == m.field, w, compact)...)
		if f == fCutEnd {
			lines = append(lines, m.barLines(t, w))
		}
		if f == fFadeOut {
			lines = append(lines, m.levelLines(t, w)...)
		}
	}
	if tab == tabOf(fPlayerArtist) {
		lines = append(lines, "")
		text := fmt.Sprintf("The player keeps only %d characters of the artist and of the album, so a longer name is shortened here, at a word. Type a name to choose it yourself.", app.PlayerFieldWidth)
		for _, l := range wrap(text, w-2) {
			lines = append(lines, " "+styles.dim.Render(l))
		}
	}
	return lines
}

// trackLines shows a track's fields, with the one being edited open.
func (m Model) trackLines(t manifest.Track, w, h int) []string {
	shown := t
	if m.editing {
		shown = previewTrack(t, m.dr)
	}
	lines := m.trackHead(t, w, h)
	foot := m.footLines(t, shown, w)
	if m.tab() == 0 { // the cover belongs to the details
		lines = append(lines, m.thumbLines(t, w, h, h-len(lines)-len(foot)-coverSlack)...)
	}
	return append(lines, foot...)
}

// footLines is what the inspector shows under the cover: where the file will
// be, when the track was added and edited, and what else is known about it.
// shown is the track as it will be once what is being typed is saved.
func (m Model) footLines(t, shown manifest.Track, w int) []string {
	lines := []string{rule(w)}
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
func (m Model) fieldLines(t manifest.Track, f fieldKind, current bool, w int, compact bool) []string {
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
	if f == fPlayerArtist || f == fPlayerAlbum {
		return playerFieldLines(t, f, prefix, indent, vw)
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
	if f.isName() && t.Status == manifest.Identified && !compact {
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
// minCoverCols columns to be worth drawing. Below it come the footer lines,
// and coverSlack more: typing a title can make the path wrap onto another
// line, and the cover is not to be sent again at another size for that.
const (
	minCoverRows = 3
	minCoverCols = 6
	coverSlack   = 1

	// comfortCoverRows is the size the cover is worth keeping: below it the
	// ASCII forms under the names give way to it.
	comfortCoverRows = 8
)

// coverRoom is how many lines the cover may take in an inspector w wide and
// h high, when headLen lines come above it, as it is planned: with nothing
// being typed.
func (m Model) coverRoom(t manifest.Track, w, h, headLen int) int {
	return h - headLen - len(m.footLines(t, t, w)) - coverSlack
}

// thumbLines draws the cover under its row, if there is a cover and room
// for it: room lines, in an inspector w wide and h high.
func (m Model) thumbLines(t manifest.Track, w, h, room int) []string {
	th, ok := m.thumbs[t.CoverSHA256]
	if t.CoverSHA256 == "" || !ok || th.img == nil {
		return nil
	}
	indent := strings.Repeat(" ", labelWidth+1)

	// In full resolution, once the terminal has the picture; the blocks stand
	// in until then, and wherever the picture is not possible.
	if m.gfx != nil && th.kitty != nil {
		cols, rows := m.coverPlan(t, th, w, h)
		if p, sent := m.pictures[t.CoverSHA256]; sent && p.cols == cols && p.rows == rows && min(rows, room) >= minCoverRows {
			// While a field is being edited there can be a line or two less room
			// than was planned. The picture is then cut off at the bottom, rather
			// than sent again at another size for every key that is typed.
			var lines []string
			for _, l := range placeholderLines(p.id, cols, rows)[:min(rows, room)] {
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
	case m.dr.field == fGenre:
		first.Width = max(vw-1, 1)
		lines = append(lines, prefix+first.View(), indent+styles.dim.Render(fit("e.g. Folk or Neoclassical · empty for none", vw)))
	case m.dr.field == fVolume:
		first.Width = max(vw-1, 1)
		lines = append(lines, prefix+first.View())
		lines = append(lines, hintLines(indent, "dB, like +3 or -4.5, in steps of 1.5 dB up to 24; empty for no change. Boosting can clip.", vw)...)
	case m.dr.field == fFadeIn || m.dr.field == fFadeOut:
		first.Width = max(vw-1, 1)
		lines = append(lines, prefix+first.View())
		lines = append(lines, hintLines(indent, "how long, like 3 or 0:02.5, or the stretch it covers, like 0:02-0:05; empty for none", vw)...)
	case m.dr.field == fPlayerArtist || m.dr.field == fPlayerAlbum:
		first.Width = max(vw-1, 1)
		n := utf8.RuneCountInString(strings.TrimSpace(first.Value()))
		count := styles.dim.Render(fit(fmt.Sprintf("%d of %d characters · empty for the automatic name", n, app.PlayerFieldWidth), vw))
		if over := n - app.PlayerFieldWidth; over > 0 {
			count = styles.warn.Render(fit(fmt.Sprintf("%d of %d characters · the player cuts the last %d", n, app.PlayerFieldWidth, over), vw))
		}
		lines = append(lines, prefix+first.View(), indent+count)
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

// quiet is the model as the cover is planned for: looking at the details,
// with nothing being typed and the keyboard in the tree. The cover is laid out
// as if so whatever the inspector shows now, so that typing, or another tab,
// does not make it be sent again at another size.
func (m Model) quiet() Model {
	q := m
	q.editing, q.focus, q.field = false, paneBrowser, 0
	return q
}

// tabStrip draws the names of the tabs, the one shown highlighted, and says
// where each label starts and ends (columns inside the pane) for the mouse.
func (m Model) tabStrip() (string, [][2]int) {
	strip, x := " ", 1
	spans := make([][2]int, len(inspectorTabs))
	for i, tab := range inspectorTabs {
		// Each label has the number that jumps to it. The one shown has a mark
		// as well as the colour, for terminals without colour.
		st, mark := styles.dim, " "
		if i == m.tab() {
			st, mark = styles.selected, "▌"
		}
		label := fmt.Sprintf(" %s%d %s ", mark, i+1, tab.name)
		w := ansi.StringWidth(label)
		strip += st.Render(label) + " "
		spans[i] = [2]int{x, x + w}
		x += w + 1
	}
	return strip, spans
}

// playerFieldLines is the artist or album the player shows, with beneath it
// where it comes from when that is worth saying.
func playerFieldLines(t manifest.Track, f fieldKind, prefix, indent string, vw int) []string {
	if t.Status != manifest.Identified {
		return []string{prefix + styles.dim.Render("— (not identified yet)")}
	}
	shown, _ := playerShown(t, f)
	own, real := t.PlayerArtist, t.Artist
	if f == fPlayerAlbum {
		own, real = t.PlayerAlbum, t.Album
	}
	lines := []string{prefix + fit(shown, vw)}

	var note []string
	switch {
	case own != "":
		note = append(note, "your own")
	case f == fPlayerAlbum && real == "":
		note = append(note, "a single, so the title stands in")
	}
	if real != "" && real != shown {
		note = append(note, `from "`+real+`"`)
	}
	over := utf8.RuneCountInString(shown) - app.PlayerFieldWidth
	if over > 0 {
		note = append(note, fmt.Sprintf("the player cuts it at %d", app.PlayerFieldWidth))
	}
	if len(note) == 0 {
		return lines
	}
	st := styles.dim
	if over > 0 {
		st = styles.warn
	}
	return append(lines, indent+st.Render(fit(strings.Join(note, " · "), vw)))
}

// hintLines is a dim explanation under a field, wrapped to the width.
func hintLines(indent, text string, vw int) []string {
	var lines []string
	for _, l := range wrap(text, vw) {
		lines = append(lines, indent+styles.dim.Render(l))
	}
	return lines
}

// levelLines is the line that shows what the fades do to the volume across the
// track, or nothing if there are none.
func (m Model) levelLines(t manifest.Track, w int) []string {
	if t.FadeIn == (manifest.Span{}) && t.FadeOut == (manifest.Span{}) {
		return nil
	}
	start, end := m.cuts(t)
	shape := mp3.Shape{FadeIn: mp3.Fade(t.FadeIn), FadeOut: mp3.Fade(t.FadeOut)}
	prefix := " " + " " + styles.dim.Render(fit("Level", labelWidth-2)) + " "
	return []string{prefix + levelBar(t.SourceLength, start, end, shape, max(w-labelWidth-2, 8))}
}
