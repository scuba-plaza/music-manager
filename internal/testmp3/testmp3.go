// Package testmp3 builds small synthetic MP3 files for tests: silent
// MPEG-1 Layer III frames, optionally wrapped in tags.
package testmp3

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"music-manager/internal/coverimg"
)

// FrameSize is the byte length of one frame (128 kbps, 44.1 kHz, no padding).
const FrameSize = 417

// Silent returns n silent frames, each 1152 samples long. The header bytes
// FF FB 90 00 mean MPEG-1, Layer III, no CRC, 128 kbps, 44100 Hz, stereo.
func Silent(n int) []byte {
	frame := make([]byte, FrameSize)
	copy(frame, []byte{0xFF, 0xFB, 0x90, 0x00})
	return bytes.Repeat(frame, n)
}

// WithID3v2 prepends an ID3v2 tag of the given major version (3 or 4) whose
// body is bodyLen bytes of padding.
func WithID3v2(audio []byte, version byte, bodyLen int) []byte {
	hdr := []byte{'I', 'D', '3', version, 0, 0,
		byte(bodyLen >> 21 & 0x7F), byte(bodyLen >> 14 & 0x7F), byte(bodyLen >> 7 & 0x7F), byte(bodyLen & 0x7F)}
	out := append(hdr, make([]byte, bodyLen)...)
	return append(out, audio...)
}

// WithID3v1 appends a 128-byte ID3v1 tag.
func WithID3v1(audio []byte) []byte {
	tag := make([]byte, 128)
	copy(tag, "TAG")
	copy(tag[3:], "Some Title")
	return append(append([]byte{}, audio...), tag...)
}

// WithAPE appends an APEv2 tag (items-less, header-less: just a footer).
func WithAPE(audio []byte) []byte {
	footer := make([]byte, 32)
	copy(footer, "APETAGEX")
	footer[8] = 0xD0 // version 2000 little-endian: D0 07 00 00
	footer[9] = 0x07
	footer[12] = 32 // tag size including footer, little-endian
	return append(append([]byte{}, audio...), footer...)
}

// WithTags prepends an ID3v2.4 tag holding UTF-8 title, artist and album
// frames. Empty values are omitted.
func WithTags(audio []byte, title, artist, album string) []byte {
	return WithTagsTrack(audio, title, artist, album, "")
}

// WithTagsTrack is WithTags with a track field such as "3" or "3/12".
func WithTagsTrack(audio []byte, title, artist, album, track string) []byte {
	return WithTextFrames(audio, [2]string{"TIT2", title}, [2]string{"TPE1", artist}, [2]string{"TALB", album}, [2]string{"TRCK", track})
}

// WithTextFrames prepends an ID3v2.4 tag holding UTF-8 text frames, given as
// {frame id, text} pairs in the order they are written. Empty texts are
// omitted.
func WithTextFrames(audio []byte, frames ...[2]string) []byte {
	var body []byte
	for _, f := range frames {
		id, text := f[0], f[1]
		if text == "" {
			continue
		}
		n := 1 + len(text) // encoding byte + text
		body = append(body, id...)
		body = append(body, byte(n>>21&0x7F), byte(n>>14&0x7F), byte(n>>7&0x7F), byte(n&0x7F), 0, 0, 3)
		body = append(body, text...)
	}
	tag := WithID3v2(nil, 4, len(body))
	copy(tag[10:], body)
	return append(tag, audio...)
}

// WithInfoFrame prepends a Xing/Info-style metadata frame (marker is "Xing",
// "Info" or "VBRI"), as encoders write at the start of a file. It looks like
// a silent frame but carries no audio.
func WithInfoFrame(audio []byte, marker string) []byte {
	frame := make([]byte, FrameSize)
	copy(frame, []byte{0xFF, 0xFB, 0x90, 0x00})
	copy(frame[36:], marker) // after the 32-byte MPEG-1 stereo side info
	return append(frame, audio...)
}

// JPEG and JPEG2 are two small pictures, already in the form covers are kept
// in (so that storing them does not change them); PNG is a third, in a form
// that is not.
var (
	JPEG  = cover(color.RGBA{200, 40, 40, 255}, true)
	JPEG2 = cover(color.RGBA{40, 40, 200, 255}, true)
	PNG   = cover(color.RGBA{40, 200, 40, 255}, false)
)

// cover draws a 16×12 picture in col, as a cover JPEG or as a PNG.
func cover(col color.RGBA, asJPEG bool) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 16, 12))
	for y := 0; y < 12; y++ {
		for x := 0; x < 16; x++ {
			img.SetRGBA(x, y, color.RGBA{col.R, col.G, uint8(20 * x), 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	if !asJPEG {
		return b.Bytes()
	}
	out, err := coverimg.Baseline(b.Bytes(), 480, 85)
	if err != nil {
		panic(err)
	}
	return out
}

// WithCover prepends an ID3v2.4 tag holding a front-cover picture.
func WithCover(audio []byte, mime string, img []byte) []byte {
	data := append([]byte{3}, mime...)
	data = append(data, 0, 3, 0) // MIME terminator, front cover, empty description
	data = append(data, img...)
	n := len(data)
	body := append([]byte("APIC"), byte(n>>21&0x7F), byte(n>>14&0x7F), byte(n>>7&0x7F), byte(n&0x7F), 0, 0)
	body = append(body, data...)
	tag := WithID3v2(nil, 4, len(body))
	copy(tag[10:], body)
	return append(tag, audio...)
}

// LowFrameSize is the byte length of a frame of SilentLow.
const LowFrameSize = 104

// SilentLow returns n silent frames at 32 kbps (header FF FB 10 00): frames
// too small to hold a Xing header with its seek table.
func SilentLow(n int) []byte {
	frame := make([]byte, LowFrameSize)
	copy(frame, []byte{0xFF, 0xFB, 0x10, 0x00})
	return bytes.Repeat(frame, n)
}
