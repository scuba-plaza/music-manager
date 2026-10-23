// Package device knows what each kind of place the library is written to
// wants: how its files are named, which tags they carry, what happens to the
// covers and how the playlists look. The library itself knows none of it; a
// sync asks a Target.
package device

import (
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
	// SingleCovers says whether a single gets a cover file of its own.
	SingleCovers() bool
	// Playlist is the file of a playlist, given where each track is written:
	// its path in the target and its content.
	Playlist(p manifest.Playlist, paths map[string]string) (name string, data []byte)
}

// Generic is the default: any folder or device that is not known better.
var Generic Target = today{"generic"}

// Wolfgang is the Wolfgang WMP01 player.
var Wolfgang Target = today{"wolfgang-wmp01"}

// today is what both targets do for now: everything the program did before
// there were targets. The two come apart one concern at a time.
type today struct{ name string }

func (t today) Name() string { return t.name }

func (today) Layout() layout.Rules {
	return layout.Rules{Fold: translit.ToASCII, ComponentMax: 100, StemMax: 22}
}

// Tags carries the ASCII fields in the standard frames and the originals in
// custom ones. A track not yet identified has only its ID.
func (today) Tags(t manifest.Track) mp3.TagSet {
	extra := map[string]string{"MM_ID": t.ID}
	if t.Status != manifest.Identified {
		return mp3.TagSet{Extra: extra}
	}
	for k, v := range map[string]string{
		"ORIGINAL_TITLE":      t.OriginalTitle,
		"ORIGINAL_ARTIST":     t.OriginalArtist,
		"ORIGINAL_ALBUM":      t.OriginalAlbum,
		"SOURCE_URL":          t.SourceURL,
		"MUSICBRAINZ_TRACKID": t.MBRecordingID,
		"MUSICBRAINZ_ALBUMID": t.MBReleaseID,
	} {
		if v != "" {
			extra[k] = v
		}
	}
	artist, album := PlayerTags(t)
	return mp3.TagSet{
		Title: t.Title, Artist: artist, Album: album,
		Genre: t.Genre,
		Track: t.TrackNumber, Total: t.TrackTotal,
		Date:   t.Released,
		Length: t.PlayingLength(),
		Extra:  extra,
	}
}

func (today) Cover(stored []byte) ([]byte, string, error) {
	data, err := coverimg.Normalize(stored)
	return data, "image/jpeg", err
}

func (today) SingleCovers() bool { return true }

// Playlist is the M3U file of a playlist. The player (a Wolfgang WMP01) was
// found to read paths from the root of its card with backslashes, one per
// line, in plain UTF-8 with Windows line endings; slashes, and paths relative
// to the playlist, give an empty playlist.
func (t today) Playlist(p manifest.Playlist, paths map[string]string) (string, []byte) {
	var b strings.Builder
	for _, id := range p.Tracks {
		if path, ok := paths[id]; ok {
			b.WriteString(`\` + strings.ReplaceAll(path, "/", `\`) + "\r\n")
		}
	}
	return layout.PlaylistPath(p.Name, t.Layout()), []byte(b.String())
}
