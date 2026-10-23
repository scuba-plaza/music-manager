package manifest

import "music-manager/internal/translit"

// The Latin form of a name is the one set by hand, or, where there is none,
// the one worked out from the name. A name that is empty has none.
func latinOf(set, truth string) string {
	switch {
	case set != "":
		return set
	case truth == "":
		return ""
	}
	return translit.ToASCII(truth)
}

// LatinTitle is the title in Latin letters.
func (t Track) LatinTitle() string { return latinOf(t.TitleLatin, t.Title) }

// LatinArtist is the artist in Latin letters.
func (t Track) LatinArtist() string { return latinOf(t.ArtistLatin, t.Artist) }

// LatinAlbum is the album in Latin letters.
func (t Track) LatinAlbum() string { return latinOf(t.AlbumLatin, t.Album) }

// LatinGenre is the genre in Latin letters.
func (t Track) LatinGenre() string { return latinOf(t.GenreLatin, t.Genre) }

// LatinName is the name of the playlist in Latin letters.
func (p Playlist) LatinName() string { return latinOf(p.NameLatin, p.Name) }
