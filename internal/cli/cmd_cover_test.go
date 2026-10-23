package cli_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"music-manager/internal/manifest"
)

// plantPNGCover gives the track a cover the way an earlier version stored it:
// a PNG, as it came.
func plantPNGCover(t *testing.T, lib, id string) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 900, 600))
	for y := 0; y < 600; y++ {
		for x := 0; x < 900; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x), uint8(y), 99, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b.Bytes())
	sha := hex.EncodeToString(sum[:])
	dir := filepath.Join(lib, ".mm", "covers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, sha+".img"), b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	store := manifest.NewStore(filepath.Join(lib, "library.json"))
	m, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	tr, _ := m.Get(id)
	tr.CoverSHA256 = sha
	m.Update(tr)
	if err := store.Save(m); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizingCoversFromTheCommandLine(t *testing.T) {
	lib, id := libWithSongs(t, "Dream", "Atlek")
	plantPNGCover(t, lib, id["Dream"])

	code, out, errs := run(t, lib, "cover", "normalize", "--dry-run")
	if code != 0 || !strings.Contains(out, "1 of 1 cover") || !strings.Contains(out, "nothing was changed") {
		t.Errorf("dry run: code %d, out %q, err %q", code, out, errs)
	}

	code, out, errs = run(t, lib, "cover", "normalize")
	if code != 0 || !strings.Contains(out, "converted 1 of 1 cover") || !strings.Contains(out, "next sync") {
		t.Errorf("normalize: code %d, out %q, err %q", code, out, errs)
	}

	code, out, errs = run(t, lib, "cover", "normalize")
	if code != 0 || !strings.Contains(out, "already") {
		t.Errorf("second run: code %d, out %q, err %q", code, out, errs)
	}
}
