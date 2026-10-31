package device

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"reflect"
	"testing"
	"time"

	"music-manager/internal/layout"
	"music-manager/internal/manifest"
	"music-manager/internal/mp3"
)

const recordedOutputs = "testdata/targets.golden.json"

type recordedLayout struct {
	ComponentMax int
	StemMax      int
	Latin        bool
	Fold         []string
}

type recordedPlaylist struct {
	File string
	Name string
	Data string
}

type recordedCover struct {
	Stored string
	SHA256 string
	MIME   string
	Err    string
}

type recordedTarget struct {
	Name         string
	Layout       recordedLayout
	Paths        map[string]string
	Tags         map[string]mp3.TagSet
	CoverFile    map[string]string
	SingleCovers bool
	ReplayGain   bool
	Playlists    []recordedPlaylist
	Diagnose     []Warning
	Covers       []recordedCover
}

var foldSamples = []string{"Fairuz", "فيروز", "Beyoncé", "Ærø — Straße", "🎵", "Mohamed Abdel Wahab"}

func fixtureTracks() []manifest.Track {
	identified := func(id, artist, album, title string) manifest.Track {
		return manifest.Track{ID: id, Status: manifest.Identified, Artist: artist, Album: album, Title: title, SourceLength: 200 * time.Second}
	}
	album := identified("album1", "Nils Frahm", "Spaces", "Says")
	album.TrackNumber, album.TrackTotal = 3, 10
	album.Released, album.Genre = "2013-11-15", "Neoclassical"
	album.SourceURL, album.MBRecordingID, album.MBReleaseID = "https://example.org/says", "rec-1", "rel-1"
	album.TrimStart, album.TrimEnd = 5*time.Second, 7*time.Second
	single := identified("single", "Max Richter", "", "On the Nature of Daylight")
	single.Genre = "Ambient"
	inbox := manifest.Track{ID: "inbox1", Status: manifest.Unidentified, Hint: manifest.Hint{Filename: "01 x.mp3", Title: "x"}}
	arabic := identified("arabic", "عبد الحليم حافظ", "روائع", "بحلم بيك")
	arabic.Genre, arabic.TrackNumber, arabic.TrackTotal = "طرب", 2, 6
	handSet := identified("handst", "عبد الحليم حافظ", "روائع", "بحلم بيك")
	handSet.Genre, handSet.TrackNumber, handSet.TrackTotal = "طرب", 1, 6
	handSet.TitleLatin, handSet.ArtistLatin, handSet.AlbumLatin, handSet.GenreLatin = "Bahlam Bek", "Abdel Halim Hafez", "Rawaa'e", "Tarab"
	arabicSingle := identified("arsing", "فيروز", "", "وطني")
	long := identified("long80", "Nils Frahm", "Spaces", "An Eighty Character Title That Goes On And On Well Past What Any Player Shows Ok")
	live := identified("live01", "The Band (Live)", "The Last Waltz (Live)", "The Weight")
	various := identified("varios", "Various Artists", "Now That's What I Call Music Volume One", "Song")
	wahab := identified("wahab1", "Mohamed Abdel Wahab", "Ahwak", "Ahwak")
	ep := identified("ep0001", "Some Long Artist Name", "Name - EP", "Opening")
	genre := identified("genre1", "Rock Band", "", "Loud")
	genre.Genre = "Alternative Rock Pop"
	cutA := identified("cuta01", "Artist", "", "A Very Long Song Title Indeed Yes")
	cutB := identified("cutb01", "Artist", "", "A Very Long Song Title Indeed No")
	duo := identified("duo001", "Fairuz & Ziad Rahbani", "", "Song")
	emoji := identified("emoji1", "🎵🎶", "", "Song")
	twelve := identified("twelve", "Fairouz Band", "Songs (Live)", "Exactly Twelve")
	return []manifest.Track{album, single, inbox, arabic, handSet, arabicSingle, long, live, various, wahab, ep, genre, cutA, cutB, duo, emoji, twelve}
}

func fixturePlaylists(tracks []manifest.Track) []manifest.Playlist {
	var all []string
	for _, t := range tracks {
		all = append(all, t.ID)
	}
	return []manifest.Playlist{
		{Name: "دراسة", Tracks: append(all, "gone")},
		{Name: "دراسة ٢", NameLatin: "Study Two", Tracks: []string{"handst", "inbox1", "album1"}},
		{Name: "Road Trip", Tracks: []string{"single", "cuta01", "cutb01"}},
		{Name: "Empty"},
	}
}

