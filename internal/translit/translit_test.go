package translit_test

import (
	"testing"
	"unicode"

	"music-manager/internal/translit"
)

func TestToASCII(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"ascii passes through", "Max Richter", "Max Richter"},
		{"latin diacritics are stripped", "Fyodor Dostoévsky – Crème Brûlée", "Fyodor Dostoevsky - Creme Brulee"},
		{"non-decomposing latin letters", "Bjørnstjerne Bjørnson, Ærø, Straße, Łódź, Œuvre", "Bjornstjerne Bjornson, AEro, Strasse, Lodz, OEuvre"},
		{"arabic waw and ya between consonants are long vowels", "فيروز", "Firuz"},
		{"arabic ya before alif is a consonant", "جوليا بطرس", "Julya Btrs"},
		{"arabic waw at word start is a consonant", "وطني", "Wtni"},
		{"arabic article and ta marbuta", "ماجدة الرومي", "Majda al-Rumi"},
		{"arabic ayn capitalises the following letter", "عبد الحليم حافظ", "'Bd al-Hlim Hafz"},
		{"arabic harakat and shadda", "مُحَمَّد", "Muhammad"},
		{"arabic long vowel after matching haraka is not doubled", "وَطَنِي", "Watani"},
		{"arabic hamza seats", "أنا أفريقي أنا سوداني", "Ana Afriqi Ana Sudani"},
		{"arabic hamza seats mid-word", "سؤال إيمان قرآن مسائل", "S'al Iman Qr'an Msa'l"},
		{"arabic digits and punctuation in mixed text", "Track ١٢، فيروز؟", "Track 12, Firuz?"},
		{"whitespace is trimmed and collapsed", "  Max \t Richter  ", "Max Richter"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := translit.ToASCII(c.in); got != c.want {
				t.Errorf("ToASCII(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestToASCIIOutputIsPrintableASCII(t *testing.T) {
	inputs := []string{
		"Фёдор Достоевский", "坂本龍一", "Björk 🎵", "ـــمـــ", "\x00\x7f‏", "ﷲ", "ﻻ",
	}
	for _, in := range inputs {
		got := translit.ToASCII(in)
		for _, r := range got {
			if r > unicode.MaxASCII || !unicode.IsPrint(r) {
				t.Errorf("ToASCII(%q) = %q, contains %U", in, got, r)
				break
			}
		}
	}
}
