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
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/device"
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
	// the older track's picture, as it is: a PNG, so a cover.png
	if got := readFile(t, a.Root(), "Band/Album/cover.png"); !bytes.Equal(got, testmp3.PNG) {
		t.Errorf("cover.png = %d bytes, want the older track's %d", len(got), len(testmp3.PNG))
	}
	if _, err := os.Stat(filepath.Join(a.Root(), "Band/Album/cover.jpg")); err == nil {
		t.Error("a cover.jpg was written as well")
	}
	if ops := mustSync(t, a, app.SyncOptions{}); len(ops) != 0 {
		t.Errorf("second sync wants %v", ops)
	}
}

func TestASingleKeepsItsCoverInItsTagAndGetsNoFileOfItsOwn(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Single", "Band", "", 0, 0)
	a.SetCover(ts[0].ID, testmp3.JPEG)
	a.SetCover(ts[1].ID, testmp3.JPEG) // not identified
	ops := mustSync(t, a, app.SyncOptions{})
	for _, op := range ops {
		if op.Kind == render.Create && filepath.Ext(op.Path) != ".mp3" {
			t.Errorf("unexpected file %s", op.Path)
		}
	}
	if got := mp3.ReadTagSet(readFile(t, a.Root(), "Band/Single.mp3")).Cover; !bytes.Equal(got, testmp3.JPEG) {
		t.Error("a single should still embed its cover")
	}
	if ops := mustSync(t, a, app.SyncOptions{}); len(ops) != 0 {
		t.Errorf("second sync wants %v", ops)
	}
}

func TestTheWolfgangGetsAPhotoBesideASingleButNoneForAnUnidentifiedTrack(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Single", "Band", "", 0, 0)
	a.SetCover(ts[0].ID, testmp3.JPEG)
	a.SetCover(ts[1].ID, testmp3.JPEG) // not identified
	target := filepath.Join(t.TempDir(), "player")
	ops := mustSync(t, a, app.SyncOptions{Target: target, Device: device.Wolfgang})
	for _, op := range ops {
		if op.Kind == render.Create && filepath.Ext(op.Path) != ".mp3" && op.Path != "Band/Single.jpg" {
			t.Errorf("unexpected image file %s", op.Path)
		}
	}
	if got := readFile(t, target, "Band/Single.jpg"); !bytes.Equal(got, testmp3.JPEG) {
		t.Errorf("Single.jpg = %d bytes, want the single's cover", len(got))
	}
	if got := mp3.ReadTagSet(readFile(t, target, "Band/Single.mp3")).Cover; !bytes.Equal(got, testmp3.JPEG) {
		t.Error("a single should still embed its cover")
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

// bigPNG is a 1200×800 picture, much larger than a small screen needs.
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

func TestACoverIsStoredExactlyAsGiven(t *testing.T) {
	a := libWithTracks(t, 10)
	tr := tracks(t, a)[0]
	big := bigPNG(t)
	if err := a.SetCover(tr.ID, big); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(big)
	sha := hex.EncodeToString(sum[:])
	if got := tracks(t, a)[0].CoverSHA256; got != sha {
		t.Errorf("cover SHA = %s, want the SHA of the file given, %s", got, sha)
	}
	stored, err := os.ReadFile(filepath.Join(a.Root(), ".mm", "covers", sha+".img"))
	if err != nil || !bytes.Equal(stored, big) {
		t.Errorf("the stored cover is not the file given (%d bytes, %v; want %d)", len(stored), err, len(big))
	}
}

func TestAPlainSyncWritesTheCoverAsItWasGiven(t *testing.T) {
	a := libWithTracks(t, 10)
	tr := tracks(t, a)[0]
	identify(t, a, tr.ID, "Song", "Band", "Album", 1, 1)
	big := bigPNG(t)
	if err := a.SetCover(tr.ID, big); err != nil {
		t.Fatal(err)
	}
	mustSync(t, a, app.SyncOptions{})

	if got := readFile(t, a.Root(), "Band/Album/cover.png"); !bytes.Equal(got, big) {
		t.Errorf("cover.png is %d bytes, want the %d of the picture given", len(got), len(big))
	}
	if _, err := os.Stat(filepath.Join(a.Root(), "Band/Album/cover.jpg")); err == nil {
		t.Error("a cover.jpg was written")
	}
	tags := mp3.ReadTagSet(readFile(t, a.Root(), "Band/Album/01 - Song.mp3"))
	if tags.CoverMIME != "image/png" || !bytes.Equal(tags.Cover, big) {
		t.Errorf("embedded cover is %s, %d bytes; want the PNG given", tags.CoverMIME, len(tags.Cover))
	}
}

func TestTheWolfgangGetsEveryCoverAsASmallJPEG(t *testing.T) {
	a := libWithTracks(t, 10)
	tr := tracks(t, a)[0]
	identify(t, a, tr.ID, "Song", "Band", "Album", 1, 1)
	if err := a.SetCover(tr.ID, bigPNG(t)); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "player")
	mustSync(t, a, app.SyncOptions{Target: target, Device: device.Wolfgang})

	file := readFile(t, target, "Band/Album/cover.jpg")
	cfg, format, err := image.DecodeConfig(bytes.NewReader(file))
	if err != nil || format != "jpeg" || cfg.Width != 480 || cfg.Height != 320 {
		t.Errorf("cover.jpg is %s %dx%d (%v), want a 480x320 jpeg", format, cfg.Width, cfg.Height, err)
	}
	tags := mp3.ReadTagSet(readFile(t, target, "Band/Album/01 - Song.mp3"))
	if tags.CoverMIME != "image/jpeg" || !bytes.Equal(tags.Cover, file) {
		t.Errorf("embedded cover is %s, %d bytes; the file has %d", tags.CoverMIME, len(tags.Cover), len(file))
	}
	if _, err := os.Stat(filepath.Join(target, "Band/Album/cover.png")); err == nil {
		t.Error("a cover.png was written")
	}
}

func TestSetCoverManySetsTheSameCoverOnAllOrNone(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30)
	ts := tracks(t, a)
	ids := []string{ts[0].ID, ts[2].ID}

	got, err := a.SetCoverMany(ids, testmp3.JPEG)
	if err != nil || len(got) != 2 {
		t.Fatalf("got %d tracks, err %v", len(got), err)
	}
	after := tracks(t, a)
	if after[0].CoverSHA256 == "" || after[0].CoverSHA256 != after[2].CoverSHA256 {
		t.Errorf("covers %q and %q, want the same one", after[0].CoverSHA256, after[2].CoverSHA256)
	}
	if after[1].CoverSHA256 != "" {
		t.Error("a track that was not named got the cover")
	}
	if got[ts[0].ID].CoverSHA256 != after[0].CoverSHA256 {
		t.Error("the tracks returned are not the ones saved")
	}

	// A track that does not exist, or a file that is not an image, changes nothing.
	if _, err := a.SetCoverMany([]string{ts[1].ID, "nope"}, testmp3.PNG); err == nil {
		t.Error("an unknown track was accepted")
	}
	if _, err := a.SetCoverMany([]string{ts[1].ID}, []byte("not an image")); err == nil {
		t.Error("a file that is not an image was accepted")
	}
	if tracks(t, a)[1].CoverSHA256 != "" {
		t.Error("a refused change was saved")
	}
}

