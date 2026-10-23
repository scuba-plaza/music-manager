package device

import (
	"strings"
	"unicode/utf8"

	"music-manager/internal/layout"
	"music-manager/internal/manifest"
)

// PlayerFieldWidth is how many characters of an artist or an album the
// player's index keeps; it cuts the rest, wherever that falls.
const PlayerFieldWidth = 12

// PlayerName is the artist or album name as the player shows it: the override
// if there is one, else full, shortened to fit PlayerFieldWidth. A name that
// fits is left alone; a longer one loses what is in parentheses and a leading
// "The ", then is cut after a whole word if that keeps at least half of it.
func PlayerName(full, override string) string {
	if o := strings.TrimSpace(override); o != "" {
		return o
	}
	full = strings.TrimSpace(full)
	if utf8.RuneCountInString(full) > PlayerFieldWidth && len(full) > 4 && strings.EqualFold(full[:4], "the ") {
		full = strings.TrimSpace(full[4:])
	}
	return layout.Shorten(full, PlayerFieldWidth)
}

// PlayerTags are the artist and album the track's file carries, which is what
// the player shows: a single has no album, so its title stands in for it
// (otherwise the player says "Unknown Album").
func PlayerTags(t manifest.Track) (artist, album string) {
	album = t.Album
	if album == "" {
		album = t.Title
	}
	return PlayerName(t.Artist, t.PlayerArtist), PlayerName(album, t.PlayerAlbum)
}
