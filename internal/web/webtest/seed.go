package webtest

import (
	"fmt"
	"os"
	"path/filepath"

	"music-manager/internal/app"
	"music-manager/internal/testmp3"
)

const longPlaylistName = "A rather long playlist name that goes on and on and on for a while"

type seededTrack struct {
	title, artist, album string
	number               int
	titleLatin           string
	hasCover             bool
	frames               int
}

var identifiedTracks = []seededTrack{
	{title: "So What", artist: "Miles Davis", album: "Kind of Blue", number: 1, hasCover: true, frames: 100},
	{title: "Blue in Green", artist: "Miles Davis", album: "Kind of Blue", number: 3, hasCover: true, frames: 120},
	{title: "Giant Steps", artist: "John Coltrane", album: "Giant Steps", number: 1, frames: 140},
	{title: "وطني", artist: "فيروز", album: "Songs", number: 1, titleLatin: "Watani", frames: 160},
	{title: "Feeling Good", artist: "Nina Simone", frames: 180},
}

const unidentifiedFrames = 200

func Seed(dir string) (*app.App, error) {
	a, err := app.Open(dir)
	if err != nil {
		return nil, err
	}
	source, err := os.MkdirTemp("", "mm-seed-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(source)
	ingest := func(name string, audio []byte) (string, error) {
		path := filepath.Join(source, name+".mp3")
		if err := os.WriteFile(path, audio, 0o644); err != nil {
			return "", err
		}
		report, err := a.Ingest([]string{path})
		if err != nil {
			return "", err
		}
		if len(report.Added) != 1 {
			return "", fmt.Errorf("seeding %s: %d duplicates, %d failed", name, len(report.Duplicates), len(report.Failed))
		}
		return report.Added[0].ID, nil
	}

	ids := map[string]string{}
	var covered []string
	for i, s := range identifiedTracks {
		id, err := ingest(fmt.Sprint(i+1), testmp3.Silent(s.frames))
		if err != nil {
			return nil, err
		}
		edit := app.Edit{Title: &s.title, Artist: &s.artist, Album: &s.album, TrackNumber: &s.number}
		if s.titleLatin != "" {
			edit.TitleLatin = &s.titleLatin
		}
		if _, err := a.Edit(id, edit); err != nil {
			return nil, err
		}
		if s.hasCover {
			covered = append(covered, id)
		}
		ids[s.title] = id
	}
	if _, err := a.SetCoverMany(covered, testmp3.JPEG); err != nil {
		return nil, err
	}

	oldTags := testmp3.WithTagsTrack(testmp3.Silent(unidentifiedFrames), "Rough Mix", "Basement Tapes", "Demos", "7")
	if _, err := ingest("rough-mix", oldTags); err != nil {
		return nil, err
	}

	if err := a.CreatePlaylist("Late night", ids["Blue in Green"]); err != nil {
		return nil, err
	}
	if err := a.CreatePlaylist(longPlaylistName, ids["Giant Steps"], ids["وطني"]); err != nil {
		return nil, err
	}
	return a, nil
}
