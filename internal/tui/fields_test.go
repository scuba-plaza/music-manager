package tui

import (
	"strings"
	"testing"
	"time"

	"music-manager/internal/device"
	"music-manager/internal/layout"
	"music-manager/internal/manifest"
)

func identified() manifest.Track {
	return manifest.Track{
		ID: "k7f2qd", Status: manifest.Identified,
		OriginalTitle: "وطني", OriginalArtist: "فيروز", OriginalAlbum: "Hits",
		TrackNumber: 3, TrackTotal: 12,
		SourceLength: 3 * time.Minute, TrimStart: 2 * time.Second, TrimEnd: 1500 * time.Millisecond,
	}
}

func TestFieldsAreInTheOrderOfTheInspector(t *testing.T) {
	want := []fieldKind{fTitle, fArtist, fAlbum, fTrack, fReleased, fGenre, fCover, fCutStart, fCutEnd, fVolume, fFadeIn, fFadeOut}
	if len(trackFields) != len(want) {
		t.Fatalf("fields = %v", trackFields)
	}
	for i := range want {
		if trackFields[i] != want[i] {
			t.Errorf("field %d = %v, want %v", i, trackFields[i], want[i])
		}
	}
	if fTitle.label() != "Title" || fCutStart.label() != "Cut start" || fTrack.label() != "Track" || fReleased.label() != "Released" {
		t.Error("labels")
	}
	if !fTitle.isName() || !fArtist.isName() || !fAlbum.isName() || fTrack.isName() || fCutEnd.isName() {
		t.Error("only title, artist and album are names")
	}
}

func TestANewDraftStartsFromTheTracksCurrentValues(t *testing.T) {
	tr := identified()
	cases := []struct {
		f            fieldKind
		text, second string
	}{
		{fTitle, "وطني", ""},
		{fArtist, "فيروز", ""},
		{fAlbum, "Hits", ""},
		{fTrack, "3", "12"},
		{fCutStart, "0:02", ""},
		{fCutEnd, "0:01.5", ""},
	}
	for _, c := range cases {
		d := newDraft(tr, c.f)
		if d.text != c.text || d.second != c.second {
			t.Errorf("%s: draft = %q / %q, want %q / %q", c.f.label(), d.text, d.second, c.text, c.second)
		}
	}
	tr.TrackNumber, tr.TrackTotal, tr.TrimStart = 0, 0, 0
	if d := newDraft(tr, fTrack); d.text != "" || d.second != "" {
		t.Errorf("unknown numbers should start empty: %q / %q", d.text, d.second)
	}
	if d := newDraft(tr, fCutStart); d.text != "" {
		t.Errorf("no cut should start empty, got %q", d.text)
	}
}

func TestADraftOfAnUnidentifiedTrackStartsFromWhatTheFileSaid(t *testing.T) {
	tr := manifest.Track{ID: "u", Status: manifest.Unidentified, Hint: manifest.Hint{Title: "Bahlam Bek", Artist: "Abdel Halim", Album: "Rawaeaa", Track: 4, Total: 7, Filename: "04 Bahlam Bek"}}
	for f, want := range map[fieldKind]string{fTitle: "Bahlam Bek", fArtist: "Abdel Halim", fAlbum: "Rawaeaa", fTrack: "4"} {
		if d := newDraft(tr, f); d.text != want {
			t.Errorf("%s starts as %q, want %q", f.label(), d.text, want)
		}
	}
	if d := newDraft(tr, fTrack); d.second != "7" {
		t.Errorf("total = %q", d.second)
	}
	tr.Hint.Title = ""
	if d := newDraft(tr, fTitle); d.text != "Bahlam Bek" {
		t.Errorf("without a tag the file name should do, got %q", d.text)
	}
}

func TestEditBuildsWhatTheAppNeeds(t *testing.T) {
	tr := identified()
	d := newDraft(tr, fTitle)
	d.text = "  Watani "
	e, err := d.edit()
	if err != nil || e.Title == nil || *e.Title != "Watani" {
		t.Fatalf("title: %+v, %v", e, err)
	}

	a := newDraft(tr, fAlbum)
	a.text = ""
	if e, err := a.edit(); err != nil || e.Album == nil || *e.Album != "" {
		t.Errorf("an empty album is a single: %+v, %v", e, err)
	}
}

func TestEditRefusesWhatWouldBreakTheTrack(t *testing.T) {
	tr := identified()
	for _, c := range []struct {
		f        fieldKind
		text, of string
		wantErr  string
	}{
		{fTitle, "  ", "", "Title"},
		{fArtist, "", "", "Artist"},
		{fTrack, "abc", "", "whole number"},
		{fTrack, "-3", "", "whole number"},
		{fTrack, "3", "x", "whole number"},
		{fCutStart, "1:99", "", "below 60"},
		{fCutEnd, "-5", "", "invalid"},
		{fCutEnd, "soon", "", "invalid"},
	} {
		d := newDraft(tr, c.f)
		d.text = c.text
		if c.f == fTrack {
			d.second = c.of
		}
		_, err := d.edit()
		if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("%s %q/%q: err = %v, want one mentioning %q", c.f.label(), c.text, c.of, err, c.wantErr)
		}
	}
}

