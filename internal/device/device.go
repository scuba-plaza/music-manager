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

// Generic is the default: any folder or device that is not known better. It
// gets the names as they are, in any script.
var Generic Target = generic{}

// Wolfgang is the Wolfgang WMP01 player: Latin names of a width it can show.
var Wolfgang Target = wolfgang{}

// plain is what the targets still do alike: the cover and the playlist, which
// come apart in their own slices.
type plain struct{}

func (plain) Cover(stored []byte) ([]byte, string, error) {
	data, err := coverimg.Normalize(stored)
	return data, "image/jpeg", err
}

func (plain) SingleCovers() bool { return true }

// playlist is the M3U file of a playlist. The player (a Wolfgang WMP01) was
// found to read paths from the root of its card with backslashes, one per
// line, in plain UTF-8 with Windows line endings; slashes, and paths relative
// to the playlist, give an empty playlist.
func playlist(p manifest.Playlist, paths map[string]string, r layout.Rules) (string, []byte) {
	var b strings.Builder
	for _, id := range p.Tracks {
		if path, ok := paths[id]; ok {
			b.WriteString(`\` + strings.ReplaceAll(path, "/", `\`) + "\r\n")
		}
	}
	return layout.PlaylistPath(p.Name, r), []byte(b.String())
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

type generic struct{ plain }

func (generic) Name() string { return "generic" }

func (generic) Layout() layout.Rules { return layout.Rules{ComponentMax: 200} }

// Tags are the true names; a single has no album.
func (generic) Tags(t manifest.Track) mp3.TagSet {
	if t.Status != manifest.Identified {
		return mp3.TagSet{Extra: extras(t)}
	}
	return mp3.TagSet{
		Title: t.OriginalTitle, Artist: t.OriginalArtist, Album: t.OriginalAlbum,
		Genre: t.Genre,
		Track: t.TrackNumber, Total: t.TrackTotal,
		Date:   t.Released,
		Length: t.PlayingLength(),
		Extra:  extras(t),
	}
}

func (g generic) Playlist(p manifest.Playlist, paths map[string]string) (string, []byte) {
	return playlist(p, paths, g.Layout())
}

type wolfgang struct{ plain }

func (wolfgang) Name() string { return "wolfgang-wmp01" }

func (wolfgang) Layout() layout.Rules {
	return layout.Rules{Fold: translit.ToASCII, ComponentMax: 100, StemMax: 22}
}

// Tags keep the true title, which the player shows nowhere; the artist and
// album are the ones it shows, in Latin letters of the width it keeps.
func (wolfgang) Tags(t manifest.Track) mp3.TagSet {
	if t.Status != manifest.Identified {
		return mp3.TagSet{Extra: extras(t)}
	}
	artist, album := PlayerTags(t)
	return mp3.TagSet{
		Title: t.OriginalTitle, Artist: artist, Album: album,
		Genre: t.Genre,
		Track: t.TrackNumber, Total: t.TrackTotal,
		Date:   t.Released,
		Length: t.PlayingLength(),
		Extra:  extras(t),
	}
}

func (w wolfgang) Playlist(p manifest.Playlist, paths map[string]string) (string, []byte) {
	return playlist(p, paths, w.Layout())
}
