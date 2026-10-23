package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"

	"music-manager/internal/manifest"
)

type Severity int

const (
	Warning Severity = iota // worth knowing, nothing is broken
	Error                   // data is missing or damaged
)

func (s Severity) String() string { return [...]string{"warning", "error"}[s] }

// Problem is one finding of Doctor.
type Problem struct {
	Severity Severity
	TrackID  string
	Message  string
}

// CoverPath is where the stored cover image with the given SHA lives.
func (a *App) CoverPath(sha string) string { return a.covers.Path(sha) }

// Doctor checks the library for damage: every record's audio and cover must
// be in the stores and still match their hashes. It also warns about tracks
// that are not identified yet and about tracks that claim the same place on
// the same album.
func (a *App) Doctor() ([]Problem, error) {
	m, err := a.store.Load()
	if err != nil {
		return nil, err
	}
	var ps []Problem
	add := func(sev Severity, id, format string, args ...any) {
		ps = append(ps, Problem{sev, id, fmt.Sprintf(format, args...)})
	}

	for _, t := range m.Tracks {
		for _, blob := range []struct{ kind, sha, path string }{
			{"audio", t.AudioSHA256, a.audio.Path(t.AudioSHA256)},
			{"cover", t.CoverSHA256, a.covers.Path(t.CoverSHA256)},
		} {
			if blob.sha == "" {
				continue
			}
			switch sum, err := hashFile(blob.path); {
			case errors.Is(err, fs.ErrNotExist):
				add(Error, t.ID, "%s file is missing from the library", blob.kind)
			case err != nil:
				return nil, err
			case sum != blob.sha:
				add(Error, t.ID, "%s file does not match its hash (damaged?)", blob.kind)
			}
		}
		if t.Status != manifest.Identified {
			add(Warning, t.ID, "not identified yet, it is rendered into _Inbox")
		}
	}

	type position struct {
		artist, album string
		track         int
	}
	at := map[position][]string{}
	for _, t := range m.Tracks {
		if t.Status == manifest.Identified && t.AsciiAlbum != "" && t.TrackNumber > 0 {
			k := position{strings.ToLower(t.AsciiArtist), strings.ToLower(t.AsciiAlbum), t.TrackNumber}
			at[k] = append(at[k], t.ID)
		}
	}
	for k, ids := range at {
		if len(ids) > 1 {
			for _, id := range ids {
				add(Warning, id, "same position as %s (track %d of %q by %q)", strings.Join(others(ids, id), ", "), k.track, k.album, k.artist)
			}
		}
	}
	sortProblems(ps)
	return ps, nil
}

func others(ids []string, except string) []string {
	var out []string
	for _, id := range ids {
		if id != except {
			out = append(out, id)
		}
	}
	return out
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sortProblems(ps []Problem) {
	sort.SliceStable(ps, func(i, j int) bool {
		if ps[i].Severity != ps[j].Severity {
			return ps[i].Severity > ps[j].Severity
		}
		return ps[i].TrackID < ps[j].TrackID
	})
}