func TestEditParsesNumbersAndTimecodes(t *testing.T) {
	tr := identified()
	d := newDraft(tr, fTrack)
	d.text, d.second = "5", "9"
	e, err := d.edit()
	if err != nil || *e.TrackNumber != 5 || *e.TrackTotal != 9 {
		t.Errorf("%+v, %v", e, err)
	}
	d.text, d.second = "", ""
	if e, _ := d.edit(); *e.TrackNumber != 0 || *e.TrackTotal != 0 {
		t.Errorf("empty means unknown: %+v", e)
	}

	c := newDraft(tr, fCutStart)
	c.text = "1:30.5"
	if e, err := c.edit(); err != nil || *e.TrimStart != 90*time.Second+500*time.Millisecond {
		t.Errorf("%+v, %v", e, err)
	}
	c.text = ""
	if e, _ := c.edit(); *e.TrimStart != 0 {
		t.Errorf("empty means no cut: %+v", e)
	}
	end := newDraft(tr, fCutEnd)
	end.text = "0:03"
	if e, _ := end.edit(); e.TrimEnd == nil || *e.TrimEnd != 3*time.Second || e.TrimStart != nil {
		t.Errorf("only the end cut should be set: %+v", e)
	}
}

func TestPreviewShowsWhereTheFileWillGo(t *testing.T) {
	tr := identified()
	base := pathFor(tr)
	if base != "فيروز/Hits/03 - وطني.mp3" {
		t.Fatalf("setup: %q", base)
	}

	title := newDraft(tr, fTitle)
	title.text = "Watani"
	if got := pathFor(previewTrack(tr, title)); got != "فيروز/Hits/03 - Watani.mp3" {
		t.Errorf("title preview = %q", got)
	}

	album := newDraft(tr, fAlbum)
	album.text = ""
	if got := pathFor(previewTrack(tr, album)); got != "فيروز/وطني.mp3" {
		t.Errorf("a single = %q", got)
	}

	num := newDraft(tr, fTrack)
	num.text, num.second = "7", "120"
	if got := pathFor(previewTrack(tr, num)); got != "فيروز/Hits/007 - وطني.mp3" {
		t.Errorf("numbered = %q", got)
	}

	artist := newDraft(tr, fArtist)
	artist.text = "عبد الحليم"
	if got := pathFor(previewTrack(tr, artist)); got != "عبد الحليم/Hits/03 - وطني.mp3" {
		t.Errorf("artist preview = %q", got)
	}

	cut := newDraft(tr, fCutStart)
	cut.text = "0:10"
	if pathFor(previewTrack(tr, cut)) != base {
		t.Error("a cut does not change the path")
	}
}

func TestPreviewOfAnUnidentifiedTrackBecomesIdentifiedOnceItHasATitleAndArtist(t *testing.T) {
	tr := manifest.Track{ID: "abc123", Status: manifest.Unidentified, Hint: manifest.Hint{Title: "Song", Artist: "Band", Filename: "x"}}
	title := newDraft(tr, fTitle)
	if got := pathFor(previewTrack(tr, title)); got != "_Inbox/abc123 - x.mp3" {
		t.Errorf("with only a title it is still in the inbox, got %q", got)
	}
	tr.OriginalArtist = "Band"
	if got := pathFor(previewTrack(tr, title)); got != "Band/Song.mp3" {
		t.Errorf("with both it gets a place, got %q", got)
	}
}

func TestDisplayValues(t *testing.T) {
	tr := identified()
	if got := displayValue(tr, fTrack); got != "3 of 12" {
		t.Errorf("= %q", got)
	}
	tr.TrackTotal = 0
	if got := displayValue(tr, fTrack); got != "3" {
		t.Errorf("= %q", got)
	}
	tr.TrackNumber = 0
	if got := displayValue(tr, fTrack); got != "" {
		t.Errorf("= %q", got)
	}
	if got := displayValue(identified(), fCutEnd); got != "0:01.5" {
		t.Errorf("= %q", got)
	}
	if got := displayValue(manifest.Track{}, fCutEnd); got != "0:00" {
		t.Errorf("no cut should read 0:00, got %q", got)
	}
}

func TestTheReleaseDateIsShownEditedAndChecked(t *testing.T) {
	tr := identified()
	tr.Released = "1999-05"
	if got := displayValue(tr, fReleased); got != "1999-05" {
		t.Errorf("shown as %q", got)
	}
	d := newDraft(tr, fReleased)
	if d.text != "1999-05" || d.second != "" {
		t.Errorf("draft = %q / %q", d.text, d.second)
	}

	d.text = " 2001-02-03 "
	e, err := d.edit()
	if err != nil || e.Released == nil || *e.Released != "2001-02-03" || e.Title != nil || e.TrackNumber != nil {
		t.Errorf("edit = %+v, %v", e, err)
	}
	d.text = ""
	if e, err := d.edit(); err != nil || e.Released == nil || *e.Released != "" {
		t.Errorf("an empty date forgets the date: %+v, %v", e, err)
	}
	d.text = "soon"
	if _, err := d.edit(); err == nil || !strings.Contains(err.Error(), "1999-05-17") {
		t.Errorf("a bad date: %v; want a message showing the format", err)
	}
}

func TestATrackNobodyIdentifiedOffersTheDateOfItsOldTags(t *testing.T) {
	tr := manifest.Track{ID: "x", Hint: manifest.Hint{Title: "Song", Date: "1987"}}
	if got := displayValue(tr, fReleased); got != "1987" || !prefilledFrom(tr, fReleased) {
		t.Errorf("shown %q, prefilled %v", got, prefilledFrom(tr, fReleased))
	}
	if d := newDraft(tr, fReleased); d.text != "1987" {
		t.Errorf("draft = %q", d.text)
	}
	tr.Released = "1990"
	if got := displayValue(tr, fReleased); got != "1990" || prefilledFrom(tr, fReleased) {
		t.Errorf("a date of its own wins: shown %q", got)
	}
}

// pathFor is where the library folder puts a track.
func pathFor(t manifest.Track) string { return layout.PathFor(t, device.Generic.Layout()) }
