package app_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/coverimg"
	"music-manager/internal/manifest"
	"music-manager/internal/mp3"
	"music-manager/internal/render"
	"music-manager/internal/testmp3"
)

func TestIngestKeepsEmbeddedArtworkAndSyncWritesIt(t *testing.T) {
	a := openLib(t)
	f := filepath.Join(t.TempDir(), "x.mp3")
	writeFile(t, f, testmp3.WithCover(testmp3.Silent(10), "image/jpeg", testmp3.JPEG))
	if _, err := a.Ingest([]string{f}); err != nil {
		t.Fatal(err)
	}
	tr := tracks(t, a)[0]
	if tr.CoverSHA256 == "" {
		t.Fatal("embedded cover was dropped at ingest")
	}

	identify(t, a, tr.ID, "Song", "Band", "Album", 1, 1)
	mustSync(t, a, app.SyncOptions{})

	tags := mp3.ReadTagSet(readFile(t, a.Root(), "Band/Album/01 - Song.mp3"))
	if !bytes.Equal(tags.Cover, testmp3.JPEG) || tags.CoverMIME != "image/jpeg" {
		t.Errorf("embedded cover = %q (%s)", tags.Cover, tags.CoverMIME)
	}
	if got := readFile(t, a.Root(), "Band/Album/cover.jpg"); !bytes.Equal(got, testmp3.JPEG) {
		t.Errorf("cover.jpg = %q", got)
	}
}

func TestAlbumFolderCoverComesFromTheOldestTrackThatHasOne(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30)
	ts := tracks(t, a)
	for _, tr := range ts {
		identify(t, a, tr.ID, "Song "+tr.ID, "Band", "Album", 1, 3)
	}
	if err := a.SetCover(ts[1].ID, testmp3.PNG); err != nil {
		t.Fatal(err)
	}
	if err := a.SetCover(ts[2].ID, testmp3.JPEG); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, app.SyncOptions{})
	want, err := coverimg.Normalize(testmp3.PNG) // the older track's picture, in the form covers are kept in
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, a.Root(), "Band/Album/cover.jpg"); !bytes.Equal(got, want) {
		t.Errorf("cover.jpg = %d bytes, want the older track's %d", len(got), len(want))
	}
	if ops := mustSync(t, a, app.SyncOptions{}); len(ops) != 0 {
		t.Errorf("second sync wants %v", ops)
	}
}

func TestASingleGetsItsCoverBesideItsFileButAnUnidentifiedTrackGetsNone(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Single", "Band", "", 0, 0)
	a.SetCover(ts[0].ID, testmp3.JPEG)
	a.SetCover(ts[1].ID, testmp3.JPEG) // not identified
	ops := mustSync(t, a, app.SyncOptions{})
	for _, op := range ops {
		if op.Kind == render.Create && filepath.Ext(op.Path) != ".mp3" && op.Path != "Band/Single.jpg" {
			t.Errorf("unexpected image file %s", op.Path)
		}
	}
	if got := readFile(t, a.Root(), "Band/Single.jpg"); !bytes.Equal(got, testmp3.JPEG) {
		t.Errorf("Single.jpg = %d bytes, want the single's cover", len(got))
	}
	if got := mp3.ReadTagSet(readFile(t, a.Root(), "Band/Single.mp3")).Cover; !bytes.Equal(got, testmp3.JPEG) {
		t.Error("a single should still embed its cover")
	}
	if ops := mustSync(t, a, app.SyncOptions{}); len(ops) != 0 {
		t.Errorf("second sync wants %v", ops)
	}
}

func TestChangingAndRemovingACover(t *testing.T) {
	a := libWithTracks(t, 10)
	tr := tracks(t, a)[0]
	identify(t, a, tr.ID, "Song", "Band", "Album", 1, 1)
	a.SetCover(tr.ID, testmp3.JPEG)
	mustSync(t, a, app.SyncOptions{})

	a.SetCover(tr.ID, testmp3.JPEG2)
	mustSync(t, a, app.SyncOptions{})
	if got := readFile(t, a.Root(), "Band/Album/cover.jpg"); !bytes.Equal(got, testmp3.JPEG2) {
		t.Errorf("cover.jpg = %q after replacing", got)
	}

	if err := a.SetCover(tr.ID, nil); err != nil {
		t.Fatal(err)
	}
	ops := mustSync(t, a, app.SyncOptions{Prune: true})
	removed := 0
	for _, op := range ops {
		if op.Kind == render.Remove && op.Path == "Band/Album/cover.jpg" {
			removed++
		}
	}
	if removed != 1 || len(mp3.ReadTagSet(readFile(t, a.Root(), "Band/Album/01 - Song.mp3")).Cover) != 0 {
		t.Errorf("cover not removed everywhere: ops %v", ops)
	}
}

func TestSetCoverRejectsNonImagesAndUnknownTracks(t *testing.T) {
	a := libWithTracks(t, 10)
	id := tracks(t, a)[0].ID
	if err := a.SetCover(id, []byte("just some text")); err == nil {
		t.Error("accepted a text file as a cover")
	}
	if err := a.SetCover("nope", testmp3.JPEG); err == nil {
		t.Error("accepted an unknown track")
	}
	if tracks(t, a)[0].CoverSHA256 != "" {
		t.Error("a rejected cover was recorded")
	}
}

