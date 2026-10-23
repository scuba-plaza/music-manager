package mp3

import (
	"fmt"
	"math"
	"time"
)

// StepDB is the change of loudness one step of global_gain makes: the
// amplitude is multiplied by 2^(1/4).
const StepDB = 1.5051499783199058

// Shape says how the loudness of audio is to change.
type Shape struct {
	// Steps is added to the global_gain of every granule.
	Steps int
	// FadeIn and FadeOut are stretches of the audio, in its own time, over which
	// the amplitude rises from nothing to its full value or falls back. Before a
	// fade-in begins and after a fade-out ends there is silence. The zero value
	// is no fade.
	FadeIn, FadeOut Fade
}

// Fade is a stretch of time, from Start to End.
type Fade struct{ Start, End time.Duration }

// floorSteps is how far a fade goes down before it is silence: 40 steps is a
// thousandth of the amplitude, 60 dB.
const floorSteps = 40

// at says what the granule in the middle of which time is t gets: the steps to
// add to its gain, or that it is to be silent.
func (s Shape) at(t float64) (steps int, silent bool) {
	f := ramp(s.FadeIn, t, true) * ramp(s.FadeOut, t, false)
	if f < math.Exp2(-floorSteps/4.0) {
		return 0, true
	}
	return s.Steps + int(math.Round(4*math.Log2(f))), false
}

// ramp is the share of the full amplitude that a fade leaves at time t.
func ramp(f Fade, t float64, in bool) float64 {
	if f.End <= f.Start {
		return 1
	}
	p := min(max((t-f.Start.Seconds())/(f.End-f.Start).Seconds(), 0), 1)
	if in {
		return p
	}
	return 1 - p
}

// granules says where the global_gains of a frame are.
type granules struct {
	side     int // where the side information starts in the frame
	sideLen  int
	first    int // the bit, in the side information, of the first global_gain
	stride   int // bits from one granule-channel's global_gain to the next
	count    int // granule-channels: granules times channels
	channels int
}

// granulesOf finds the global_gains of the frame that starts with header.
func granulesOf(header []byte) granules {
	mpeg1, mono := header[1]>>3&3 == 3, header[3]>>6 == 3
	g := granules{side: 4, channels: 2}
	if header[1]&1 == 0 {
		g.side += 2 // a CRC comes first
	}
	if mono {
		g.channels = 1
	}
	// The side information holds a few fields for the whole frame, then 59 bits
	// (63 in MPEG-2, whose scalefactors take more) for each channel of each
	// granule. Its global_gain follows the 12 bits of part2_3_length and the 9
	// of big_values.
	const gain = 12 + 9
	switch {
	case mpeg1 && mono:
		g.sideLen, g.first, g.stride, g.count = 17, 9+5+4+gain, 59, 2
	case mpeg1:
		g.sideLen, g.first, g.stride, g.count = 32, 9+3+8+gain, 59, 4
	case mono:
		g.sideLen, g.first, g.stride, g.count = 9, 8+1+gain, 63, 1
	default:
		g.sideLen, g.first, g.stride, g.count = 17, 8+2+gain, 63, 2
	}
	return g
}

// GlobalGains returns the global_gain of every granule of audio, as returned
// by Parse, with a value for each channel.
func GlobalGains(audio []byte) ([][]int, error) {
	var out [][]int
	err := eachFrame(audio, func(frame []byte, g granules, _ frameInfo) {
		for i := 0; i < g.count; i += g.channels {
			row := make([]int, g.channels)
			for c := range row {
				row[c] = getBits(frame[g.side:g.side+g.sideLen], g.first+(i+c)*g.stride, 8)
			}
			out = append(out, row)
		}
	})
	return out, err
}

// Adjust returns audio, as returned by Parse, with its loudness changed as
// shape says. The change is lossless: only the global_gain of the granules
// differs from audio, which is left as it is.
func Adjust(audio []byte, shape Shape) ([]byte, error) {
	out := append([]byte(nil), audio...)
	err := eachFrame(out, func(frame []byte, g granules, info frameInfo) {
		side := frame[g.side : g.side+g.sideLen]
		for i := 0; i < g.count; i++ {
			at := g.first + i*g.stride
			steps, silent := shape.at(info.start + (float64(i/g.channels)+0.5)*info.granule)
			gain := min(max(getBits(side, at, 8)+steps, 0), 255)
			if silent {
				gain = 0
			}
			setBits(side, at, 8, gain)
		}
		fixCRC(frame, g)
	})
	return out, err
}

// frameInfo says where a frame is in the audio: when it starts and how long a
// granule of it is, in seconds.
type frameInfo struct{ start, granule float64 }

// eachFrame calls fn for every frame of audio that carries sound, with the
// frame as a slice of audio, so that it can be changed in place.
func eachFrame(audio []byte, fn func(frame []byte, g granules, info frameInfo)) error {
	var elapsed float64
	for pos := 0; pos < len(audio); {
		f, ok := frameAt(audio[pos:])
		if !ok || f.size > len(audio)-pos {
			return fmt.Errorf("mp3: no frame at byte %d", pos)
		}
		if !f.info {
			frame := audio[pos : pos+f.size]
			fn(frame, granulesOf(frame), frameInfo{start: elapsed, granule: 576 / float64(f.rate)})
			elapsed += float64(f.samples) / float64(f.rate)
		}
		pos += f.size
	}
	return nil
}

// getBits reads the n bits, the most significant first, from bit position at
// of b.
func getBits(b []byte, at, n int) int {
	v := 0
	for i := 0; i < n; i++ {
		pos := at + i
		v = v<<1 | int(b[pos/8]>>(7-pos%8))&1
	}
	return v
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

// fixCRC writes the CRC that a frame with a CRC carries, which covers the last
// two bytes of its header and its side information.
func fixCRC(frame []byte, g granules) {
	if g.side == 4 {
		return
	}
	crc := crc16(0xFFFF, frame[2:4])
	crc = crc16(crc, frame[g.side:g.side+g.sideLen])
	frame[4], frame[5] = byte(crc>>8), byte(crc)
}

// crc16 continues the CRC-16 of the standard (x^16 + x^15 + x^2 + 1) over b.
func crc16(crc uint16, b []byte) uint16 {
	for _, c := range b {
		for i := 7; i >= 0; i-- {
			in := uint16(c>>i) & 1
			top := crc >> 15
			crc <<= 1
			if top^in == 1 {
				crc ^= 0x8005
			}
		}
	}
	return crc
}
