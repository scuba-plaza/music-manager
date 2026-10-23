//go:build pty

package tui

import (
	"encoding/json"
	"image"
	"image/color"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// kittyMakes runs output through kitty's own terminal core, which this
// machine has if kitty is installed, and reports what it made of it.
type kittyResult struct {
	Images  int
	Replies string
	Layers  []struct{ Left, Top, Right, Bottom float64 }
}

// kittyMakes is kittyMakesAt on a screen of 80×24 cells.
func kittyMakes(t *testing.T, output string) kittyResult {
	t.Helper()
	return kittyMakesAt(t, output, 80, 24)
}

func kittyMakesAt(t *testing.T, output string, cols, rows int) (result kittyResult) {
	t.Helper()
	kitty, err := exec.LookPath("kitty")
	if err != nil {
		t.Skip("kitty is not installed, so the pictures cannot be checked against it")
	}
	dir := t.TempDir()
	in := filepath.Join(dir, "output.bin")
	if err := os.WriteFile(in, []byte(output), 0o644); err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs("testdata/kitty_screen.py")
	if err != nil {
		t.Fatal(err)
	}
	code := "import sys; sys.argv=['x', " + pyString(in) + ", " + strconv.Itoa(cols) + ", " + strconv.Itoa(rows) + "]; exec(open(" + pyString(script) + ").read())"
	out, err := exec.Command(kitty, "+runpy", code).CombinedOutput()
	if err != nil {
		t.Fatalf("kitty: %v\n%s", err, out)
	}
	var raw struct {
		Images  int                                          `json:"images"`
		Replies string                                       `json:"replies"`
		Layers  []struct{ Left, Top, Right, Bottom float64 } `json:"layers"`
	}
	line := out[strings.LastIndex(strings.TrimSpace(string(out)), "\n")+1:]
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(line))), &raw); err != nil {
		t.Fatalf("kitty said %q: %v", out, err)
	}
	result.Images, result.Replies, result.Layers = raw.Images, raw.Replies, raw.Layers
	return result
}

func pyString(s string) string { b, _ := json.Marshal(s); return string(b) }

func noise(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewSource(1))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(r.Intn(256)), uint8(r.Intn(256)), uint8(r.Intn(256)), 255})
		}
	}
	return img
}

