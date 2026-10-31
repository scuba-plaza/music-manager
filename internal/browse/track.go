package browse

import (
	"math"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"music-manager/internal/app"
	"music-manager/internal/device"
	"music-manager/internal/layout"
	"music-manager/internal/manifest"
	"music-manager/internal/mp3"
	"music-manager/internal/timecode"
)

// FieldView is one row of the inspector: a label and a value, and what is
// needed to edit it.
type FieldView struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Text  string `json:"text"`
	// Empty is shown in place of an empty Text.
	Empty string `json:"empty,omitempty"`
	// Dim marks a value that is not the track's own: it comes from the old
	// tags of a track nobody has identified, or is a Latin form worked out.
	Dim bool `json:"dim,omitempty"`
	// Mixed says the tracks of a group differ in this field.
	Mixed    bool `json:"mixed,omitempty"`
	Editable bool `json:"editable"`
	// Edit is what the input starts with: the text, and for the track number
	// the number of tracks.
	Edit   string `json:"edit"`
	Second string `json:"second,omitempty"`
	Hint   string `json:"hint,omitempty"`
}

// BarView is what the cuts and fades do to a track, for a screen to draw.
type BarView struct {
	Total int64  `json:"total"` // ns
	Start int64  `json:"start"`
	End   int64  `json:"end"`
	Kept  string `json:"kept"`
	// Level is the volume across the track in 100 steps, 0 to 1, -1 where a
	// cut removes the audio; empty when there are no fades.
	Level []float64 `json:"level,omitempty"`
}

type DeviceRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
	// Tail is dim text after the value: how much of a width it uses, or the
	// part of a name that is cut off.
	Tail string `json:"tail,omitempty"`
	Hint string `json:"hint,omitempty"`
	Cut  bool   `json:"cut,omitempty"`
}

type DeviceView struct {
	Notice   string      `json:"notice,omitempty"`
	Rows     []DeviceRow `json:"rows"`
	Warnings []string    `json:"warnings,omitempty"`
	Footnote string      `json:"footnote"`
}

// TabView is a page of the inspector.
type TabView struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Notes    []string    `json:"notes,omitempty"`
	Warn     bool        `json:"warn,omitempty"`
	Fields   []FieldView `json:"fields"`
	Bar      *BarView    `json:"bar,omitempty"`
	Wolfgang *DeviceView `json:"wolfgang,omitempty"`
	Device   *DeviceView `json:"device,omitempty"`
}

// TrackView is the inspector for one track.
type TrackView struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Identified bool      `json:"identified"`
	Tabs       []TabView `json:"tabs"`
	CoverSHA   string    `json:"coverSha,omitempty"`
	CoverInfo  string    `json:"coverInfo,omitempty"`
	Path       string    `json:"path"`
	Dates      string    `json:"dates"`
	About      string    `json:"about"`
	// Reveal lists the rows of the tree that must be open to see the track.
	Reveal []string `json:"reveal"`
}

// TrackOptions are what the inspector depends on besides the track.
type TrackOptions struct {
	// Wolfgang adds the preview tab.
	Wolfgang bool
	// CoverInfo describes a stored cover image, for example "500×500 JPEG".
	CoverInfo func(sha string) string
	Profile   string
	Profiles  *device.Registry
}

