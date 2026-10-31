// Package device knows what each kind of place the library is written to
// wants: how its files are named, which tags they carry, what happens to the
// covers and how the playlists look. The library itself knows none of it; a
// sync asks a Target.
package device

import (
	"music-manager/internal/layout"
	"music-manager/internal/manifest"
	"music-manager/internal/mp3"
)

// Target is a kind of place the library is written to.
type Target interface {
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
	// ReplayGain says whether the files carry their loudness, as ReplayGain tags,
	// for a player that evens out volume.
	ReplayGain() bool
	// PlaylistFile is where the playlist is written in the target: under its true
	// name, or under the Latin form of it where the target cannot show the true one.
	PlaylistFile(p manifest.Playlist) string
	// Playlist is the file of a playlist, given the tracks by ID and where each
	// is written: its path in the target and its content.
	Playlist(p manifest.Playlist, tracks map[string]manifest.Track, paths map[string]string) (name string, data []byte)
	// Diagnose says what the target will not do well for these tracks.
	Diagnose(tracks []manifest.Track) []Warning
	Profile() Profile
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
