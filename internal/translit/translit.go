// Package translit converts arbitrary Unicode text to a readable ASCII form.
package translit

import (
	"strings"
	"unicode"

	"github.com/mozillazg/go-unidecode"
	"golang.org/x/text/unicode/norm"
)

// ToASCII returns an ASCII rendering of s. Runs of Arabic letters are
// romanised word by word; everything else is decomposed and stripped of
// combining marks, with unidecode as the fallback.
func ToASCII(s string) string {
	var b strings.Builder
	rs := []rune(norm.NFC.String(s))
	for i := 0; i < len(rs); {
		j := i
		if isArabic(rs[i]) {
			for j < len(rs) && isArabic(rs[j]) {
				j++
			}
			b.WriteString(arabicWord(rs[i:j]))
		} else {
			for j < len(rs) && !isArabic(rs[j]) {
				j++
			}
			b.WriteString(latin(string(rs[i:j])))
		}
		i = j
	}
	printable := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || !unicode.IsPrint(r) {
			return ' '
		}
		return r
	}, b.String())
	return strings.Join(strings.Fields(printable), " ")
}

func latin(s string) string {
	var b strings.Builder
	for _, r := range norm.NFKD.String(s) {
		switch {
		case r < unicode.MaxASCII:
			b.WriteRune(r)
		case unicode.Is(unicode.Mn, r):
			// combining mark left behind by decomposition: drop it
		default:
			b.WriteString(unidecode.Unidecode(string(r)))
		}
	}
	return b.String()
}

func isArabic(r rune) bool {
	// harakat are in the Inherited script, not Arabic
	return unicode.Is(unicode.Arabic, r) && unicode.IsLetter(r) || isMark(r)
}

const shadda = 'ّ'

// harakat maps short-vowel marks to their vowel. Sukun is silent.
var harakat = map[rune]string{
	'َ': "a", 'ِ': "i", 'ُ': "u", 'ْ': "",
	'ً': "an", 'ٍ': "in", 'ٌ': "un",
}

func isMark(r rune) bool { _, ok := harakat[r]; return ok || r == shadda }

var consonants = map[rune]string{
	'ب': "b", 'ت': "t", 'ث': "th", 'ج': "j", 'ح': "h", 'خ': "kh",
	'د': "d", 'ذ': "dh", 'ر': "r", 'ز': "z", 'س': "s", 'ش': "sh",
	'ص': "s", 'ض': "d", 'ط': "t", 'ظ': "z", 'ع': "'", 'غ': "gh",
	'ف': "f", 'ق': "q", 'ك': "k", 'ل': "l", 'م': "m", 'ن': "n",
	'ه': "h", 'ء': "'",
}

// vowelLetters are always read as the vowel a.
var vowelLetters = map[rune]string{'ا': "a", 'ى': "a", 'ة': "a"}

// hamzaSeats are read as a vowel at a word start and as a glottal stop
// elsewhere.
var hamzaSeats = map[rune][2]string{
	'أ': {"a", "'"}, 'إ': {"i", "'"}, 'آ': {"a", "'a"}, 'ؤ': {"u", "'"}, 'ئ': {"i", "'"},
}

// semivowels read as a consonant at a word start or before a vowel, and
// as a long vowel otherwise.
var semivowels = map[rune][2]string{'و': {"w", "u"}, 'ي': {"y", "i"}}

func arabicWord(w []rune) string {
	if len(w) > 2 && w[0] == 'ا' && w[1] == 'ل' {
		return "al-" + arabicWord(w[2:])
	}
	var b strings.Builder
	prevVowel := "" // vowel sound the previous letter ended on
	for i := 0; i < len(w); {
		r := w[i]
		i++
		doubled, vowel, hasVowel := false, "", false
		for ; i < len(w) && isMark(w[i]); i++ {
			if w[i] == shadda {
				doubled = true
			} else {
				vowel, hasVowel = harakat[w[i]], true
			}
		}
		emit := func(s string) {
			b.WriteString(s)
			if doubled {
				b.WriteString(s)
			}
			b.WriteString(vowel)
		}

		if c, ok := consonants[r]; ok {
			emit(c)
		} else if v, ok := vowelLetters[r]; ok {
			b.WriteString(v)
		} else if h, ok := hamzaSeats[r]; ok {
			if b.Len() == 0 {
				b.WriteString(h[0])
				vowel = h[0]
			} else {
				emit(h[1])
			}
		} else if sv, ok := semivowels[r]; ok {
			nextIsVowel := i < len(w) && vowelLetters[w[i]] != ""
			switch {
			case hasVowel || b.Len() == 0 || nextIsVowel:
				emit(sv[0])
			case prevVowel == sv[1]:
				// long vowel already written by the preceding haraka
			default:
				b.WriteString(sv[1])
			}
		}
		prevVowel = vowel
	}
	return capitalise(b.String())
}

// capitalise upper-cases the first letter, skipping a leading apostrophe.
func capitalise(s string) string {
	i := strings.IndexFunc(s, unicode.IsLetter)
	if i < 0 {
		return s
	}
	return s[:i] + strings.ToUpper(s[i:i+1]) + s[i+1:]
}
