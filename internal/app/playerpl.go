package app

import (
	"bytes"
	"encoding/binary"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"

	"music-manager/internal/manifest"
	"music-manager/internal/translit"
)

// The playlist files of the Wolfang WMP01 (USERPL1.PL to USERPL3.PL) were
// worked out from the player's own files; nothing about them is published.
// A file is a header of 512 bytes ("USERPL" and a digit, and the 0x55 0xAA of
// a sector at its end) and then one record of 256 bytes for each entry. A
// record starts with a number that is 0 for the record that ends the list.
// It says where the file is on the card by the number of its folder's first
// cluster and its place in the folder, which only the player can use, and
// holds the file's name in UTF-16 after a byte-order mark at byte 104,
// cut to 26 characters.
const (
	playerHeader   = 512
	playerRecord   = 256
	playerNameAt   = 104
	playerMagic    = "USERPL"
	playerMaxEntry = 1000
)

// ReadPlayerPlaylist returns the file names in one of the player's playlist
// files, in the order they are stored. A name is as the player keeps it: at
// most 26 characters, so a long one is cut short and has lost its ".mp3".
func ReadPlayerPlaylist(data []byte) ([]string, error) {
	if len(data) < playerHeader || !bytes.HasPrefix(data, []byte(playerMagic)) || data[len(playerMagic)] < '0' || data[len(playerMagic)] > '9' {
		return nil, errors.New("not a playlist file of the player (USERPL1.PL to USERPL3.PL)")
	}
	var names []string
	for off := playerHeader; off+playerRecord <= len(data) && len(names) < playerMaxEntry; off += playerRecord {
		rec := data[off : off+playerRecord]
		if binary.LittleEndian.Uint32(rec) == 0 {
			break // the record that ends the list
		}
		if rec[playerNameAt] != 0xff || rec[playerNameAt+1] != 0xfe {
			return nil, errors.New("a record of the playlist has no name: is it really a playlist file of the player?")
		}
		var units []uint16
		for i := playerNameAt + 2; i+1 < playerRecord; i += 2 {
			u := binary.LittleEndian.Uint16(rec[i:])
			if u == 0 {
				break
			}
			units = append(units, u)
		}
		names = append(names, string(utf16.Decode(units)))
	}
	return names, nil
}

// PlayerMatch is what one entry of a playlist file of the player came to.
type PlayerMatch struct {
	Name string // as the player kept it
	// Track is the track it is, if one could be told. Candidates are the
	// tracks it could be when more than one fits and nothing tells them apart.
	Track      *manifest.Track
	Candidates []manifest.Track
	// Partial means the entry was matched by the start of a title only: the
	// player cuts long names short, and titles may have grown since.
	Partial bool
}

// MatchPlayerNames finds, for each file name that a playlist of the player
// held, the track in the library that it was. The names are the old ones, from
// before the library renamed its files, so they are compared with the titles:
// without the track number and ".mp3" at the start and end, ignoring case and
// everything that is not a letter or a digit. A number that is there tells
// apart tracks of the same title.
func (a *App) MatchPlayerNames(names []string) ([]PlayerMatch, error) {
	m, err := a.store.Load()
	if err != nil {
		return nil, err
	}
	matches := make([]PlayerMatch, len(names))
	for i, name := range names {
		matches[i] = matchPlayerName(name, m.Tracks)
	}
	return matches, nil
}

var leadingNumber = regexp.MustCompile(`^(\d{1,3})(?:\s*[-.]\s*|\s+)(.+)$`)

func matchPlayerName(name string, tracks []manifest.Track) PlayerMatch {
	res := PlayerMatch{Name: name}
	stem := strings.TrimSpace(strings.TrimSuffix(name, ".mp3"))
	number, keys := 0, []string{matchKey(stem)}
	if g := leadingNumber.FindStringSubmatch(stem); g != nil {
		number, _ = strconv.Atoi(g[1])
		keys = []string{matchKey(g[2]), keys[0]} // a title can start with a number: try the whole too
	}

	var cands []manifest.Track
	for _, key := range keys {
		if key == "" {
			continue
		}
		if cands = titled(tracks, key, false); len(cands) > 0 {
			break
		}
	}
	if len(cands) == 0 {
		for _, key := range keys {
			if key == "" {
				continue
			}
			if cands = titled(tracks, key, true); len(cands) > 0 {
				res.Partial = true
				break
			}
		}
	}

	if len(cands) > 1 && number > 0 {
		var same []manifest.Track
		for _, t := range cands {
			if t.TrackNumber == number {
				same = append(same, t)
			}
		}
		if len(same) > 0 {
			cands = same
		}
	}
	switch {
	case len(cands) == 1:
		res.Track = &cands[0]
	case len(cands) > 1:
		res.Candidates = cands
	}
	return res
}

// titled lists the tracks whose title, or its Latin form, is key once it is
// reduced the same way; or starts with it, if prefix is set.
func titled(tracks []manifest.Track, key string, prefix bool) []manifest.Track {
	var out []manifest.Track
	for _, t := range tracks {
		for _, title := range []string{t.OriginalTitle, translit.ToASCII(t.OriginalTitle)} {
			k := matchKey(title)
			if k == key || prefix && strings.HasPrefix(k, key) {
				out = append(out, t)
				break
			}
		}
	}
	return out
}

// matchKey reduces a title to its letters and digits, in lower case.
func matchKey(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}
