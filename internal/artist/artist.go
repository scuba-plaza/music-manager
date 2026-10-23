// Package artist knows how a name that stands for several artists is written.
// A track has one artist, so such a name is cut to its first or pointed out.
package artist

import (
	"regexp"
	"strings"
)

// A separator that says for certain that a tag names several artists: a name
// that merely contains "&" or "," ("Simon & Garfunkel", "Earth, Wind & Fire")
// is a band.
var separator = regexp.MustCompile(`(?i)\x00|;|\s+[(\[]?(?:feat\.|ft\.|featuring)\s+`)

// First is the first artist a tag names: all of it if it has no separator.
func First(s string) string {
	if first := strings.TrimSpace(separator.Split(s, 2)[0]); first != "" {
		return first
	}
	return strings.TrimSpace(s)
}

// several is what makes a name look like more than one artist, though it may
// be one band: a hint to look at it, never a reason to change it.
var several = regexp.MustCompile(`(?i)\x00|;|,\s|\s(?:&|and|x|\+|/|vs\.?|و)\s|\s+[(\[]?(?:feat\.|ft\.|featuring)\s+`)

// LooksLikeSeveral reports whether the name looks like it stands for more than
// one artist.
func LooksLikeSeveral(s string) bool { return several.MatchString(s) }
