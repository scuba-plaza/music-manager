package app

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"music-manager/internal/manifest"
	"music-manager/internal/mp3"
)

// IngestReport says what happened to each file offered to Ingest.
type IngestReport struct {
	Added      []manifest.Track
	Duplicates []string // paths whose audio was already in the library
	Failed     []IngestFailure
}

type IngestFailure struct {
	Path string
	Err  error
}

// staged is an MP3 file whose audio and cover are in the stores but which
// has no record yet.
type staged struct {
	sha, coverSHA string
	length        time.Duration
	tags          mp3.TagSet
	name          string // the file name without extension
}

// stage reads the MP3 at path and files its bare audio and its artwork
// into the library's stores. The file itself is left alone.
func (a *App) stage(path string) (staged, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return staged{}, err
	}
	audio, length, err := mp3.Parse(data)
	if err != nil {
		return staged{}, err
	}
	sha, err := a.audio.Put(bytes.NewReader(audio))
	if err != nil {
		return staged{}, err
	}
	tags := mp3.ReadTagSet(data)
	coverSHA, _ := a.storeCover(tags.Cover) // a bad cover is not worth losing the track over
	return staged{
		sha: sha, coverSHA: coverSHA, length: length, tags: tags,
		name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
	}, nil
}

// newTrack is the record for a freshly staged file.
func newTrack(s staged, sourceURL string) manifest.Track {
	now := time.Now().UTC()
	return manifest.Track{
		AudioSHA256:  s.sha,
		SourceURL:    sourceURL,
		Status:       manifest.Unidentified,
		SourceLength: s.length,
		CoverSHA256:  s.coverSHA,
		Hint: manifest.Hint{
			Title: s.tags.Title, Artist: s.tags.Artist, Album: s.tags.Album,
			Track: s.tags.Track, Total: s.tags.Total, Date: leadingDate(s.tags.Date), Filename: s.name,
		},
		AddedAt:   now,
		UpdatedAt: now,
	}
}

// Ingest adds the MP3 files at the given files or directories to the
// library as unidentified tracks. The files themselves are left alone.
func (a *App) Ingest(paths []string) (IngestReport, error) {
	var rep IngestReport
	m, err := a.store.Load()
	if err != nil {
		return rep, err
	}
	for _, p := range paths {
		err := filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() && d.Name() == ".mm" {
				return filepath.SkipDir // the library's own store
			}
			if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".mp3") {
				return err
			}
			s, err := a.stage(path)
			if err != nil {
				rep.Failed = append(rep.Failed, IngestFailure{path, err})
				return nil
			}
			if _, dup := m.FindByAudio(s.sha); dup {
				rep.Duplicates = append(rep.Duplicates, path)
				return nil
			}
			rep.Added = append(rep.Added, m.Add(newTrack(s, "")))
			return nil
		})
		if err != nil {
			return rep, err
		}
	}
	return rep, a.store.Save(m)
}