// Track makes the inspector of the track with the given ID.
func Track(s Snapshot, id string, o TrackOptions) (TrackView, bool) {
	t, ok := s.Track(id)
	if !ok {
		return TrackView{}, false
	}
	_, title, _ := app.Display(t)
	v := TrackView{
		ID: t.ID, Title: title, Identified: t.Status == manifest.Identified, CoverSHA: t.CoverSHA256,
		Path: pathOf(t), Dates: datesText(t), About: aboutText(t),
		Reveal: ancestorKeys(t),
	}
	if t.CoverSHA256 != "" && o.CoverInfo != nil {
		v.CoverInfo = o.CoverInfo(t.CoverSHA256)
	}
	profiles := o.Profiles
	if profiles == nil {
		profiles = device.Builtin()
	}
	previewed, ok := profiles.Get(o.Profile)
	if o.Profile != "" && !ok {
		return TrackView{}, false
	}
	for i, tab := range inspectorTabs {
		if tab.id == "wolfgang" && !o.Wolfgang || tab.id == "device" && o.Profile == "" {
			continue
		}
		tv := TabView{ID: tab.id, Name: tab.name, Fields: []FieldView{}}
		switch {
		case i == 0 && t.Status != manifest.Identified:
			tv.Notes, tv.Warn = []string{"Not identified yet. Dim values are from the file's old tags."}, true
		case i == tabOf(fLatinTitle):
			tv.Notes = []string{"Dim forms are worked out from the names. Type one to set it by hand; sorting, search and devices that cannot show the true names use it."}
		}
		for _, f := range tab.fields {
			if f == fWolfgang {
				wolfgang, _ := profiles.Get("wolfgang-wmp01")
				tv.Wolfgang = deviceView(s, t, wolfgang)
				continue
			}
			if f == fDevice {
				tv.Device = deviceView(s, t, previewed)
				continue
			}
			fv := fieldView(t, f, v.CoverInfo)
			tv.Fields = append(tv.Fields, fv)
		}
		if tab.name == "Audio" {
			tv.Bar = barView(t)
		}
		v.Tabs = append(v.Tabs, tv)
	}
	return v, true
}

// fieldView describes one field of a track.
func fieldView(t manifest.Track, f fieldKind, coverInfo string) FieldView {
	d := newDraft(t, f)
	fv := FieldView{
		Key: f.key(), Label: f.label(), Text: displayValue(t, f), Editable: true,
		Edit: d.text, Second: d.second, Hint: f.hint(t),
	}
	switch {
	case f == fCover:
		fv.Text, fv.Empty = coverInfo, "none"
		if t.CoverSHA256 != "" && coverInfo == "" {
			fv.Text = "cover"
		}
		fv.Edit = ""
	case f == fAlbum:
		fv.Empty = "— (a single)"
	case f == fCutStart || f == fCutEnd:
		fv.Empty = ""
	default:
		fv.Empty = "—"
	}
	switch {
	case prefilledFrom(t, f):
		fv.Dim = true
	case f.isLatin() && !handSet(t, f):
		fv.Dim = true
	}
	if f.isLatin() && trueName(t, f) == "" {
		fv.Editable = false // there is no name to give a form of
	}
	return fv
}

// key is the name a field goes by outside this package.
func (f fieldKind) key() string {
	return [...]string{
		"title", "artist", "album", "track", "released", "genre", "cutStart", "cutEnd", "cover",
		"volume", "fadeIn", "fadeOut", "latinTitle", "latinArtist", "latinAlbum", "latinGenre",
		"wolfgang", "albumOf", "playlistName", "playlistLatin", "device",
	}[f]
}

// hint explains what may be typed in a field.
func (f fieldKind) hint(t manifest.Track) string {
	switch {
	case f.isLatin():
		return "for " + trueName(t, f) + " · empty takes the form worked out back"
	case f == fCover:
		return "a JPEG or PNG image · choose or drop one, or remove the cover"
	case f == fReleased:
		return "e.g. 1999, 1999-05 or 1999-05-17 · empty if unknown"
	case f == fGenre:
		return "e.g. Folk or Neoclassical · empty for none"
	case f == fVolume:
		return "dB, like +3 or -4.5, in steps of 1.5 dB up to 24; empty for no change. Boosting can clip."
	case f == fFadeIn || f == fFadeOut:
		return "how long, like 3 or 0:02.5, or the stretch it covers, like 0:02-0:05; empty for none"
	case f == fCutStart || f == fCutEnd:
		return "e.g. 0:12 or 1:30.5"
	}
	return ""
}

// prefilledFrom reports whether the field of t shows what the file's old tags
// said, because the track is not identified and has nothing of its own.
func prefilledFrom(t manifest.Track, f fieldKind) bool {
	if t.Status == manifest.Identified {
		return false
	}
	switch f {
	case fTitle:
		return t.Title == ""
	case fArtist:
		return t.Artist == ""
	case fAlbum:
		return t.Album == "" && t.Hint.Album != ""
	case fTrack:
		return t.TrackNumber == 0 && t.Hint.Track > 0
	case fReleased:
		return t.Released == "" && t.Hint.Date != ""
	case fGenre:
		return t.Genre == "" && t.Hint.Genre != ""
	}
	return false
}

