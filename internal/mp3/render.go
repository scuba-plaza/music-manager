package mp3

import (
	"bytes"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bogem/id3v2/v2"
)

// TagSet is everything the manager writes into a rendered file's tag.
type TagSet struct {
	Title, Artist, Album string
	Genre                string
	Track, Total         int               // track n of total; zero if unknown
	Date                 string            // release date: 1999, 1999-05 or 1999-05-17; empty if unknown
	Length               time.Duration     // playing length, stored as TLEN
	Extra                map[string]string // custom TXXX frames by description
	Cover                []byte
	CoverMIME            string
}

// Render returns a complete MP3 file: a fresh ID3v2.4 tag built from ts
// followed by audio. Equal inputs give byte-identical output, which is why
// the tag is written here rather than by the id3v2 library (it writes its
// frames in map order).
func Render(audio []byte, ts TagSet) ([]byte, error) {
	var body bytes.Buffer
	text := func(id, s string) {
		if s != "" {
			writeFrame(&body, id, append([]byte{utf8Encoding}, s...))
		}
	}
	text("TIT2", ts.Title)
	text("TPE1", ts.Artist)
	text("TALB", ts.Album)
	if ts.Track > 0 {
		trck := strconv.Itoa(ts.Track)
		if ts.Total > 0 {
			trck += "/" + strconv.Itoa(ts.Total)
		}
		text("TRCK", trck)
	}
	text("TDRC", ts.Date)
	text("TCON", ts.Genre)
	if ts.Length > 0 {
		text("TLEN", strconv.FormatInt(ts.Length.Milliseconds(), 10))
	}
	keys := make([]string, 0, len(ts.Extra))
	for k := range ts.Extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		data := append([]byte{utf8Encoding}, k...)
		data = append(append(data, 0), ts.Extra[k]...)
		writeFrame(&body, "TXXX", data)
	}
	if len(ts.Cover) > 0 {
		data := append([]byte{utf8Encoding}, ts.CoverMIME...)
		data = append(data, 0, pictureFrontCover, 0) // MIME terminator, type, empty description
		writeFrame(&body, "APIC", append(data, ts.Cover...))
	}

	if body.Len() >= 1<<28 {
		return nil, errors.New("mp3: tag too large")
	}
	audio = append(xingFrame(audio), audio...)
	out := make([]byte, 0, 10+body.Len()+len(audio))
	out = append(out, 'I', 'D', '3', 4, 0, 0)
	out = append(out, syncsafe(body.Len())...)
	out = append(out, body.Bytes()...)
	return append(out, audio...), nil
}

const (
	utf8Encoding      = 3
	pictureFrontCover = 3
)

func writeFrame(w *bytes.Buffer, id string, data []byte) {
	w.WriteString(id)
	w.Write(syncsafe(len(data)))
	w.Write([]byte{0, 0}) // no flags
	w.Write(data)
}

func syncsafe(n int) []byte {
	return []byte{byte(n >> 21 & 0x7F), byte(n >> 14 & 0x7F), byte(n >> 7 & 0x7F), byte(n & 0x7F)}
}

// ReadTagSet reads back what Render wrote. Data without a readable ID3v2 tag
// yields the zero value.
func ReadTagSet(data []byte) TagSet {
	tag, err := id3v2.ParseReader(bytes.NewReader(data), id3v2.Options{Parse: true})
	if err != nil {
		return TagSet{}
	}
	defer tag.Close()
	ts := TagSet{Title: tag.Title(), Artist: tag.Artist(), Album: tag.Album(), Genre: plainGenre(tag.Genre())}
	if f, ok := tag.GetLastFrame("TRCK").(id3v2.TextFrame); ok {
		n, total, _ := strings.Cut(f.Text, "/")
		ts.Track, _ = strconv.Atoi(n)
		ts.Total, _ = strconv.Atoi(total)
	}
	for _, id := range []string{"TDRC", "TYER"} { // the 2.4 frame first, then the older year
		if f, ok := tag.GetLastFrame(id).(id3v2.TextFrame); ok && strings.TrimSpace(f.Text) != "" {
			ts.Date = strings.TrimSpace(f.Text)
			break
		}
	}
	if f, ok := tag.GetLastFrame("TLEN").(id3v2.TextFrame); ok {
		if ms, err := strconv.ParseInt(f.Text, 10, 64); err == nil {
			ts.Length = time.Duration(ms) * time.Millisecond
		}
	}
	for _, f := range tag.GetFrames("TXXX") {
		if u, ok := f.(id3v2.UserDefinedTextFrame); ok {
			if ts.Extra == nil {
				ts.Extra = map[string]string{}
			}
			ts.Extra[u.Description] = u.Value
		}
	}
	if f, ok := tag.GetLastFrame("APIC").(id3v2.PictureFrame); ok {
		ts.Cover, ts.CoverMIME = f.Picture, f.MimeType
	}
	return ts
}

// genreNumber is the "(17)" that older tags put in front of a genre's name.
var genreNumber = regexp.MustCompile(`^\(\d+\)\s*`)

// plainGenre is the text of a genre frame without such a number.
func plainGenre(s string) string {
	s = strings.TrimSpace(s)
	if rest := genreNumber.ReplaceAllString(s, ""); rest != "" {
		return rest
	}
	return s
}
