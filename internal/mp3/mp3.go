// Package mp3 extracts the bare audio stream of an MP3 file and measures it.
package mp3

import (
	"errors"
	"time"
)

var (
	v1Bitrates = [16]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}
	v2Bitrates = [16]int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0}
	// indexed by [version][sample rate index]; version 0 = MPEG-2.5, 1 = MPEG-2, 2 = MPEG-1
	sampleRates = [3][3]int{{11025, 12000, 8000}, {22050, 24000, 16000}, {44100, 48000, 32000}}
)

// maxSkip bounds how far Parse looks for the next frame after garbage.
const maxSkip = 64 * 1024

type frame struct {
	size    int
	samples int
	rate    int
	info    bool // an encoder's Xing/Info/VBRI header, not audio
}

// frameAt decodes an MPEG audio Layer III frame header at the start of b.
func frameAt(b []byte) (frame, bool) {
	if len(b) < 4 || b[0] != 0xFF || b[1]&0xE0 != 0xE0 {
		return frame{}, false
	}
	version := int(b[1]>>3) & 3 // 0 = 2.5, 1 = reserved, 2 = MPEG-2, 3 = MPEG-1
	layer := int(b[1]>>1) & 3   // 1 = Layer III
	brIdx, srIdx, pad := int(b[2]>>4), int(b[2]>>2)&3, int(b[2]>>1)&1
	if version == 1 || layer != 1 || brIdx == 0 || brIdx == 15 || srIdx == 3 {
		return frame{}, false
	}
	vi := map[int]int{0: 0, 2: 1, 3: 2}[version]
	rate := sampleRates[vi][srIdx]
	f := frame{size: 144*v1Bitrates[brIdx]*1000/rate + pad, samples: 1152, rate: rate}
	if version != 3 {
		f = frame{size: 72*v2Bitrates[brIdx]*1000/rate + pad, samples: 576, rate: rate}
	}
	f.info = f.size <= len(b) && isInfoFrame(b[:f.size], version == 3, b[3]>>6 == 3)
	return f, true
}

// xingOffset is where a Xing/Info marker sits in a frame: after the 4-byte
// header and the side information, whose length depends on the MPEG version
// and channel mode.
func xingOffset(mpeg1, mono bool) int {
	switch {
	case mpeg1 && mono, !mpeg1 && !mono:
		return 4 + 17
	case !mpeg1 && mono:
		return 4 + 9
	}
	return 4 + 32
}

// isInfoFrame looks for the Xing/Info marker after the side information,
// whose length depends on MPEG version and channel mode, or a VBRI marker.
func isInfoFrame(b []byte, mpeg1, mono bool) bool {
	off := xingOffset(mpeg1, mono)
	has := func(at int, markers ...string) bool {
		for _, m := range markers {
			if len(b) >= at+len(m) && string(b[at:at+len(m)]) == m {
				return true
			}
		}
		return false
	}
	return has(off, "Xing", "Info") || has(4+32, "VBRI")
}

// verifiedFrameAt reports a frame at b that fits in b and is followed by
// another valid frame or the end of b, to rule out false sync patterns.
func verifiedFrameAt(b []byte) (frame, bool) {
	f, ok := frameAt(b)
	if !ok || f.size > len(b) {
		return frame{}, false
	}
	if f.size == len(b) {
		return f, true
	}
	_, ok = frameAt(b[f.size:])
	return f, ok
}

// Parse returns the audio of an MP3 file with all tags and any garbage
// removed, together with its duration.
func Parse(data []byte) ([]byte, time.Duration, error) {
	pos := skipID3v2(data)
	var audio []byte
	var dur time.Duration
	synced := false // the previous frame was valid, so this one needs no extra proof
	for pos < len(data) {
		f, ok := frameAt(data[pos:])
		ok = ok && f.size <= len(data)-pos
		if !synced && ok {
			_, ok = verifiedFrameAt(data[pos:])
		}
		if !ok {
			skip := nextFrame(data[pos:])
			if skip < 0 {
				break
			}
			pos += skip
			synced = false
			continue
		}
		if !f.info {
			audio = append(audio, data[pos:pos+f.size]...)
			dur += time.Duration(f.samples) * time.Second / time.Duration(f.rate)
		}
		pos += f.size
		synced = true
	}
	if len(audio) == 0 {
		return nil, 0, errors.New("mp3: no audio frames found")
	}
	return audio, dur, nil
}

// nextFrame returns the offset of the next verified frame in b, or -1.
func nextFrame(b []byte) int {
	for i := 1; i < len(b) && i <= maxSkip; i++ {
		if _, ok := verifiedFrameAt(b[i:]); ok {
			return i
		}
	}
	return -1
}

// skipID3v2 returns the offset after any ID3v2 tags at the start of data.
func skipID3v2(data []byte) int {
	pos := 0
	for len(data)-pos >= 10 && string(data[pos:pos+3]) == "ID3" {
		flags := data[pos+5]
		size := int(data[pos+6]&0x7F)<<21 | int(data[pos+7]&0x7F)<<14 | int(data[pos+8]&0x7F)<<7 | int(data[pos+9]&0x7F)
		pos += 10 + size
		if flags&0x10 != 0 { // footer present
			pos += 10
		}
	}
	return min(pos, len(data))
}