// barView is what the cuts keep of a track, and the volume across it.
func barView(t manifest.Track) *BarView {
	b := &BarView{
		Total: int64(t.SourceLength), Start: int64(t.TrimStart), End: int64(t.TrimEnd),
		Kept: timecode.Clock(max(t.SourceLength-t.TrimStart-t.TrimEnd, 0)) + " of " + timecode.Clock(t.SourceLength),
	}
	if t.SourceLength <= 0 || (t.FadeIn == manifest.Span{} && t.FadeOut == manifest.Span{}) {
		return b
	}
	shape := mp3.Shape{FadeIn: mp3.Fade(t.FadeIn), FadeOut: mp3.Fade(t.FadeOut)}
	const steps = 100
	for i := 0; i < steps; i++ {
		middle := t.SourceLength * time.Duration(2*i+1) / time.Duration(2*steps)
		if middle < t.TrimStart || middle >= t.SourceLength-t.TrimEnd {
			b.Level = append(b.Level, -1)
			continue
		}
		b.Level = append(b.Level, math.Round(shape.Level(middle)*1000)/1000)
	}
	return b
}

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

// aboutText is the line under the dates: how long the track plays, and where it is from.
func aboutText(t manifest.Track) string {
	about := timecode.Clock(t.PlayingLength()) + " plays"
	if t.Status == manifest.Identified {
		about += " · identified"
	}
	if t.SourceURL != "" {
		about += " · from " + t.SourceURL
	}
	return about
}

func deviceView(s Snapshot, t manifest.Track, dev device.Target) *DeviceView {
	v := &DeviceView{Footnote: "A preview: nothing of it is stored.", Rows: []DeviceRow{}}
	p := layout.Assign(s.Tracks, dev.Layout())[t.ID]
	if t.Status != manifest.Identified {
		v.Notice = "Not identified yet: it goes to the _Inbox folder."
	}
	v.Rows = append(v.Rows,
		DeviceRow{Label: "Folder", Value: path.Dir(p)},
		DeviceRow{Label: "File", Value: path.Base(p)},
	)
	stem, width := dev.Profile().StemMax, dev.Profile().TagWidth
	uncut := dev.Layout()
	uncut.StemMax = 0
	whole := strings.TrimSuffix(path.Base(layout.PathFor(t, uncut)), ".mp3")
	if n := utf8.RuneCountInString(whole); stem > 0 && n > stem {
		runes := []rune(whole)
		v.Rows = append(v.Rows, DeviceRow{
			Label: "Cut", Value: string(runes[:stem]), Tail: string(runes[stem:]), Cut: true,
			Hint: "The player shows " + strconv.Itoa(stem+len(".mp3")) + " characters of a name, the .mp3 included, and scrolls a longer one: the name is cut at " + strconv.Itoa(stem) + ", the dim part is cut off.",
		})
	}
	if t.Status == manifest.Identified {
		tags := dev.Tags(t)
		title := DeviceRow{Label: "Title", Value: tags.Title}
		if stem > 0 {
			title.Hint = "The title is kept whole: the player shows file names, not titles."
		}
		v.Rows = append(v.Rows,
			DeviceRow{Label: "Artist", Value: tags.Artist, Tail: counted(tags.Artist, width)},
			DeviceRow{Label: "Album", Value: tags.Album, Tail: counted(tags.Album, width)},
			DeviceRow{Label: "Genre", Value: tags.Genre, Tail: counted(tags.Genre, width)},
			title,
		)
		for _, w := range dev.Diagnose(s.Tracks) {
			if w.TrackID == t.ID {
				v.Warnings = append(v.Warnings, w.Message)
			}
		}
	}
	return v
}

// counted says how much of a width a name uses.
func counted(s string, width int) string {
	if s == "" || width <= 0 {
		return ""
	}
	return "· " + strconv.Itoa(utf8.RuneCountInString(s)) + " of " + strconv.Itoa(width)
}

// pathOf is where the file of a track goes in the library folder.
func pathOf(t manifest.Track) string { return layout.PathFor(t, device.Generic.Layout()) }
