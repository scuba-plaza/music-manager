package device

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"music-manager/internal/layout"
	"music-manager/internal/manifest"
	"music-manager/internal/translit"
)

// PlayerFieldWidth is how many characters of an artist or an album the
// player's index keeps; it cuts the rest, wherever that falls.
const PlayerFieldWidth = 12

// PlayerTags are the artist and album the track's file carries, which is what
// the player shows: worked out from the Latin forms of the names. A single has
// no album, so its title stands in for it (otherwise the player says "Unknown
// Album").
func PlayerTags(t manifest.Track) (artist, album string) {
	source := t.LatinAlbum()
	if t.Album == "" {
		source = t.LatinTitle()
	}
	return ArtistName(t.LatinArtist()), AlbumName(source, t.LatinArtist())
}

// latinName is a name in Latin letters, as one line of words without an
// apostrophe at the edge of any (what Arabic letters such as ayn and hamza
// leave there).
func latinName(s string) string {
	var words []string
	for _, w := range strings.Fields(translit.ToASCII(s)) {
		if w = strings.Trim(w, "'"); w != "" {
			words = append(words, w)
		}
	}
	return strings.Join(words, " ")
}

func fits(s string) bool { return utf8.RuneCountInString(s) <= PlayerFieldWidth }

var (
	leadingThe    = regexp.MustCompile(`(?i)^the\s+`)
	parenthetical = regexp.MustCompile(`\s*[(\[][^()\[\]]*[)\]]`)
	trailingEP    = regexp.MustCompile(`(?i)(\s+-)?\s+EP$|\s+-\s+Single$`)
)

// unadorned is s without a leading "The " and what is in parentheses or
// brackets, as long as something is left.
func unadorned(s string) string {
	t := strings.TrimSpace(parenthetical.ReplaceAllString(s, ""))
	t = leadingThe.ReplaceAllString(t, "")
	if t == "" {
		return s
	}
	return t
}

// particles are the words that belong to the surname they come before.
var particles = map[string]bool{
	"al": true, "el": true, "abu": true, "bin": true, "ibn": true,
	"de": true, "di": true, "da": true, "van": true, "von": true, "der": true, "le": true, "la": true,
}

// ArtistName is the artist as the Wolfgang shows it: the name in Latin
// letters, in PlayerFieldWidth characters. The first of these that fits
// wins: the name; without a leading "The" and what is in parentheses;
// "Various Artists" as "Various"; for two words or more, the initials of the
// given names and the surname ("M. A. Wahab"), the first initial and the
// surname, the surname alone, the last word alone; the name shortened at a
// word; the name cut.
func ArtistName(name string) string {
	name = latinName(name)
	if fits(name) {
		return name
	}
	if name = unadorned(name); fits(name) {
		return name
	}
	if strings.EqualFold(name, "Various Artists") {
		return "Various"
	}
	if words := strings.Fields(name); len(words) >= 2 {
		cut := len(words) - 1 // where the surname starts: the last word and the particles before it
		for cut > 0 && particles[strings.ToLower(words[cut-1])] {
			cut--
		}
		surname, given := strings.Join(words[cut:], " "), words[:cut]
		var initials []string
		for _, w := range given {
			if i := initial(w); i != "" {
				initials = append(initials, i)
			}
		}
		candidates := []string{strings.Join(append(initials, surname), " ")}
		if len(initials) > 0 {
			candidates = append(candidates, initials[0]+" "+surname)
		}
		candidates = append(candidates, surname, words[len(words)-1])
		for _, c := range candidates {
			if fits(c) {
				return c
			}
		}
	}
	return layout.Shorten(name, PlayerFieldWidth)
}

// initial is the first letter or digit of a word, in capitals, and a dot.
func initial(word string) string {
	for _, r := range word {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return string(unicode.ToUpper(r)) + "."
		}
	}
	return ""
}

// AlbumName is the album as the Wolfgang shows it, in the same way: the name
// in Latin letters, in PlayerFieldWidth characters. The first of these that
// fits wins: the name; without a leading "The", what is in parentheses and a
// trailing "- EP" or "- Single"; without the artist's name, or a word of
// four letters or more from it, at the start; the name shortened at a word;
// the name cut.
func AlbumName(name, artist string) string {
	name, artist = latinName(name), latinName(artist)
	if fits(name) {
		return name
	}
	name = strings.TrimSpace(trailingEP.ReplaceAllString(unadorned(name), ""))
	if fits(name) {
		return name
	}
	if rest := withoutArtist(name, artist); rest != "" {
		if name = rest; fits(name) {
			return name
		}
	}
	return layout.Shorten(name, PlayerFieldWidth)
}

// withoutArtist is name without the artist, or a word of four letters or
// more of the artist, at its start, and the separator after it; "" if it
// does not start with one, or nothing would be left.
func withoutArtist(name, artist string) string {
	candidates := append([]string{artist}, strings.Fields(artist)...)
	lower := strings.ToLower(name)
	for _, c := range candidates {
		if c == "" || utf8.RuneCountInString(c) < 4 && c != artist {
			continue
		}
		if !strings.HasPrefix(lower, strings.ToLower(c)) {
			continue
		}
		rest := name[len(c):]
		if rest != "" && rest[0] != ' ' && rest[0] != '-' {
			continue // a longer word that merely starts the same
		}
		if rest = strings.TrimLeft(rest, " -:"); rest != "" {
			return rest
		}
	}
	return ""
}