func encodedImage(t *testing.T, edge int, asPNG bool) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, edge, edge*2/3))
	for y := range edge * 2 / 3 {
		for x := range edge {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x), uint8(y), uint8(x ^ y), 255})
		}
	}
	var b bytes.Buffer
	var err error
	if asPNG {
		err = png.Encode(&b, img)
	} else {
		err = jpeg.Encode(&b, img, &jpeg.Options{Quality: 90})
	}
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func fixtureCovers(t *testing.T) []struct {
	name string
	data []byte
} {
	return []struct {
		name string
		data []byte
	}{
		{"png", encodedImage(t, 600, true)},
		{"jpeg", encodedImage(t, 300, false)},
		{"jpeg-3000", encodedImage(t, 3000, false)},
		{"not-an-image", []byte("GIF89a not a cover")},
	}
}

func record(t *testing.T, target Target) recordedTarget {
	t.Helper()
	tracks := fixtureTracks()
	byID := map[string]manifest.Track{}
	rules := target.Layout()
	r := recordedTarget{
		Name:         target.Name(),
		Layout:       recordedLayout{ComponentMax: rules.ComponentMax, StemMax: rules.StemMax, Latin: rules.Latin},
		Paths:        layout.Assign(tracks, rules),
		Tags:         map[string]mp3.TagSet{},
		CoverFile:    map[string]string{},
		SingleCovers: target.SingleCovers(),
		ReplayGain:   target.ReplayGain(),
		Diagnose:     target.Diagnose(tracks),
	}
	if rules.Fold != nil {
		for _, s := range foldSamples {
			r.Layout.Fold = append(r.Layout.Fold, rules.Fold(s))
		}
	}
	for _, tr := range tracks {
		byID[tr.ID] = tr
		r.Tags[tr.ID] = target.Tags(tr)
	}
	for _, mime := range []string{"image/jpeg", "image/png"} {
		r.CoverFile[mime] = target.CoverFile(mime)
	}
	for _, p := range fixturePlaylists(tracks) {
		name, data := target.Playlist(p, byID, r.Paths)
		r.Playlists = append(r.Playlists, recordedPlaylist{File: target.PlaylistFile(p), Name: name, Data: string(data)})
	}
	for _, c := range fixtureCovers(t) {
		data, mime, err := target.Cover(c.data)
		sum := sha256.Sum256(data)
		rc := recordedCover{Stored: c.name, SHA256: hex.EncodeToString(sum[:]), MIME: mime}
		if err != nil {
			rc.Err = err.Error()
		}
		r.Covers = append(r.Covers, rc)
	}
	return r
}

func compareRecordings(t *testing.T, got, want recordedTarget) {
	t.Helper()
	g, gv := reflect.ValueOf(got), reflect.ValueOf(want)
	for i := range g.NumField() {
		if !reflect.DeepEqual(g.Field(i).Interface(), gv.Field(i).Interface()) {
			gotJSON, _ := json.MarshalIndent(g.Field(i).Interface(), "", "  ")
			wantJSON, _ := json.MarshalIndent(gv.Field(i).Interface(), "", "  ")
			t.Errorf("%s differs:\ngot  %s\nwant %s", g.Type().Field(i).Name, gotJSON, wantJSON)
		}
	}
}

func TestBuiltinTargetsMatchTheRecordedOutputs(t *testing.T) {
	data, err := os.ReadFile(recordedOutputs)
	if err != nil {
		t.Fatal(err)
	}
	var recorded map[string]recordedTarget
	if err := json.Unmarshal(data, &recorded); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"generic", "wolfgang-wmp01"} {
		t.Run(name, func(t *testing.T) {
			want, ok := recorded[name]
			if !ok {
				t.Fatalf("%s has no outputs for %q", recordedOutputs, name)
			}
			encoded, err := json.Marshal(record(t, builtinTarget(name)))
			if err != nil {
				t.Fatal(err)
			}
			var got recordedTarget
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatal(err)
			}
			compareRecordings(t, got, want)
		})
	}
}
