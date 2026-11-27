package pocket

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"music-manager/internal/translit"
)

var replacements = map[rune]string{
	'\u2010': "-", '\u2011': "-", '\u2012': "-", '\u2013': "-", '\u2014': "-", '\u2015': "-", '\u2212': "-",
	'\u2018': "'", '\u2019': "'", '\u201a': "'", '\u201b': "'", '\u2032': "'", '\u2039': "'", '\u203a': "'",
	'\u201c': "\"", '\u201d': "\"", '\u201e': "\"", '\u201f': "\"", '\u2033': "\"",
	'\u2022': "\u00b7", '\u2044': "/",
	'\u067e': "\u0628", '\u0686': "\u062c", '\u0698': "\u0632", '\u06a9': "\u0643", '\u06af': "\u0643",
	'\u06cc': "\u064a", '\u06c0': "\u0647", '\u06c1': "\u0647", '\u06d5': "\u0647", '\u0671': "\u0627",
	'\u06f0': "\u0660", '\u06f1': "\u0661", '\u06f2': "\u0662", '\u06f3': "\u0663", '\u06f4': "\u0664",
	'\u06f5': "\u0665", '\u06f6': "\u0666", '\u06f7': "\u0667", '\u06f8': "\u0668", '\u06f9': "\u0669",
}

func drawable(r rune) bool {
	switch {
	case r >= 0x20 && r <= 0x7e:
		return true
	case r >= 0xa1 && r <= 0x17f:
		return r != 0xad
	case r == 0x60c || r == 0x61b || r == 0x61f:
		return true
	case r >= 0x621 && r <= 0x63a, r >= 0x640 && r <= 0x64a, r >= 0x660 && r <= 0x669:
		return true
	}
	return false
}

func arabicBlock(r rune) bool {
	return (r >= 0x600 && r <= 0x6ff) || (r >= 0x750 && r <= 0x77f) || (r >= 0x8a0 && r <= 0x8ff) ||
		(r >= 0xfb50 && r <= 0xfdff) || (r >= 0xfe70 && r <= 0xfeff)
}

func clean(s string) string {
	var b strings.Builder
	for _, r := range norm.NFKC.String(s) {
		switch {
		case drawable(r):
			b.WriteRune(r)
		case replacements[r] != "":
			b.WriteString(replacements[r])
		case unicode.IsSpace(r):
			b.WriteByte(' ')
		case unicode.In(r, unicode.Cc, unicode.Cf, unicode.Mn, unicode.Me) || arabicBlock(r):
		default:
			for _, f := range translit.ToASCII(string(r)) {
				if drawable(f) {
					b.WriteRune(f)
				}
			}
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
