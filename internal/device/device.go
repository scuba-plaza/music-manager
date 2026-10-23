// Package device knows what each kind of place the library is written to
// wants: how its files are named, which tags they carry, what happens to the
// covers and how the playlists look. The library itself knows none of it; a
// sync asks a Target.
package device

import (
	"errors"
	"fmt"
	"math"
	"path"
	"strings"

	"music-manager/internal/coverimg"
	"music-manager/internal/layout"
	"music-manager/internal/manifest"
	"music-manager/internal/mp3"
	"music-manager/internal/translit"
)

// Target is a kind of place the library is written to.
type Target interface {
	// Name is how the target is called on the command line and in messages.
	Name() string
	// Layout says how paths are made.
	Layout() layout.Rules
	// Tags is the complete tag of the track's file, except for its cover.
	Tags(t manifest.Track) mp3.TagSet
	// Cover turns a stored cover into the picture that is embedded and written
	// beside the tracks, and says what kind of image it is.
	Cover(stored []byte) (data []byte, mime string, err error)
	// CoverFile is the name of the cover file of an album folder, for a stored
	// cover of this kind ("image/jpeg" or "image/png").
	CoverFile(storedMIME string) string
	// SingleCovers says whether a single gets a cover file of its own.
	SingleCovers() bool
	// PlaylistFile is where the playlist of this name is written in the target.
	PlaylistFile(name string) string
	// Playlist is the file of a playlist, given the tracks by ID and where each
	// is written: its path in the target and its content.
	Playlist(p manifest.Playlist, tracks map[string]manifest.Track, paths map[string]string) (name string, data []byte)
	// Diagnose says what the target will not do well for these tracks.
	Diagnose(tracks []manifest.Track) []Warning
}

// Generic is the default: any folder or device that is not known better. It
// gets the names as they are, in any script.
var Generic Target = generic{}

// stemWidth is how many characters of a file name the Wolfgang shows.
const stemWidth = 24

// Wolfgang is the Wolfgang WMP01 player: Latin names of a width it can show.
var Wolfgang Target = wolfgang{}

