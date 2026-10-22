package tui

import (
	"fmt"
	"strings"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/layout"
	"music-manager/internal/manifest"
	"music-manager/internal/timecode"
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
		return trackLines(r.Track, w)
	case kindGroup:
		return groupLines(r, w)
	}
	return m.summaryLines(r, w)
}

func groupLines(r Row, w int) []string {
	lines := []string{"", " " + styles.bold.Render(r.Label), " " + styles.dim.Render(strings.Repeat("─", max(w-2, 0))), ""}
	text := "These tracks have not been identified yet, so they have no proper title or artist and sit in the _Inbox folder."
	for _, l := range wrap(text, w-2) {
		lines = append(lines, " "+l)
	}
	return lines
}

// summaryLines describes an artist or album: how much is in it.
func (m Model) summaryLines(r Row, w int) []string {
	lines := []string{"", " " + styles.bold.Render(r.Label), " " + styles.dim.Render(strings.Repeat("─", max(w-2, 0))), ""}
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

const labelWidth = 11

// trackLines shows a track's details.
func trackLines(t manifest.Track, w int) []string {
	artist, title, album := app.Display(t)
	lines := []string{"", " " + styles.bold.Render(fit(title, w-2)), " " + styles.dim.Render(strings.Repeat("─", max(w-2, 0)))}
	field := func(name, value string) {
		lines = append(lines, " "+styles.dim.Render(fit(name, labelWidth))+value)
	}

	if t.Status != manifest.Identified {
		lines = append(lines, "")
		lines = append(lines, " "+styles.warn.Render("Not identified yet. This is what the file said:"))
		field("Title", title)
		field("Artist", artist)
		field("Album", album)
		return lines
	}

	nameField := func(name, original, ascii string) {
		shown := original
		if shown == "" {
			shown = ascii
		}
		if shown == "" {
			shown = styles.dim.Render("—")
		}
		field(name, shown)
		if original != "" && original != ascii {
			kind := "auto"
			if ascii != translit.ToASCII(original) {
				kind = "custom"
			}
			lines = append(lines, " "+strings.Repeat(" ", labelWidth)+styles.dim.Render("ascii  "+ascii+" · "+kind))
		}
	}
	nameField("Title", t.OriginalTitle, t.Title)
	nameField("Artist", t.OriginalArtist, t.Artist)
	if t.Album == "" {
		field("Album", styles.dim.Render("— (a single)"))
	} else {
		nameField("Album", t.OriginalAlbum, t.Album)
	}
	switch {
	case t.TrackNumber > 0 && t.TrackTotal > 0:
		field("Track", fmt.Sprintf("%d of %d", t.TrackNumber, t.TrackTotal))
	case t.TrackNumber > 0:
		field("Track", fmt.Sprintf("%d", t.TrackNumber))
	default:
		field("Track", styles.dim.Render("—"))
	}
	field("Cut start", timecode.Format(t.TrimStart))
	field("Cut end", timecode.Format(t.TrimEnd))

	lines = append(lines, " "+styles.dim.Render(strings.Repeat("─", max(w-2, 0))))
	for i, l := range wrap(layout.PathFor(t), w-4) {
		prefix := "   "
		if i == 0 {
			prefix = " → "
		}
		lines = append(lines, prefix+styles.accent.Render(l))
	}
	about := clock(t.PlayingLength()) + " plays · identified"
	if t.SourceURL != "" {
		about += " · from " + t.SourceURL
	}
	lines = append(lines, " "+styles.dim.Render(fit(about, w-2)))
	return lines
}

// clock formats a length as m:ss.
func clock(d time.Duration) string {
	s := int(d / time.Second)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }
