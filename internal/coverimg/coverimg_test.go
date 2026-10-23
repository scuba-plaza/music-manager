package coverimg_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"music-manager/internal/coverimg"
)

// The limits a device that is fussy about covers would give: the Wolfgang WMP01's.
const (
	maxEdge = 480
	quality = 85
)

// jpegShape is what the player cares about in a JPEG file, read from the
// markers themselves rather than through the decoder.
type jpegShape struct {
	jfif        bool   // an APP0 "JFIF" segment comes right after the start
	frame       byte   // the marker of the frame header: 0xC0 is baseline, 0xC2 progressive
	width       int    //
	height      int    //
	sampling    []byte // per component, horizontal<<4 | vertical
	apps        int    // segments of metadata (APPn)
	startsClean bool   // begins with the start-of-image marker
}

func shapeOf(t *testing.T, data []byte) jpegShape {
	t.Helper()
	var s jpegShape
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 {
		return s
	}
	s.startsClean = true
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xff {
			t.Fatalf("no marker at byte %d", i)
		}
		marker := data[i+1]
		if marker == 0xff { // padding
			i++
			continue
		}
		size := int(binary.BigEndian.Uint16(data[i+2:]))
		body := data[i+4 : i+2+size]
		switch {
		case marker == 0xe0 && i == 2 && bytes.HasPrefix(body, []byte("JFIF\x00")):
			s.jfif = true
		case marker >= 0xe0 && marker <= 0xef:
			s.apps++
		case marker >= 0xc0 && marker <= 0xcf && marker != 0xc4 && marker != 0xc8 && marker != 0xcc:
			s.frame = marker
			s.height = int(binary.BigEndian.Uint16(body[1:]))
			s.width = int(binary.BigEndian.Uint16(body[3:]))
			for c := 0; c < int(body[5]); c++ {
				s.sampling = append(s.sampling, body[7+3*c])
			}
			return s
		}
		i += 2 + size
	}
	return s
}

func pngOf(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// translucent is a w×h picture with a see-through corner.
func translucent(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := uint8(255)
			if x < w/4 && y < h/4 {
				a = 0
			}
			img.SetNRGBA(x, y, color.NRGBA{uint8(255 * x / w), uint8(255 * y / h), 90, a})
		}
	}
	return img
}

func TestAPNGBecomesABaselineJPEGWithAJFIFHeader(t *testing.T) {
	out, err := coverimg.Baseline(pngOf(t, translucent(100, 60)), maxEdge, quality)
	if err != nil {
		t.Fatal(err)
	}
	s := shapeOf(t, out)
	if !s.startsClean || !s.jfif {
		t.Errorf("not a JPEG with a JFIF header: %+v", s)
	}
	if s.frame != 0xc0 {
		t.Errorf("frame marker %#x, want baseline 0xc0", s.frame)
	}
	if want := []byte{0x22, 0x11, 0x11}; !bytes.Equal(s.sampling, want) {
		t.Errorf("sampling %x, want 4:2:0 %x", s.sampling, want)
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil || img.Bounds().Dx() != 100 || img.Bounds().Dy() != 60 {
		t.Errorf("decodes to %v, %v", img.Bounds(), err)
	}
}

func TestSeeThroughAreasComeOutWhiteNotBlack(t *testing.T) {
	out, err := coverimg.Baseline(pngOf(t, translucent(100, 60)), maxEdge, quality)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := img.At(5, 5).RGBA() // inside the transparent corner
	if r>>8 < 235 || g>>8 < 235 || b>>8 < 235 {
		t.Errorf("transparent corner is (%d,%d,%d), want near white", r>>8, g>>8, b>>8)
	}
}

func TestABigPictureIsFittedInsideTheLimitKeepingItsShape(t *testing.T) {
	for _, tc := range []struct {
		name               string
		w, h, wantW, wantH int
	}{
		{"landscape", 1200, 800, 480, 320},
		{"portrait", 800, 1200, 320, 480},
		{"square", 1571, 1571, 480, 480},
		{"at the limit", 480, 480, 480, 480},
		{"thin", 2400, 100, 480, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := coverimg.Baseline(pngOf(t, translucent(tc.w, tc.h)), maxEdge, quality)
			if err != nil {
				t.Fatal(err)
			}
			if s := shapeOf(t, out); s.width != tc.wantW || s.height != tc.wantH {
				t.Errorf("%dx%d came out %dx%d, want %dx%d", tc.w, tc.h, s.width, s.height, tc.wantW, tc.wantH)
			}
		})
	}
}