// wolfgangPlaylist is the M3U file of a playlist for the Wolfgang WMP01. The
// player was found to read paths from the root of its card with backslashes,
// one per line, in plain UTF-8 with Windows line endings; slashes, and paths
// relative to the playlist, give an empty playlist.
func wolfgangPlaylist(p manifest.Playlist, paths map[string]string) []byte {
	var b strings.Builder
	for _, id := range p.Tracks {
		if path, ok := paths[id]; ok {
			b.WriteString(`\` + strings.ReplaceAll(path, "/", `\`) + "\r\n")
		}
	}
	return []byte(b.String())
}

// genericPlaylist is the M3U8 file of a playlist: UTF-8, one line per track
// with its length and name, and the paths relative to the playlist, which is
// what players and phones understand.
func genericPlaylist(p manifest.Playlist, tracks map[string]manifest.Track, paths map[string]string) []byte {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	for _, id := range p.Tracks {
		file, ok := paths[id]
		if !ok {
			continue
		}
		t := tracks[id]
		seconds := int(math.Round(t.PlayingLength().Seconds()))
		if seconds <= 0 {
			seconds = -1 // unknown
		}
		name := strings.TrimSuffix(path.Base(file), path.Ext(file))
		if t.Status == manifest.Identified {
			name = t.Artist + " - " + t.Title
		}
		fmt.Fprintf(&b, "#EXTINF:%d,%s\n../%s\n", seconds, name, file)
	}
	return []byte(b.String())
}

// extras are the tag fields that belong to no standard frame. A track not yet
// identified has only its ID.
func extras(t manifest.Track) map[string]string {
	extra := map[string]string{"MM_ID": t.ID}
	if t.Status != manifest.Identified {
		return extra
	}
	for k, v := range map[string]string{
		"SOURCE_URL":          t.SourceURL,
		"MUSICBRAINZ_TRACKID": t.MBRecordingID,
		"MUSICBRAINZ_ALBUMID": t.MBReleaseID,
	} {
		if v != "" {
			extra[k] = v
		}
	}
	return extra
}

type generic struct{}

func (generic) Name() string { return "generic" }

func (generic) Layout() layout.Rules { return layout.Rules{ComponentMax: 200} }

// Tags are the true names; a single has no album.
func (generic) Tags(t manifest.Track) mp3.TagSet {
	if t.Status != manifest.Identified {
		return mp3.TagSet{Extra: extras(t)}
	}
	return mp3.TagSet{
		Title: t.Title, Artist: t.Artist, Album: t.Album,
		Genre: t.Genre,
		Track: t.TrackNumber, Total: t.TrackTotal,
		Date:   t.Released,
		Length: t.PlayingLength(),
		Extra:  extras(t),
	}
}

// Cover is kept as it was given, whatever it is; the file is named by its kind.
func (generic) Cover(stored []byte) ([]byte, string, error) {
	mime := coverimg.Sniff(stored)
	if mime == "" {
		return nil, "", errors.New("cover must be a JPEG or PNG image")
	}
	return stored, mime, nil
}

func (generic) CoverFile(storedMIME string) string {
	if storedMIME == "image/png" {
		return "cover.png"
	}
	return "cover.jpg"
}

// SingleCovers: a single has its cover in its tag, and no file of its own.
func (generic) SingleCovers() bool { return false }

func (g generic) PlaylistFile(name string) string {
	return layout.PlaylistPath(name, ".m3u8", g.Layout())
}

func (g generic) Playlist(p manifest.Playlist, tracks map[string]manifest.Track, paths map[string]string) (string, []byte) {
	return g.PlaylistFile(p.Name), genericPlaylist(p, tracks, paths)
}

type wolfgang struct{}

func (wolfgang) Name() string { return "wolfgang-wmp01" }

// Layout: the player shows the first 24 characters of a file name and stores
// 26 with the extension, so a longer name is cut at 24, where what is shown is
// the end of the name and not the start of ".mp3".
func (wolfgang) Layout() layout.Rules {
	return layout.Rules{Fold: translit.ToASCII, ComponentMax: 100, StemMax: stemWidth}
}

// Tags keep the true title, which the player shows nowhere; the artist and
// album are the ones it shows, in Latin letters of the width it keeps.
func (wolfgang) Tags(t manifest.Track) mp3.TagSet {
	if t.Status != manifest.Identified {
		return mp3.TagSet{Extra: extras(t)}
	}
	artist, album := PlayerTags(t)
	return mp3.TagSet{
		Title: t.Title, Artist: artist, Album: album,
		Genre: t.Genre,
		Track: t.TrackNumber, Total: t.TrackTotal,
		Date:   t.Released,
		Length: t.PlayingLength(),
		Extra:  extras(t),
	}
}

// The Wolfgang's Photos menu opens baseline JFIF files, 4:2:0, up to 480
// pixels, and does not list PNG files: whatever was stored is turned into one.
const (
	coverMaxEdge = 480
	coverQuality = 85
)

func (wolfgang) Cover(stored []byte) ([]byte, string, error) {
	data, err := coverimg.Baseline(stored, coverMaxEdge, coverQuality)
	return data, "image/jpeg", err
}

func (wolfgang) CoverFile(string) string { return "cover.jpg" }

// SingleCovers: the player shows a photo named like the track, so a single gets
// one beside its file.
func (wolfgang) SingleCovers() bool { return true }

func (w wolfgang) PlaylistFile(name string) string {
	return layout.PlaylistPath(name, ".m3u", w.Layout())
}

func (w wolfgang) Playlist(p manifest.Playlist, tracks map[string]manifest.Track, paths map[string]string) (string, []byte) {
	return w.PlaylistFile(p.Name), wolfgangPlaylist(p, paths)
}