func TestSetCoverManyWithNoImageTakesTheCoverAway(t *testing.T) {
	a := libWithTracks(t, 10, 20)
	ts := tracks(t, a)
	a.SetCover(ts[0].ID, testmp3.JPEG)
	a.SetCover(ts[1].ID, testmp3.PNG)
	if _, err := a.SetCoverMany([]string{ts[0].ID, ts[1].ID}, nil); err != nil {
		t.Fatal(err)
	}
	for _, tr := range tracks(t, a) {
		if tr.CoverSHA256 != "" {
			t.Errorf("%s still has a cover", tr.ID)
		}
	}
}

func TestAnEditCanGiveTracksTheCoverAnotherTrackHasAndNeverOneThatIsNotStored(t *testing.T) {
	a := libWithTracks(t, 10, 20, 30)
	ts := tracks(t, a)
	if err := a.SetCover(ts[0].ID, testmp3.JPEG); err != nil {
		t.Fatal(err)
	}
	sha := tracks(t, a)[0].CoverSHA256
	artist := "Band"

	got, err := a.EditMany(map[string]app.Edit{
		ts[1].ID: {Cover: &sha, Artist: &artist}, // together with another change
		ts[2].ID: {Cover: &sha},
	})
	if err != nil || len(got) != 2 {
		t.Fatalf("got %d tracks, err %v", len(got), err)
	}
	after := tracks(t, a)
	if after[1].CoverSHA256 != sha || after[2].CoverSHA256 != sha || after[1].Artist != "Band" {
		t.Errorf("tracks after: %+v", after)
	}
	if again := tracks(t, a); again[1].CoverSHA256 != sha {
		t.Error("the change was not saved")
	}

	// A cover that is not in the store is refused, and so is the whole edit.
	unknown := "0000000000000000000000000000000000000000000000000000000000000000"
	other := "Other"
	if _, err := a.EditMany(map[string]app.Edit{ts[0].ID: {Artist: &other}, ts[1].ID: {Cover: &unknown}}); err == nil {
		t.Error("a cover that is not stored was accepted")
	}
	if tracks(t, a)[0].Artist == "Other" {
		t.Error("a refused edit was partly saved")
	}
	if _, err := a.Edit(ts[1].ID, app.Edit{Cover: &unknown}); err == nil {
		t.Error("Edit accepted a cover that is not stored")
	}

	// An empty one takes the cover away.
	none := ""
	if _, err := a.Edit(ts[1].ID, app.Edit{Cover: &none}); err != nil {
		t.Fatal(err)
	}
	if got := tracks(t, a)[1].CoverSHA256; got != "" {
		t.Errorf("cover = %q, want none", got)
	}
}