func TestKittyTakesOurPictureAndMapsEveryPlaceholderRowOntoIt(t *testing.T) {
	const id = 0x2157F8
	output := transmitSeq(id, encodePNG(t, twoTone(60, 60)), 12, 6) + strings.Join(placeholderLines(id, 12, 6), "\r\n")
	got := kittyMakes(t, output)

	if got.Images != 1 || !strings.Contains(got.Replies, "i=2185208;OK") {
		t.Fatalf("kitty did not take the picture: %d images, replies %q", got.Images, got.Replies)
	}
	if len(got.Layers) != 6 {
		t.Fatalf("kitty drew the picture in %d pieces, want one per row, 6", len(got.Layers))
	}
	for i, l := range got.Layers {
		wantTop, wantBottom := float64(i)/6, float64(i+1)/6
		if l.Left != 0 || l.Right != 1 || abs(l.Top-wantTop) > 0.001 || abs(l.Bottom-wantBottom) > 0.001 {
			t.Errorf("row %d shows %+v of the picture, want its full width and %.3f–%.3f of its height", i, l, wantTop, wantBottom)
		}
	}
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func TestKittyAssemblesABigPictureSentInChunks(t *testing.T) {
	data, _, _, err := encodeForKitty(encodePNG(t, noise(700, 700)))
	if err != nil {
		t.Fatal(err)
	}
	got := kittyMakes(t, transmitSeq(77, data, 20, 10))
	if got.Images != 1 || got.Replies != "\x1b_Gi=77;OK\x1b\\" {
		t.Errorf("%d images, replies %q (a picture of %d bytes was sent in chunks)", got.Images, got.Replies, len(data))
	}
}

func TestKittyForgetsPicturesWeDelete(t *testing.T) {
	png := encodePNG(t, twoTone(20, 20))
	one := kittyMakes(t, transmitSeq(1, png, 4, 2)+transmitSeq(2, png, 4, 2)+deleteSeq(1))
	if one.Images != 1 {
		t.Errorf("after deleting one of two pictures kitty has %d", one.Images)
	}
	both := kittyMakes(t, transmitSeq(1, png, 4, 2)+transmitSeq(2, png, 4, 2)+deleteSeq(1)+deleteSeq(2))
	if both.Images != 0 {
		t.Errorf("after deleting both kitty has %d", both.Images)
	}
}

// TestKittyShowsTheCoverOfTheRealProgram runs the real mm in a terminal,
// selects a track with a cover, and plays everything the program wrote to
// the terminal to kitty's own terminal core: the picture must arrive and be
// laid over the cells where the program drew its placeholders.
func TestKittyShowsTheCoverOfTheRealProgram(t *testing.T) {
	a := testApp(t)
	seed(t, a, seedTrack{"The Wanderer", "Shawn James", "Shadows", 1, 12, 0}, seedTrack{"Flow", "Shawn James", "Shadows", 7, 12, 0})
	flow := trackNamed(t, NewLibrary(a), "Flow")
	if err := a.SetCover(flow.ID, encodePNG(t, twoTone(300, 300))); err != nil {
		t.Fatal(err)
	}
	s := startMM(t, 120, 36, a.Root(), "MM_COVERS=kitty")
	s.MustSee("Shawn James")
	s.Type("/flow\r")
	s.Type("\x1b[B\x1b[B\x1b[B")
	s.MustSee("300×300 JPEG")
	if !s.WaitFor(5*time.Second, "\U0010EEEE") {
		t.Fatalf("no placeholders on the screen:\n%s", s.Screen())
	}
	time.Sleep(300 * time.Millisecond) // let the last frame be written

	got := kittyMakesAt(t, string(s.Raw()), 120, 36)
	if got.Images != 1 || !strings.Contains(got.Replies, ";OK") || strings.Contains(got.Replies, "ERR") {
		t.Fatalf("kitty has %d pictures and said %q", got.Images, got.Replies)
	}
	rows := strings.Count(s.Screen(), "\U0010EEEE") // every cell is one placeholder
	if rows == 0 || len(got.Layers) == 0 || len(got.Layers) > 24 {
		t.Fatalf("kitty laid the picture out in %d pieces for %d placeholders", len(got.Layers), rows)
	}
	for i, l := range got.Layers {
		if l.Left != 0 || l.Right != 1 {
			t.Errorf("piece %d shows only %+v of the picture's width", i, l)
		}
	}
	if last := got.Layers[len(got.Layers)-1]; abs(last.Bottom-1) > 0.001 {
		t.Errorf("the pieces end at %.3f of the picture's height, they should reach its bottom", last.Bottom)
	}
}

func TestKittyIsLeftTidyWhenTheProgramQuits(t *testing.T) {
	a := testApp(t)
	seed(t, a, seedTrack{"Flow", "Shawn James", "Shadows", 7, 12, 0})
	flow := trackNamed(t, NewLibrary(a), "Flow")
	if err := a.SetCover(flow.ID, encodePNG(t, twoTone(300, 300))); err != nil {
		t.Fatal(err)
	}
	s := startMM(t, 100, 30, a.Root(), "MM_COVERS=kitty")
	s.MustSee("Shawn James")
	s.Type("/flow\r")
	s.Type("\x1b[B\x1b[B\x1b[B")
	s.MustSee("300×300 JPEG")
	if !s.WaitFor(5*time.Second, "\U0010EEEE") {
		t.Fatalf("no placeholders:\n%s", s.Screen())
	}

	s.Type("q")
	if exited, err := s.Exited(3 * time.Second); !exited || err != nil {
		t.Fatalf("exited %v, err %v", exited, err)
	}
	if s.em.IsAltScreen() {
		t.Error("the terminal was left in the alternate screen")
	}
	if got := kittyMakesAt(t, string(s.Raw()), 100, 30); got.Images != 0 {
		t.Errorf("%d pictures were left in the terminal's memory", got.Images)
	}
}
