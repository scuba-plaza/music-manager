package app

import (
	"fmt"
	"strings"
	"time"

	"music-manager/internal/manifest"
	"music-manager/internal/translit"
)

// Edit describes changes to a track. A nil field is left alone.
//
// Title, Artist and Album are the original (possibly non-ASCII) values; their
// ASCII forms follow automatically. A *ASCII field overrides the derived
// form and sticks across later changes of the original; setting it to ""
// goes back to deriving.
type Edit struct {
	Title, Artist, Album                *string
	TitleASCII, ArtistASCII, AlbumASCII *string
	TrackNumber, TrackTotal             *int
	TrimStart, TrimEnd                  *time.Duration
}

// Edit applies e to the track with the given ID and returns the result. A
// track with a title and an artist is identified.
func (a *App) Edit(id string, e Edit) (manifest.Track, error) {
	m, err := a.store.Load()
	if err != nil {
		return manifest.Track{}, err
	}
	t, ok := m.Get(id)
	if !ok {
		return manifest.Track{}, fmt.Errorf("no track with id %q", id)
	}

	t, err = applyEdit(t, e)
	if err != nil {
		return manifest.Track{}, err
	}
	m.Update(t)
	return t, a.store.Save(m)
}

// applyEdit returns t with e applied and its status recomputed. It changes
// nothing if the result would be invalid.
func applyEdit(t manifest.Track, e Edit) (manifest.Track, error) {
	t.Title, t.OriginalTitle = setName(t.Title, t.OriginalTitle, e.Title, e.TitleASCII)
	t.Artist, t.OriginalArtist = setName(t.Artist, t.OriginalArtist, e.Artist, e.ArtistASCII)
	t.Album, t.OriginalAlbum = setName(t.Album, t.OriginalAlbum, e.Album, e.AlbumASCII)
	if e.TrackNumber != nil {
		t.TrackNumber = *e.TrackNumber
	}
	if e.TrackTotal != nil {
		t.TrackTotal = *e.TrackTotal
	}
	if e.TrimStart != nil {
		t.TrimStart = *e.TrimStart
	}
	if e.TrimEnd != nil {
		t.TrimEnd = *e.TrimEnd
	}
	if err := validate(t); err != nil {
		return t, err
	}

	if t.Title != "" && t.Artist != "" {
		t.Status = manifest.Identified
		t.Hint = manifest.Hint{}
	} else {
		t.Status = manifest.Unidentified
	}
	t.UpdatedAt = time.Now().UTC()
	return t, nil
}

// setName returns the new ASCII and original forms of a name field.
func setName(ascii, orig string, newOrig, newASCII *string) (string, string) {
	overridden := ascii != translit.ToASCII(orig)
	if newOrig != nil {
		orig = strings.TrimSpace(*newOrig)
		if !overridden || orig == "" {
			ascii = translit.ToASCII(orig)
		}
	}
	if newASCII != nil {
		if *newASCII == "" {
			ascii = translit.ToASCII(orig)
		} else {
			ascii = strings.TrimSpace(*newASCII)
		}
	}
	return ascii, orig
}

func validate(t manifest.Track) error {
	switch {
	case t.TrimStart < 0 || t.TrimEnd < 0:
		return fmt.Errorf("trims cannot be negative")
	case t.TrimStart+t.TrimEnd >= t.SourceLength:
		return fmt.Errorf("trims (%v + %v) leave nothing of a %v track", t.TrimStart, t.TrimEnd, t.SourceLength.Round(time.Millisecond))
	case t.TrackNumber < 0 || t.TrackTotal < 0:
		return fmt.Errorf("track numbers cannot be negative")
	}
	return nil
}
