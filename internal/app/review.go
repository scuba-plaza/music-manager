package app

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"music-manager/internal/manifest"
)

// A review file is a list of proposals for one field of many tracks, written
// to be read and corrected by a person before anything is applied:
//
//	# comments
//	## ARTISTS -> genre
//
//	Fairouz  (10 tracks)  [high ; MusicBrainz]
//	    Arabic
//
// There are up to three sections, ARTISTS, ALBUMS and TRACKS, each naming the
// field it sets after an arrow. An entry is a heading and, indented under it,
// the value to set. The heading starts with who the entry is for: the name of
// an artist or an album, or the ID of a track. It may end with a note in
// brackets that starts with how sure the proposal is: high, medium or low.
// A value of "-" or nothing skips the entry.
//
// An artist's value is set on every track of the artist, an album's on every
// track of the album, which wins over the artist's, and a track's on that
// track, which wins over both.

// ReviewEntry is one proposal of a review file.
type ReviewEntry struct {
	Section    string // ARTISTS, ALBUMS or TRACKS
	Field      string // what the value is for: genre
	Key        string // the name of the artist or album, or the ID of the track
	Value      string // "" or "-" for none
	Confidence string // high, medium or low
	Line       int    // where the entry is in the file
}

// reviewField is a field a review file can set.
type reviewField struct {
	set func(e *Edit, v string)
}

var reviewFields = map[string]reviewField{
	"genre":         {set: func(e *Edit, v string) { e.Genre = &v }},
	"player-artist": {set: func(e *Edit, v string) { e.PlayerArtist = &v }},
	"player-album":  {set: func(e *Edit, v string) { e.PlayerAlbum = &v }},
}

// reviewSections are the parts of a review file, from the least to the most
// specific.
var reviewSections = []string{"ARTISTS", "ALBUMS", "TRACKS"}

var (
	reviewNote     = regexp.MustCompile(`^(.*?)\s*\[([^\[\]]*)\]\s*$`)
	artistCount    = regexp.MustCompile(`\s+\(\d+ tracks?\)\s*$`)
	albumCount     = regexp.MustCompile(`\s+\([^()]*\d+ tracks?\)\s*$`)
	reviewArrow    = regexp.MustCompile(`\s*(?:->|→)\s*`)
	reviewHeadline = regexp.MustCompile(`^##\s+(\S+)(.*)$`)
)

// ParseReview reads a review file.
func ParseReview(text string) ([]ReviewEntry, error) {
	var (
		entries []ReviewEntry
		section string
		field   string
		pending *ReviewEntry
	)
	flush := func() {
		if pending != nil {
			entries = append(entries, *pending)
			pending = nil
		}
	}
	for i, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		n := i + 1
		switch {
		case strings.HasPrefix(line, "##"):
			flush()
			head := reviewHeadline.FindStringSubmatch(line)
			if head == nil {
				return nil, fmt.Errorf("line %d: a section looks like %q", n, "## ARTISTS -> genre")
			}
			section = strings.ToUpper(head[1])
			if !contains(reviewSections, section) {
				return nil, fmt.Errorf("line %d: section %s is not one of %s", n, head[1], strings.Join(reviewSections, ", "))
			}
			parts := reviewArrow.Split(strings.TrimSpace(head[2]), 2)
			if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
				return nil, fmt.Errorf("line %d: section %s needs the field it sets: ## %s -> genre", n, section, section)
			}
			field = strings.ToLower(strings.TrimSpace(parts[1]))
			if _, ok := reviewFields[field]; !ok {
				return nil, fmt.Errorf("line %d: %s is not a field a review can set (%s)", n, field, strings.Join(reviewFieldNames(), ", "))
			}
		case strings.HasPrefix(line, "#"), strings.TrimSpace(line) == "" && line == "":
			// a comment, or a gap between entries
		case line[0] == ' ' || line[0] == '\t':
			if pending == nil {
				return nil, fmt.Errorf("line %d: a value with no entry above it", n)
			}
			pending.Value = strings.TrimSpace(line)
			flush()
		default:
			flush()
			if section == "" {
				return nil, fmt.Errorf("line %d: an entry before any section (## ARTISTS -> genre)", n)
			}
			e, err := reviewHeading(line, section, field)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", n, err)
			}
			e.Line = n
			pending = &e
		}
	}
	flush()
	return entries, nil
}

