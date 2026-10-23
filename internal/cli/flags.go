package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/pflag"

	"music-manager/internal/app"
	"music-manager/internal/timecode"
)

// trackFlags are the flags that describe a track, shared by add and edit.
type trackFlags struct {
	title, artist, album                            string
	track, of                                       int
	trimStart, trimEnd                              string
	released                                        string
	genre                                           string
	titleLatin, artistLatin, albumLatin, genreLatin string
	sourceURL                                       string
	cover                                           string
}

func (tf *trackFlags) register(f *pflag.FlagSet) {
	f.StringVar(&tf.title, "title", "", "track title")
	f.StringVar(&tf.artist, "artist", "", "artist")
	f.StringVar(&tf.album, "album", "", "album (empty for a single)")
	f.IntVar(&tf.track, "track", 0, "track number")
	f.IntVar(&tf.of, "of", 0, "number of tracks on the album")
	f.StringVar(&tf.released, "released", "", "when the track came out: 1999, 1999-05 or 1999-05-17; \"\" for unknown")
	f.StringVar(&tf.genre, "genre", "", "genre, free text such as Folk or Neoclassical; \"\" for none")
	f.StringVar(&tf.trimStart, "trim-start", "", "cut this much off the start, e.g. 0:12")
	f.StringVar(&tf.trimEnd, "trim-end", "", "cut this much off the end, e.g. 0:05.5")
	f.StringVar(&tf.cover, "cover", "", "cover image file (JPEG or PNG); \"\" means no cover")
}

// registerLatin adds the flags for the Latin forms of the names, which only edit has.
func (tf *trackFlags) registerLatin(f *pflag.FlagSet) {
	f.StringVar(&tf.titleLatin, "title-latin", "", "the title in Latin letters, where the worked-out one is not good enough; \"\" for the worked-out one")
	f.StringVar(&tf.artistLatin, "artist-latin", "", "the artist in Latin letters; \"\" for the worked-out one")
	f.StringVar(&tf.albumLatin, "album-latin", "", "the album in Latin letters; \"\" for the worked-out one")
	f.StringVar(&tf.genreLatin, "genre-latin", "", "the genre in Latin letters; \"\" for the worked-out one")
}

// parsed is what the user put on the command line.
type parsed struct {
	edit         app.Edit
	cover        []byte
	coverChanged bool
}

// build turns the flags that were actually passed into an app.Edit.
func (tf *trackFlags) build(f *pflag.FlagSet) (p parsed, err error) {
	str := func(name string, v *string, dst **string) {
		if f.Changed(name) {
			*dst = v
		}
	}
	e := &p.edit
	str("title", &tf.title, &e.Title)
	str("artist", &tf.artist, &e.Artist)
	str("album", &tf.album, &e.Album)
	if f.Changed("track") {
		e.TrackNumber = &tf.track
	}
	if f.Changed("of") {
		e.TrackTotal = &tf.of
	}
	str("released", &tf.released, &e.Released)
	str("genre", &tf.genre, &e.Genre)
	str("title-latin", &tf.titleLatin, &e.TitleLatin)
	str("artist-latin", &tf.artistLatin, &e.ArtistLatin)
	str("album-latin", &tf.albumLatin, &e.AlbumLatin)
	str("genre-latin", &tf.genreLatin, &e.GenreLatin)
	str("source-url", &tf.sourceURL, &e.SourceURL)
	if e.TrimStart, err = parseTrim(f, "trim-start", tf.trimStart); err != nil {
		return p, err
	}
	if e.TrimEnd, err = parseTrim(f, "trim-end", tf.trimEnd); err != nil {
		return p, err
	}
	if p.coverChanged = f.Changed("cover"); p.coverChanged && tf.cover != "" {
		if p.cover, err = os.ReadFile(tf.cover); err != nil {
			return p, fmt.Errorf("--cover: %w", err)
		}
	}
	return p, nil
}

// parseTrim returns the duration given in flag name, or nil if it was not set.
func parseTrim(f *pflag.FlagSet, name, value string) (*time.Duration, error) {
	if !f.Changed(name) {
		return nil, nil
	}
	d, err := timecode.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("--%s: %w", name, err)
	}
	return &d, nil
}
