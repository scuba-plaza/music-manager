package tui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"image"
	"image/png"
	"math"
	"strings"

	"music-manager/internal/coverimg"
)

// This file speaks the part of the kitty graphics protocol that draws the
// cover in full resolution, in terminals that have it (kitty, Ghostty).
//
// The picture is sent to the terminal once, as a "virtual placement" of a
// given size in character cells. Where it is to appear, the interface then
// writes ordinary text: a special placeholder character in every cell, whose
// colour names the picture and whose two combining marks name the cell's row
// and column within it. Because it is only text, the picture scrolls, gets
// covered by dialogs and is redrawn like everything else, which a picture
// placed at the cursor would not be.
//
// https://sw.kovidgoyal.net/kitty/graphics-protocol/#unicode-placeholders

// placeholderRune is the character kitty draws a picture's cell over.
const placeholderRune = '\U0010EEEE'

// rowColumnMarks are the combining marks that number rows and columns, in
// order: the first is 0, the second 1, and so on. kitty defines 297; this is
// the start of the list, from kitty's own output, which is far more than a
// cover ever needs.
var rowColumnMarks = [...]rune{
	0x0305, 0x030D, 0x030E, 0x0310, 0x0312, 0x033D, 0x033E, 0x033F,
	0x0346, 0x034A, 0x034B, 0x034C, 0x0350, 0x0351, 0x0352, 0x0357,
	0x035B, 0x0363, 0x0364, 0x0365, 0x0366, 0x0367, 0x0368, 0x0369,
	0x036A, 0x036B, 0x036C, 0x036D, 0x036E, 0x036F, 0x0483, 0x0484,
	0x0485, 0x0486, 0x0487, 0x0592, 0x0593, 0x0594, 0x0595, 0x0597,
	0x0598, 0x0599, 0x059C, 0x059D, 0x059E, 0x059F, 0x05A0, 0x05A1,
	0x05A8, 0x05A9, 0x05AB, 0x05AC, 0x05AF, 0x05C4, 0x0610, 0x0611,
	0x0612, 0x0613, 0x0614, 0x0615, 0x0616, 0x0617, 0x0657, 0x0658,
	0x0659, 0x065A, 0x065B, 0x065D, 0x065E, 0x06D6, 0x06D7, 0x06D8,
	0x06D9, 0x06DA, 0x06DB, 0x06DC, 0x06DF, 0x06E0, 0x06E1, 0x06E2,
	0x06E4, 0x06E7, 0x06E8, 0x06EB, 0x06EC, 0x0730, 0x0732, 0x0733,
	0x0735, 0x0736, 0x073A, 0x073D, 0x073F, 0x0740, 0x0741, 0x0743,
	0x0745, 0x0747, 0x0749, 0x074A, 0x07EB, 0x07EC, 0x07ED, 0x07EE,
	0x07EF, 0x07F0, 0x07F1, 0x07F3, 0x0816, 0x0817, 0x0818, 0x0819,
	0x081B, 0x081C, 0x081D, 0x081E, 0x081F, 0x0820, 0x0821, 0x0822,
	0x0823, 0x0825, 0x0826, 0x0827, 0x0829, 0x082A, 0x082B, 0x082C,
}

// maxPlaceholders is the most rows or columns a picture can have.
const maxPlaceholders = len(rowColumnMarks)

// kittyMaxPixels is the longest side of the picture sent to the terminal.
// The terminal scales it to the cells it is given.
const kittyMaxPixels = 640

// chunkSize is the most payload one graphics command may carry.
const chunkSize = 4096

// placeholderLines is the text that shows a picture of cols×rows cells.
func placeholderLines(id uint32, cols, rows int) []string {
	cols, rows = min(cols, maxPlaceholders), min(rows, maxPlaceholders)
	colour := fmt.Sprintf("\x1b[38;2;%d;%d;%dm", id>>16&0xFF, id>>8&0xFF, id&0xFF)
	lines := make([]string, rows)
	for r := range lines {
		var b strings.Builder
		b.WriteString(colour)
		for c := 0; c < cols; c++ {
			b.WriteRune(placeholderRune)
			b.WriteRune(rowColumnMarks[r])
			b.WriteRune(rowColumnMarks[c])
		}
		b.WriteString("\x1b[39m")
		lines[r] = b.String()
	}
	return lines
}

