package app

import "music-manager/internal/manifest"

// Display returns the artist, title and album to show for t: its own when
// identified, what the ingested file said about itself otherwise.
func Display(t manifest.Track) (artist, title, album string) {
	if t.Status == manifest.Identified {
		return t.Artist, t.Title, t.Album
	}
	title = t.Hint.Title
	if title == "" {
		title = t.Hint.Filename
	}
	return t.Hint.Artist, title, t.Hint.Album
}
