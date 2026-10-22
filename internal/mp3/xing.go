package mp3

import "encoding/binary"

const (
	xingFlagsAll = 0x7 // frame count, byte count and seek table present
	tocEntries   = 100
)

// xingFrame returns a Xing header frame describing audio, a run of
// contiguous frames as returned by Parse or Trim. Players use it to show the
// right length and to seek in variable-bitrate files, where length cannot be
// worked out from the file size. It returns nil only if audio holds no frame.
func xingFrame(audio []byte) []byte {
	if _, ok := frameAt(audio); !ok {
		return nil
	}
	mpeg1, mono := audio[1]>>3&3 == 3, audio[3]>>6 == 3
	off := xingOffset(mpeg1, mono)
	need := off + 16 + tocEntries

	// The header frame takes the first frame's header, without padding and
	// with the smallest bitrate whose frames are big enough to hold the table.
	var frame []byte
	for br := byte(1); br < 15; br++ {
		h := []byte{audio[0], audio[1], audio[2]&^0x02&0x0F | br<<4, audio[3]}
		if f, ok := frameAt(h); ok && f.size >= need {
			frame = make([]byte, f.size)
			copy(frame, h)
			break
		}
	}
	if frame == nil {
		return nil
	}
	size := len(frame)

	var offsets []int
	for pos := 0; pos < len(audio); {
		f, ok := frameAt(audio[pos:])
		if !ok || f.size > len(audio)-pos {
			break
		}
		offsets = append(offsets, pos)
		pos += f.size
	}
	total := len(audio) + size

	copy(frame[off:], "Xing")
	binary.BigEndian.PutUint32(frame[off+4:], xingFlagsAll)
	binary.BigEndian.PutUint32(frame[off+8:], uint32(len(offsets)))
	binary.BigEndian.PutUint32(frame[off+12:], uint32(total))
	// Entry i is how far into the stream, in 1/256ths, i% of the playing time
	// lies. As encoders write it, entry 0 is the very start, header frame included.
	for i := 1; i < tocEntries; i++ {
		pos := size + offsets[i*len(offsets)/tocEntries]
		frame[off+16+i] = byte(min(255, pos*256/total))
	}
	return frame
}