// reviewHeading reads the heading of an entry of the section.
func reviewHeading(line, section, field string) (ReviewEntry, error) {
	e := ReviewEntry{Section: section, Field: field, Confidence: "high"}
	head := strings.TrimSpace(line)
	if note := reviewNote.FindStringSubmatch(head); note != nil {
		head = note[1]
		word, _, _ := strings.Cut(note[2], ";")
		if word = strings.ToLower(strings.TrimSpace(word)); word != "" {
			if word != "high" && word != "medium" && word != "low" {
				return e, fmt.Errorf("%q is not how sure a proposal is: use high, medium or low", word)
			}
			e.Confidence = word
		}
	}
	switch section {
	case "ARTISTS":
		e.Key = artistCount.ReplaceAllString(head, "")
	case "ALBUMS":
		e.Key = albumCount.ReplaceAllString(head, "")
	case "TRACKS":
		e.Key, _, _ = strings.Cut(head, " ")
	}
	if e.Key = strings.TrimSpace(e.Key); e.Key == "" {
		return e, fmt.Errorf("no name in %q", line)
	}
	return e, nil
}

func reviewFieldNames() []string {
	names := make([]string, 0, len(reviewFields))
	for name := range reviewFields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// ReviewOptions say how to apply a review.
type ReviewOptions struct {
	DryRun     bool // check and count, change nothing
	IncludeLow bool // apply the proposals marked low too
}

// ReviewReport says what applying a review did, or would do.
type ReviewReport struct {
	Applied   int      // entries that were used
	Blank     int      // entries with no value
	Low       int      // entries left out for being marked low
	Unmatched []string // entries that fit no track, as "SECTION: key"
	Changed   int      // tracks that changed
}

// ApplyReview sets the proposed values: either all of them are saved together
// or, if any is invalid, none.
func (a *App) ApplyReview(entries []ReviewEntry, opts ReviewOptions) (ReviewReport, error) {
	var rep ReviewReport
	m, err := a.store.Load()
	if err != nil {
		return rep, err
	}

	// The most specific proposals come last, so that they win.
	ordered := append([]ReviewEntry(nil), entries...)
	rank := func(s string) int {
		for i, name := range reviewSections {
			if name == s {
				return i
			}
		}
		return len(reviewSections)
	}
	sort.SliceStable(ordered, func(i, j int) bool { return rank(ordered[i].Section) < rank(ordered[j].Section) })

	edits := map[string]Edit{}
	for _, e := range ordered {
		f, ok := reviewFields[e.Field]
		if !ok {
			return rep, fmt.Errorf("%s is not a field a review can set (%s)", e.Field, strings.Join(reviewFieldNames(), ", "))
		}
		switch {
		case e.Value == "" || e.Value == "-":
			rep.Blank++
			continue
		case e.Confidence == "low" && !opts.IncludeLow:
			rep.Low++
			continue
		}
		matched := false
		for _, t := range m.Tracks {
			if reviewMatches(e, t) {
				matched = true
				edit := edits[t.ID]
				f.set(&edit, e.Value)
				edits[t.ID] = edit
			}
		}
		if !matched {
			rep.Unmatched = append(rep.Unmatched, e.Section+": "+e.Key)
			continue
		}
		rep.Applied++
	}

	// Only the tracks the values would change, and every value checked before
	// anything is saved.
	for id, edit := range edits {
		t, _ := m.Get(id)
		after, err := applyEdit(t, edit)
		if err != nil {
			return rep, fmt.Errorf("%s: %w", id, err)
		}
		after.UpdatedAt = t.UpdatedAt
		if reflect.DeepEqual(after, t) {
			delete(edits, id)
		}
	}
	rep.Changed = len(edits)
	if opts.DryRun || len(edits) == 0 {
		return rep, nil
	}
	_, err = a.EditMany(edits)
	return rep, err
}

// reviewMatches reports whether the entry is for the track.
func reviewMatches(e ReviewEntry, t manifest.Track) bool {
	switch e.Section {
	case "ARTISTS":
		return t.Artist == e.Key || t.OriginalArtist == e.Key
	case "ALBUMS":
		return t.Album == e.Key || t.OriginalAlbum == e.Key
	case "TRACKS":
		return t.ID == e.Key
	}
	return false
}