func TestAFileThatSuitsThePlayerAlreadyIsLeftAlone(t *testing.T) {
	once, err := coverimg.Baseline(pngOf(t, translucent(300, 200)), maxEdge, quality)
	if err != nil {
		t.Fatal(err)
	}
	twice, err := coverimg.Baseline(once, maxEdge, quality)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(once, twice) {
		t.Errorf("normalising a normalised cover changed it: %d bytes became %d", len(once), len(twice))
	}
}

func TestAJPEGWithoutAJFIFHeaderGetsOne(t *testing.T) {
	var plain bytes.Buffer // Go's own encoder writes no JFIF segment
	if err := jpeg.Encode(&plain, translucent(64, 48), nil); err != nil {
		t.Fatal(err)
	}
	if shapeOf(t, plain.Bytes()).jfif {
		t.Fatal("test input already has a JFIF header")
	}
	out, err := coverimg.Baseline(plain.Bytes(), maxEdge, quality)
	if err != nil {
		t.Fatal(err)
	}
	if s := shapeOf(t, out); !s.jfif || s.frame != 0xc0 {
		t.Errorf("result %+v", s)
	}
}

// The fixtures in testdata were made with ImageMagick (see testdata/README).
func TestAwkwardJPEGsAreRewrittenTheWayThePlayerOpensThem(t *testing.T) {
	for _, tc := range []struct {
		file     string
		sampling []byte
	}{
		{"progressive-444.jpg", []byte{0x22, 0x11, 0x11}},
		{"baseline-444-exif.jpg", []byte{0x22, 0x11, 0x11}},
		{"baseline-420-exif.jpg", []byte{0x22, 0x11, 0x11}}, // right shape, but with metadata in the way
		{"cmyk.jpg", []byte{0x22, 0x11, 0x11}},
		{"gray.jpg", []byte{0x11}},
	} {
		t.Run(tc.file, func(t *testing.T) {
			in, err := os.ReadFile(filepath.Join("testdata", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			out, err := coverimg.Baseline(in, maxEdge, quality)
			if err != nil {
				t.Fatal(err)
			}
			s := shapeOf(t, out)
			if !s.jfif || s.frame != 0xc0 || s.apps != 0 || !bytes.Equal(s.sampling, tc.sampling) || s.width != 96 || s.height != 64 {
				t.Errorf("%+v, want baseline JFIF without metadata, sampling %x, 96x64", s, tc.sampling)
			}
		})
	}
}

func TestValidateSaysWhatKindOfPictureItIs(t *testing.T) {
	var asJPEG bytes.Buffer
	if err := jpeg.Encode(&asJPEG, translucent(8, 8), nil); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		data []byte
		want string
		ok   bool
	}{
		{"a JPEG", asJPEG.Bytes(), "image/jpeg", true},
		{"a PNG", pngOf(t, translucent(8, 8)), "image/png", true},
		{"text", []byte("just some text"), "", false},
		{"a JPEG cut short", asJPEG.Bytes()[:len(asJPEG.Bytes())/2], "", false},
		{"nothing", nil, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := coverimg.Validate(c.data)
			if got != c.want || (err == nil) != c.ok {
				t.Errorf("Validate = %q, %v; want %q, ok %v", got, err, c.want, c.ok)
			}
		})
	}
}

func TestSniffReadsTheKindFromTheFirstBytes(t *testing.T) {
	var asJPEG bytes.Buffer
	if err := jpeg.Encode(&asJPEG, translucent(8, 8), nil); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		data []byte
		want string
	}{
		"jpeg": {asJPEG.Bytes(), "image/jpeg"},
		"png":  {pngOf(t, translucent(8, 8)), "image/png"},
		"text": {[]byte("just some text"), ""},
		"none": {nil, ""},
	} {
		if got := coverimg.Sniff(c.data); got != c.want {
			t.Errorf("%s: Sniff = %q, want %q", name, got, c.want)
		}
	}
}