// apc wraps a graphics command: keys, and optionally a payload.
func apc(keys, payload string) string {
	if payload == "" {
		return "\x1b_G" + keys + "\x1b\\"
	}
	return "\x1b_G" + keys + ";" + payload + "\x1b\\"
}

// transmitSeq sends a PNG to the terminal as picture id and creates its
// virtual placement, cols×rows cells large. q=2 keeps the terminal from
// answering, which would arrive as typing.
func transmitSeq(id uint32, pngData []byte, cols, rows int) string {
	payload := base64.StdEncoding.EncodeToString(pngData)
	keys := fmt.Sprintf("a=T,f=100,U=1,q=2,C=1,i=%d,c=%d,r=%d", id, cols, rows)
	var b strings.Builder
	for first := true; ; first = false {
		n := min(len(payload), chunkSize)
		chunk := payload[:n]
		payload = payload[n:]
		more := 0
		if len(payload) > 0 {
			more = 1
		}
		switch {
		case first && more == 0:
			b.WriteString(apc(keys, chunk))
		case first:
			b.WriteString(apc(keys+",m=1", chunk))
		default:
			b.WriteString(apc(fmt.Sprintf("m=%d", more), chunk))
		}
		if more == 0 {
			return b.String()
		}
	}
}

// deleteSeq removes picture id from the terminal, and frees its memory.
func deleteSeq(id uint32) string { return apc(fmt.Sprintf("a=d,d=I,q=2,i=%d", id), "") }

// imageID names the picture of a cover shown at a size. Each size gets an
// id of its own, so a picture is never resized in place, which keeps the
// terminal's notion of it and ours from ever disagreeing. Ids are 24 bits:
// that is what fits in a colour.
func imageID(sha string, cols, rows int) uint32 {
	h := fnv.New32a()
	fmt.Fprintf(h, "%s:%dx%d", sha, cols, rows)
	return max(h.Sum32()&0xFFFFFF, 1)
}

// placementSize is how many cells wide and high to draw a picture so that,
// with cells of cellW×cellH pixels, it keeps its proportions and fits in a
// box of boxCols×boxRows cells. A cell size of 0 means unknown, and cells are
// then taken to be twice as tall as wide, as they nearly always are.
func placementSize(imgW, imgH, boxCols, boxRows, cellW, cellH int) (cols, rows int) {
	if imgW < 1 || imgH < 1 || boxCols < 1 || boxRows < 1 {
		return 0, 0
	}
	if cellW < 1 || cellH < 1 {
		cellW, cellH = 1, 2
	}
	scale := math.Min(float64(boxCols*cellW)/float64(imgW), float64(boxRows*cellH)/float64(imgH))
	cols = min(max(int(math.Round(float64(imgW)*scale/float64(cellW))), 1), boxCols)
	rows = min(max(int(math.Round(float64(imgH)*scale/float64(cellH))), 1), boxRows)
	return cols, rows
}

// encodeForKitty turns a cover (JPEG or PNG) into the PNG that is sent to
// the terminal, shrunk if it is bigger than the terminal could show, and
// says how big that PNG is.
func encodeForKitty(data []byte) (encoded []byte, w, h int, err error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, err
	}
	b := img.Bounds()
	w, h = b.Dx(), b.Dy()
	if longest := max(w, h); longest > kittyMaxPixels {
		w, h = max(w*kittyMaxPixels/longest, 1), max(h*kittyMaxPixels/longest, 1)
		img = coverimg.Resize(img, w, h)
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), w, h, nil
}

// wantsKittyGraphics says whether covers should be drawn with the graphics
// protocol: in kitty and Ghostty, but not through tmux or screen, which do not
// pass the pictures on. MM_COVERS=kitty or MM_COVERS=blocks overrides the
// guess.
func wantsKittyGraphics(getenv func(string) string) bool {
	switch getenv("MM_COVERS") {
	case "kitty":
		return true
	case "blocks":
		return false
	}
	if getenv("TMUX") != "" || getenv("STY") != "" {
		return false
	}
	term := getenv("TERM")
	return term == "xterm-kitty" || term == "xterm-ghostty" ||
		getenv("KITTY_WINDOW_ID") != "" || getenv("TERM_PROGRAM") == "ghostty"
}
