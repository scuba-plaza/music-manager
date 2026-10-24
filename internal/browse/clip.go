package browse

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"music-manager/internal/app"
	"music-manager/internal/coverimg"
	"music-manager/internal/manifest"
)

// c copies attributes of a track and v pastes them on another track, on the
// marked tracks, or on all the tracks of an album or an artist. What is copied
// is kept until the program stops or something else is copied.

// clipAttr is one attribute copied from a track: the artist, the album, how
// many tracks the album has, when it came out, the genre or the cover.
type clipAttr struct {
	field fieldKind
	// text is the value, and for the cover its SHA. A name takes the form in
	// Latin letters that was set by hand along with it, since that form belongs
	// to the name.
	text, latin string
}

// clipContents is what c copied.
type clipContents struct {
	from  string // the title of the track it came from
	attrs []clipAttr
}

// copyOrder is the order of the attributes in the checklist.
var copyOrder = []fieldKind{fCover, fArtist, fAlbum, fAlbumOf, fReleased, fGenre}

// copyKind is the attribute that c copies when the cursor is on the field f of
// a track. A title, a track number, the cuts and the volume belong to the
// track and are not copied.
func copyKind(f fieldKind) (fieldKind, bool) {
	switch f {
	case fArtist, fAlbum, fReleased, fGenre, fCover:
		return f, true
	case fTrack:
		return fAlbumOf, true
	case fLatinArtist:
		return fArtist, true
	case fLatinAlbum:
		return fAlbum, true
	case fLatinGenre:
		return fGenre, true
	}
	return 0, false
}

// clipName names an attribute in what is said about a copy.
func clipName(f fieldKind) string {
	switch f {
	case fAlbumOf:
		return "tracks on album"
	case fCover:
		return "cover"
	}
	return strings.ToLower(f.label())
}

// attrOf is the attribute f of t, if t has one to copy.
func attrOf(t manifest.Track, f fieldKind) (clipAttr, bool) {
	a := clipAttr{field: f}
	switch f {
	case fArtist:
		a.text, a.latin = t.Artist, t.ArtistLatin
	case fAlbum:
		a.text, a.latin = t.Album, t.AlbumLatin
	case fGenre:
		a.text, a.latin = t.Genre, t.GenreLatin
	case fReleased:
		a.text = t.Released
	case fAlbumOf:
		if t.TrackTotal > 0 {
			a.text = strconv.Itoa(t.TrackTotal)
		}
	case fCover:
		a.text = t.CoverSHA256
	}
	return a, a.text != ""
}

// edit is the change that pasting the attributes makes to a track.
func (c clipContents) edit() app.Edit {
	var e app.Edit
	for _, a := range c.attrs {
		text, latin := a.text, a.latin
		switch a.field {
		case fArtist:
			e.Artist, e.ArtistLatin = &text, &latin
		case fAlbum:
			e.Album, e.AlbumLatin = &text, &latin
		case fGenre:
			e.Genre, e.GenreLatin = &text, &latin
		case fReleased:
			e.Released = &text
		case fAlbumOf:
			n, _ := strconv.Atoi(text)
			e.TrackTotal = &n
		case fCover:
			e.Cover = &text
		}
	}
	return e
}

// names lists the attributes in words: "cover", "artist and cover".
func (c clipContents) names() string {
	var names []string
	for _, a := range c.attrs {
		names = append(names, clipName(a.field))
	}
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// CopyChoice is one line of the checklist of what a track has to copy.
type CopyChoice struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Has   bool   `json:"has"`
	Text  string `json:"text"`
}

// CopyChoices lists what the track has that c can copy, in the order of the checklist.
func CopyChoices(s Snapshot, id string) ([]CopyChoice, bool) {
	t, ok := s.Track(id)
	if !ok {
		return nil, false
	}
	out := []CopyChoice{}
	for _, f := range copyOrder {
		a, has := attrOf(t, f)
		label := f.label()
		if f == fAlbumOf {
			label = "Tracks on album"
		}
		text := a.text
		switch {
		case f == fCover && has:
			text = "yes"
		case a.latin != "":
			text += " (" + a.latin + ")"
		}
		out = append(out, CopyChoice{Key: f.key(), Label: label, Has: has, Text: text})
	}
	return out, true
}