// bigPNG is a 1200×800 picture, much larger than a cover needs to be.
func bigPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 1200, 800))
	for y := 0; y < 800; y++ {
		for x := 0; x < 1200; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestAnyCoverIsSyncedAsASmallJPEGThePlayerOpens(t *testing.T) {
	a := libWithTracks(t, 10)
	tr := tracks(t, a)[0]
	identify(t, a, tr.ID, "Song", "Band", "Album", 1, 1)
	if err := a.SetCover(tr.ID, bigPNG(t)); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, app.SyncOptions{})

	file := readFile(t, a.Root(), "Band/Album/cover.jpg")
	cfg, format, err := image.DecodeConfig(bytes.NewReader(file))
	if err != nil || format != "jpeg" || cfg.Width != 480 || cfg.Height != 320 {
		t.Errorf("cover.jpg is %s %dx%d (%v), want a 480x320 jpeg", format, cfg.Width, cfg.Height, err)
	}
	tags := mp3.ReadTagSet(readFile(t, a.Root(), "Band/Album/01 - Song.mp3"))
	if tags.CoverMIME != "image/jpeg" || !bytes.Equal(tags.Cover, file) {
		t.Errorf("embedded cover is %s, %d bytes; the file has %d", tags.CoverMIME, len(tags.Cover), len(file))
	}
	if _, err := os.Stat(filepath.Join(a.Root(), "Band/Album/cover.png")); err == nil {
		t.Error("a cover.png was written")
	}
}

// plantLegacyCover gives a track a cover as an earlier version stored it:
// the image just as it came, whatever its form.
func plantLegacyCover(t *testing.T, a *app.App, id string, image []byte) string {
	t.Helper()
	sum := sha256.Sum256(image)
	sha := hex.EncodeToString(sum[:])
	dir := filepath.Join(a.Root(), ".mm", "covers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, sha+".img"), image)
	store := manifest.NewStore(filepath.Join(a.Root(), "library.json"))
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
	return sha
}

func TestACoverStoredAsAPNGLongAgoIsStillSyncedAsAJPEG(t *testing.T) {
	a := libWithTracks(t, 10)
	tr := tracks(t, a)[0]
	identify(t, a, tr.ID, "Song", "Band", "Album", 1, 1)
	plantLegacyCover(t, a, tr.ID, bigPNG(t))
	mustSync(t, a, app.SyncOptions{})

	file := readFile(t, a.Root(), "Band/Album/cover.jpg")
	if cfg, format, err := image.DecodeConfig(bytes.NewReader(file)); err != nil || format != "jpeg" || cfg.Width != 480 {
		t.Errorf("cover.jpg is %s %dx%d (%v)", format, cfg.Width, cfg.Height, err)
	}
	if tags := mp3.ReadTagSet(readFile(t, a.Root(), "Band/Album/01 - Song.mp3")); tags.CoverMIME != "image/jpeg" || !bytes.Equal(tags.Cover, file) {
		t.Errorf("embedded cover is %s, %d bytes", tags.CoverMIME, len(tags.Cover))
	}
}

func coverShas(t *testing.T, a *app.App) []string {
	t.Helper()
	var out []string
	for _, tr := range tracks(t, a) {
		out = append(out, tr.CoverSHA256)
	}
	return out
}

func TestNormalizingTheCoversOfALibraryKeptByAnEarlierVersion(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30)
	ts := tracks(t, a)
	old := plantLegacyCover(t, a, ts[0].ID, bigPNG(t))
	plantLegacyCover(t, a, ts[1].ID, bigPNG(t)) // the same image: one blob, two users
	if err := a.SetCover(ts[2].ID, testmp3.JPEG); err != nil {
		t.Fatal(err)
	}
	before := coverShas(t, a)

	rep, err := a.NormalizeCovers(true)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Images != 2 || rep.Converted != 1 || rep.Tracks != 2 || rep.Before == 0 || rep.After == 0 {
		t.Errorf("dry run reports %+v", rep)
	}
	if !slices.Equal(coverShas(t, a), before) {
		t.Error("a dry run changed the library")
	}

	if rep, err = a.NormalizeCovers(false); err != nil {
		t.Fatal(err)
	}
	if rep.Converted != 1 || rep.Tracks != 2 {
		t.Errorf("report %+v", rep)
	}
	after := coverShas(t, a)
	if after[0] == old || after[0] != after[1] || after[2] != before[2] {
		t.Errorf("cover shas %v became %v: the two PNG users should share one new image, the JPEG stay", before, after)
	}
	data, err := os.ReadFile(filepath.Join(a.Root(), ".mm", "covers", after[0]+".img"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg, format, err := image.DecodeConfig(bytes.NewReader(data)); err != nil || format != "jpeg" || cfg.Width != 480 {
		t.Errorf("new image is %s %dx%d (%v)", format, cfg.Width, cfg.Height, err)
	}
	if _, err := os.Stat(filepath.Join(a.Root(), ".mm", "library.json.before-covers")); err != nil {
		t.Errorf("no backup of the library: %v", err)
	}

	if rep, err = a.NormalizeCovers(false); err != nil || rep.Converted != 0 || rep.Tracks != 0 {
		t.Errorf("a second run reports %+v, %v; want nothing to do", rep, err)
	}
}
