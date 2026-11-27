package pocket

import (
	"encoding/binary"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"music-manager/internal/manifest"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func sample() Library {
	track := func(id, title, artist, album string, number int, genre, released, added string, minutes int) manifest.Track {
		return manifest.Track{
			ID: id, Status: manifest.Identified, Title: title, Artist: artist, Album: album,
			TrackNumber: number, TrackTotal: 10, Genre: genre, Released: released,
			AddedAt: day(added), SourceLength: time.Duration(minutes) * time.Minute,
		}
	}
	tracks := []manifest.Track{
		track("t1", "Coming Home", "Leon Bridges", "Coming Home", 1, "Soul", "2015-06-23", "2024-06-03", 3),
		track("t2", "Better Man", "Leon Bridges", "Coming Home", 2, "Soul", "2015", "2024-06-01", 4),
		track("t3", "Babel", "Gustavo Santaolalla", "Babel", 1, "Soundtrack", "2006", "2024-01-10", 2),
		track("t4", "بحلم بيك", "عبد الحليم حافظ", "عبد الحليم حافظ", 5, "طرب", "", "2024-03-12", 5),
		track("t5", "1979", "Leon Bridges", "", 0, "Soul", "2016", "2024-07-01", 1),
	}
	tracks[3].TitleLatin = "Bahlam Bik"
	tracks[3].GenreLatin = "Tarab"
	tracks[3].ArtistLatin = "Abdel Halim Hafez"
	tracks[3].AlbumLatin = "Abdel Halim Hafez"
	paths := map[string]string{}
	for _, t := range tracks {
		paths[t.ID] = "Music/" + t.ID + ".mp3"
	}
	return Library{
		Tracks:    tracks,
		Playlists: []manifest.Playlist{{Name: "Road trip", Tracks: []string{"t3", "gone", "t1"}}},
		Paths:     paths,
	}
}

func u16s(data []byte) []int {
	var out []int
	for i := 0; i+1 < len(data); i += 2 {
		out = append(out, int(binary.LittleEndian.Uint16(data[i:])))
	}
	return out
}

func text(files map[string][]byte, at uint32) string {
	pool := files["PLAYER/STRINGS.BIN"]
	n := uint32(pool[at])
	return string(pool[at+1 : at+1+n])
}

func TestHeaderCountsEveryKindOfRecord(t *testing.T) {
	h := Files(sample())["PLAYER/HEADER.BIN"]
	if string(h[:4]) != "PKPL" {
		t.Fatalf("magic %q", h[:4])
	}
	if got, want := u16s(h[4:]), []int{1, 1, 5, 3, 3, 3, 1}; !slices.Equal(got, want) {
		t.Fatalf("header = %v, want %v", got, want)
	}
}

func TestOrdersFollowTheLatinSpellingAndPutNamesWithoutALetterLast(t *testing.T) {
	files := Files(sample())
	want := map[string][]int{
		"SONGAZ.ORD":   {2, 3, 1, 0, 4},
		"SONGART.ORD":  {3, 2, 0, 1, 4},
		"SONGADD.ORD":  {4, 0, 1, 3, 2},
		"SONGYEAR.ORD": {4, 1, 0, 2, 3},
		"ALBAZ.ORD":    {2, 1, 0},
		"ALBART.ORD":   {2, 1, 0},
		"ALBADD.ORD":   {0, 2, 1},
		"ALBYEAR.ORD":  {0, 1, 2},
		"ARTAZ.ORD":    {2, 1, 0},
		"ARTMOST.ORD":  {0, 2, 1},
		"GENAZ.ORD":    {0, 1, 2},
		"GENMOST.ORD":  {0, 1, 2},
	}
	for name, order := range want {
		if got := u16s(files["PLAYER/"+name]); !slices.Equal(got, order) {
			t.Errorf("%s = %v, want %v", name, got, order)
		}
	}
}

func TestLetterIndexGivesTheFirstPositionOfEachLetterThenTheTotal(t *testing.T) {
	letters := func(starts map[int]int, rest, total int) []int {
		out := make([]int, 28)
		for i := range 27 {
			out[i] = rest
			if v, ok := starts[i]; ok {
				out[i] = v
			}
		}
		out[27] = total
		return out
	}
	files := Files(sample())
	if got, want := u16s(files["PLAYER/SONGAZ.LET"]), letters(map[int]int{0: 0, 1: 0, 2: 3}, 4, 5); !slices.Equal(got, want) {
		t.Errorf("SONGAZ.LET = %v, want %v", got, want)
	}
	if got, want := u16s(files["PLAYER/ALBAZ.LET"]), letters(map[int]int{0: 0, 1: 1, 2: 2}, 3, 3); !slices.Equal(got, want) {
		t.Errorf("ALBAZ.LET = %v, want %v", got, want)
	}
}

type reader struct {
	files map[string][]byte
	data  []byte
	at    int
}

func record(files map[string][]byte, name string, size, i int) *reader {
	return &reader{files: files, data: files["PLAYER/"+name][size*i : size*(i+1)]}
}

func (r *reader) u8() int {
	r.at++
	return int(r.data[r.at-1])
}

func (r *reader) u16() int {
	r.at += 2
	return int(binary.LittleEndian.Uint16(r.data[r.at-2:]))
}

func (r *reader) u32() uint32 {
	r.at += 4
	return binary.LittleEndian.Uint32(r.data[r.at-4:])
}

func (r *reader) str() string { return text(r.files, r.u32()) }

func (r *reader) list() []int {
	at, n := int(r.u32()), r.u16()
	return u16s(r.files["PLAYER/LISTS.BIN"][2*at : 2*(at+n)])
}

func days(s string) int { return int(day(s).Unix() / 86400) }

func TestTrackRecordHoldsTheDisplayTextAndTheFile(t *testing.T) {
	r := record(Files(sample()), "TRACKS.BIN", 24, 3)
	got := []any{r.str(), r.str(), r.u16(), r.u16(), r.u16(), r.u8(), r.u8(), r.u32(), r.u16(), r.u16()}
	want := []any{"بحلم بيك", "Music/t4.mp3", 2, 2, 2, 5, 10, uint32(300000), 0, days("2024-03-12")}
	if !slices.Equal(got, want) {
		t.Fatalf("track = %v, want %v", got, want)
	}
}

func TestGroupRecordsHoldCountsLengthsAndSongLists(t *testing.T) {
	files := Files(sample())
	a := record(files, "ALBUMS.BIN", 24, 0)
	if got, want := []any{a.str(), a.u16(), a.u16(), a.u16(), a.u16(), a.u32(), a.list(), a.u16()}, []any{"Coming Home", 0, 0, 2015, days("2024-06-03"), uint32(420000), []int{0, 1}, 0}; !equal(got, want) {
		t.Errorf("album = %v, want %v", got, want)
	}
	ar := record(files, "ARTISTS.BIN", 24, 0)
	if got, want := []any{ar.str(), ar.u16(), ar.u16(), ar.u32(), ar.list(), ar.u16(), ar.u32()}, []any{"Leon Bridges", 0, 1, uint32(480000), []int{0, 1, 4}, 0, uint32(7)}; !equal(got, want) {
		t.Errorf("artist = %v, want %v", got, want)
	}
	g := record(files, "GENRES.BIN", 20, 0)
	if got, want := []any{g.str(), g.u16(), g.u16(), g.u32(), g.list(), g.u16()}, []any{"Soul", 1, 1, uint32(480000), []int{1, 0, 4}, 0}; !equal(got, want) {
		t.Errorf("genre = %v, want %v", got, want)
	}
	p := record(files, "PLAYLIST.BIN", 16, 0)
	if got, want := []any{p.str(), p.u32(), p.list(), p.u16()}, []any{"Road trip", uint32(300000), []int{2, 0}, 0}; !equal(got, want) {
		t.Errorf("playlist = %v, want %v", got, want)
	}
	if got, want := u16s(files["PLAYER/LISTS.BIN"])[7], 0; got != want {
		t.Errorf("first album of Leon Bridges = %d, want %d", got, want)
	}
}

func equal(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, xok := a[i].([]int)
		y, yok := b[i].([]int)
		if xok && yok {
			if !slices.Equal(x, y) {
				return false
			}
			continue
		}
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTextLongerThanALengthByteIsCutOnACharacter(t *testing.T) {
	lib := sample()
	lib.Paths["t1"] = strings.Repeat("é", 150)
	files := Files(lib)
	r := record(files, "TRACKS.BIN", 24, 0)
	r.str()
	got := r.str()
	if len(got) > 255 || len(got) < 254 || !utf8.ValidString(got) || !strings.HasPrefix(lib.Paths["t1"], got) {
		t.Fatalf("stored path is %d bytes, valid %v", len(got), utf8.ValidString(got))
	}
}