// Copy keeps the attributes of a track as what Paste pastes: the keys are those of CopyChoices.
func (e *Editor) Copy(id string, attrs []string) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	t, err := track(s, id)
	if err != nil {
		return Result{}, err
	}
	var got []clipAttr
	for _, k := range attrs {
		f, ok := fieldByKey(k)
		if !ok {
			return Result{}, fmt.Errorf("there is no attribute %q to copy", k)
		}
		if a, ok := attrOf(t, f); ok {
			got = append(got, a)
		}
	}
	if len(got) == 0 {
		return Result{}, errors.New("Tick what to copy")
	}
	return e.copied(t, got), nil
}

// CopyField is c in the fields of a track: copy what the field the cursor is
// on stands for.
func (e *Editor) CopyField(id, field string) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	f, ok := fieldByKey(field)
	if !ok {
		return Result{}, fmt.Errorf("there is no field %q", field)
	}
	s, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	t, err := track(s, id)
	if err != nil {
		return Result{}, err
	}
	kind, ok := copyKind(f)
	if !ok {
		return Result{}, errors.New("The " + strings.ToLower(f.label()) + " belongs to its track and is not copied")
	}
	attr, ok := attrOf(t, kind)
	if !ok {
		return Result{}, errors.New("This track has no " + clipName(kind) + " to copy")
	}
	return e.copied(t, []clipAttr{attr}), nil
}

func (e *Editor) copied(t manifest.Track, attrs []clipAttr) Result {
	_, title, _ := app.Display(t)
	e.clip = &clipContents{from: title, attrs: attrs}
	if len(attrs) == 1 {
		return Result{Message: "Copied the " + e.clip.names() + " of " + title}
	}
	return Result{Message: "Copied " + e.clip.names() + " from " + title}
}

func haveOrHas(n int) string {
	if n == 1 {
		return "has"
	}
	return "have"
}

// Paste puts what was copied on the tracks.
func (e *Editor) Paste(ids []string) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.clip == nil {
		return Result{}, errors.New("Nothing is copied yet: c copies from a track")
	}
	s, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	targets := s.tracksFor(ids)
	if len(targets) == 0 {
		return Result{}, errors.New("Select a track, an album or an artist to paste on")
	}
	edit := e.clip.edit()
	edits := map[string]app.Edit{}
	var changed []string
	for _, t := range targets {
		if changes(t, edit) {
			edits[t.ID] = edit
			changed = append(changed, t.ID)
		}
	}
	if len(edits) == 0 {
		return Result{Message: "Nothing to paste: " + plural(len(targets), "track") + " already " + haveOrHas(len(targets)) + " the " + e.clip.names(), Same: true}, nil
	}
	names := e.clip.names()
	return e.saveMany(s, "paste of the "+names, "Pasted the "+names+" on "+plural(len(changed), "track")+" · u undoes it", changed,
		func() (map[string]manifest.Track, error) { return e.lib.EditMany(edits) })
}

// PastePicture makes the picture the cover of the tracks.
func (e *Editor) PastePicture(ids []string, data []byte, name string) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, err := coverimg.Validate(data); err != nil {
		return Result{}, err
	}
	s, err := e.snapshot()
	if err != nil {
		return Result{}, err
	}
	targets := s.tracksFor(ids)
	if len(targets) == 0 {
		return Result{}, errors.New("Select a track, an album or an artist to paste on")
	}
	sha := fmt.Sprintf("%x", sha256sum(data))
	var todo []string
	for _, t := range targets {
		if t.CoverSHA256 != sha {
			todo = append(todo, t.ID)
		}
	}
	if len(todo) == 0 {
		return Result{Message: "Nothing to paste: " + plural(len(targets), "track") + " already " + haveOrHas(len(targets)) + " this cover", Same: true}, nil
	}
	from := "the clipboard"
	if name != "" {
		from = name
	}
	return e.saveMany(s, "paste of a cover", "Pasted the cover from "+from+" on "+plural(len(todo), "track")+" · u undoes it", todo,
		func() (map[string]manifest.Track, error) { return e.lib.SetCoverMany(todo, data) })
}

func sha256sum(data []byte) [32]byte { return sha256.Sum256(data) }
