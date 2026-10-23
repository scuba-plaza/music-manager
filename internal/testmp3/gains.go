package testmp3

import "fmt"

// Layout is the kind of frames WithGains builds.
type Layout struct {
	MPEG2 bool // MPEG-2 at 22.05 kHz, one granule a frame, instead of MPEG-1 at 44.1 kHz with two
	Mono  bool
	CRC   bool // the frames carry a CRC of their side information
}

// Channels is how many channels the frames have.
func (l Layout) Channels() int {
	if l.Mono {
		return 1
	}
	return 2
}

// Granules is how many granules (576 samples) a frame has.
func (l Layout) Granules() int {
	if l.MPEG2 {
		return 1
	}
	return 2
}

// side describes the side information as the standard lays it out: the bits
// before the first granule's, how many bits each granule's channel takes, and
// how many bytes the whole comes to.
func (l Layout) side() (head, block, bytes int) {
	switch {
	case !l.MPEG2 && l.Mono:
		return 9 + 5 + 4, 59, 17
	case !l.MPEG2:
		return 9 + 3 + 8, 59, 32
	case l.Mono:
		return 8 + 1, 63, 9
	}
	return 8 + 2, 63, 17
}

// gainBit is where the global_gain of a granule's channel starts in the side
// information: after the 12 bits of part2_3_length and the 9 of big_values.
func (l Layout) gainBit(granule, channel int) int {
	head, block, _ := l.side()
	return head + (granule*l.Channels()+channel)*block + 12 + 9
}

func init() {
	// The side information must come to the size the standard gives it.
	for _, l := range []Layout{{}, {Mono: true}, {MPEG2: true}, {MPEG2: true, Mono: true}} {
		head, block, size := l.side()
		if bits := head + l.Granules()*l.Channels()*block; bits != size*8 {
			panic(fmt.Sprintf("testmp3: %+v side information is %d bits, not %d", l, bits, size*8))
		}
	}
}

// headerAndSize are the four header bytes of a frame of the layout, and its
// length: 128 kbps at 44.1 kHz for MPEG-1, 64 kbps at 22.05 kHz for MPEG-2.
func (l Layout) headerAndSize() ([]byte, int) {
	h := []byte{0xFF, 0xFB, 0x90, 0x00}
	size := FrameSize
	if l.MPEG2 {
		h[1], h[2], size = 0xF3, 0x80, 208
	}
	if l.CRC {
		h[1] &^= 1
	}
	if l.Mono {
		h[3] = 0xC0
	}
	return h, size
}

// WithGains returns frames of the layout whose granules have the global_gain
// given in gains, one list per granule in order with a value per channel; a
// last frame left short of granules repeats the last ones. Everything else in
// the frames is a fixed filler, different from one frame to the next, so that
// a test can see what was and what was not changed.
func WithGains(l Layout, gains [][]int) []byte {
	var out []byte
	rng := uint32(12345)
	next := func() byte {
		rng = rng*1664525 + 1013904223
		return byte(rng >> 24)
	}
	for first := 0; first < len(gains); first += l.Granules() {
		header, size := l.headerAndSize()
		frame := make([]byte, size)
		for i := range frame {
			frame[i] = next()
		}
		copy(frame, header)
		_, _, sideLen := l.side()
		side := 4
		if l.CRC {
			side += 2
		}
		for g := 0; g < l.Granules(); g++ {
			for c := 0; c < l.Channels(); c++ {
				v := gains[min(first+g, len(gains)-1)][c]
				setBits(frame[side:side+sideLen], l.gainBit(g, c), 8, v)
			}
		}
		if l.CRC {
			crc := GainCRC(frame[2:4], frame[side:side+sideLen])
			frame[4], frame[5] = byte(crc>>8), byte(crc)
		}
		out = append(out, frame...)
	}
	return out
}

// GainCRC is the CRC an MPEG audio frame carries for the last two bytes of
// its header and its side information.
func GainCRC(headerTail, sideInfo []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range append(append([]byte{}, headerTail...), sideInfo...) {
		crc = crcByte(crc, b)
	}
	return crc
}

// crcByte feeds a byte to the CRC-16 of the standard: x^16 + x^15 + x^2 + 1.
func crcByte(crc uint16, b byte) uint16 {
	for i := 7; i >= 0; i-- {
		in := uint16(b>>i) & 1
		top := crc >> 15
		crc <<= 1
		if top^in == 1 {
			crc ^= 0x8005
		}
	}
	return crc
}

// setBits writes the n bits of v, the most significant first, from bit
// position at of b.
func setBits(b []byte, at, n, v int) {
	for i := 0; i < n; i++ {
		bit := byte(v>>(n-1-i)) & 1
		pos := at + i
		b[pos/8] = b[pos/8]&^(0x80>>(pos%8)) | bit<<(7-pos%8)
	}
}
