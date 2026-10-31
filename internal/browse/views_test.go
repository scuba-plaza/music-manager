package browse

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/device"
	"music-manager/internal/manifest"
)

func snapshot() Snapshot {
	tracks := append([]manifest.Track{}, sampleTracks...)
	return Snapshot{Tracks: tracks, Playlists: []manifest.Playlist{{Name: "Road", Tracks: []string{"t1", "t5"}}}}
}

func TestTheLibraryViewCountsAndFlattens(t *testing.T) {
	v := Library(snapshot(), Options{})
	if v.Tracks != 8 || v.NeedWork != 1 {
		t.Errorf("tracks %d, need work %d", v.Tracks, v.NeedWork)
	}
	var tracks int
	for _, r := range v.Rows {
		if r.Kind == "track" {
			tracks++
			if r.TrackID == "" || r.Length != "3:00" {
				t.Errorf("track row %+v", r)
			}
		}
	}
	if tracks != v.Shown || tracks != 8+2 { // every track once, and two of them again in the playlist
		t.Errorf("track rows %d, shown %d", tracks, v.Shown)
	}
	if len(v.Playlists) != 1 || v.Playlists[0].Count != 2 || v.Playlists[0].Length != "6:00" {
		t.Errorf("playlists %+v", v.Playlists)
	}
}

func TestOnlyWhatCanBeEditedAsAGroupSaysSo(t *testing.T) {
	editable := map[string]bool{}
	for _, r := range Library(snapshot(), Options{}).Rows {
		if r.Kind == "album" || r.Kind == "artist" || r.Kind == "playlist" || r.Kind == "group" {
			editable[r.Kind+":"+r.Label] = r.Editable
		}
	}
	if !editable["artist:Fairuz"] || !editable["album:Shadows"] || !editable["playlist:Road"] {
		t.Errorf("%v", editable)
	}
	if editable["album:Singles"] || editable["group:Needs work"] || editable["group:Playlists"] {
		t.Errorf("%v", editable)
	}
}

func TestASortedLibraryIsOneListWithAWayBackToTheTree(t *testing.T) {
	v := Library(snapshot(), Options{Sort: app.SortTitle})
	for _, r := range v.Rows {
		if r.Kind != "track" || r.Depth != 0 {
			t.Fatalf("a row that is not a flat track: %+v", r)
		}
	}
	if v.Sort.Title != "Library · Title ▲" || v.Sort.Choices[0].Label != "Tree" || len(v.Sort.Choices) != 9 {
		t.Errorf("sort %+v", v.Sort)
	}
	r := Library(snapshot(), Options{Sort: app.SortAdded})
	if !r.Sort.Descending || !strings.HasSuffix(r.Sort.Title, "▼") {
		t.Errorf("dates run newest first: %+v", r.Sort)
	}
	if rev := Library(snapshot(), Options{Sort: app.SortAdded, Reverse: true}); rev.Sort.Descending {
		t.Errorf("reversed: %+v", rev.Sort)
	}
}

func TestATrackHasThreeTabsAndAFourthUnderWolfgang(t *testing.T) {
	s := snapshot()
	v, ok := Track(s, "t1", TrackOptions{})
	if !ok || len(v.Tabs) != 3 || v.Tabs[0].Name != "Details" || v.Tabs[1].Name != "Audio" || v.Tabs[2].Name != "Latin" {
		t.Fatalf("%+v", v.Tabs)
	}
	w, _ := Track(s, "t1", TrackOptions{Wolfgang: true})
	if len(w.Tabs) != 4 || w.Tabs[3].Wolfgang == nil || len(w.Tabs[3].Wolfgang.Rows) == 0 {
		t.Errorf("wolfgang tab: %+v", w.Tabs)
	}
	if _, ok := Track(s, "nope", TrackOptions{}); ok {
		t.Error("found a track that is not there")
	}
	if got := fieldKeys(v.Tabs[0]); strings.Join(got, ",") != "title,artist,album,track,released,genre,cover" {
		t.Errorf("details fields %v", got)
	}
	if got := fieldKeys(v.Tabs[1]); strings.Join(got, ",") != "cutStart,cutEnd,volume,fadeIn,fadeOut" {
		t.Errorf("audio fields %v", got)
	}
}

func fieldKeys(t TabView) []string {
	var keys []string
	for _, f := range t.Fields {
		keys = append(keys, f.Key)
	}
	return keys
}

func TestAnUnidentifiedTrackShowsItsOldTagsDimly(t *testing.T) {
	s := Snapshot{Tracks: []manifest.Track{{ID: "u", Status: manifest.Unidentified, Hint: manifest.Hint{Title: "Old", Artist: "Someone", Album: "Disc", Track: 3}}}}
	v, _ := Track(s, "u", TrackOptions{})
	d := v.Tabs[0]
	if !d.Warn || len(d.Notes) != 1 {
		t.Errorf("notes %+v", d.Notes)
	}
	for _, f := range d.Fields[:4] {
		if !f.Dim || f.Text == "" {
			t.Errorf("%s: %+v", f.Key, f)
		}
	}
	if v.Title != "Old" || v.Identified {
		t.Errorf("%+v", v)
	}
}

func TestTheAudioTabDrawsTheCutsAndTheFades(t *testing.T) {
	tr := trk("a", "A", "B", "C", 1)
	tr.TrimStart, tr.TrimEnd = 10*time.Second, 20*time.Second
	tr.FadeIn = manifest.Span{Start: 10 * time.Second, End: 15 * time.Second}
	v, _ := Track(Snapshot{Tracks: []manifest.Track{tr}}, "a", TrackOptions{})
	bar := v.Tabs[1].Bar
	if bar == nil || bar.Total != int64(3*time.Minute) || bar.Start != int64(10*time.Second) || bar.Kept != "2:30 of 3:00" {
		t.Fatalf("%+v", bar)
	}
	if len(bar.Level) != 100 || bar.Level[0] != -1 || bar.Level[99] != -1 {
		t.Errorf("level %v", bar.Level)
	}
	var quiet, loud bool
	for _, l := range bar.Level {
		quiet = quiet || (l >= 0 && l < 0.5)
		loud = loud || l == 1
	}
	if !quiet || !loud {
		t.Error("the fade does not go from quiet to loud")
	}
	if plain, _ := Track(Snapshot{Tracks: []manifest.Track{trk("b", "A", "B", "C", 1)}}, "b", TrackOptions{}); plain.Tabs[1].Bar.Level != nil {
		t.Error("a level line without fades")
	}
}

func TestALatinFormIsDimUntilSetByHandAndNeedsAName(t *testing.T) {
	tr := trk("a", "سلام", "داود", "", 0)
	tr.ArtistLatin = "Dawud"
	v, _ := Track(Snapshot{Tracks: []manifest.Track{tr}}, "a", TrackOptions{})
	latin := map[string]FieldView{}
	for _, f := range v.Tabs[2].Fields {
		latin[f.Key] = f
	}
	if !latin["latinTitle"].Dim || latin["latinArtist"].Dim || latin["latinArtist"].Text != "Dawud" {
		t.Errorf("%+v", latin)
	}
	if latin["latinAlbum"].Editable || !latin["latinTitle"].Editable {
		t.Errorf("a form of a name that is empty cannot be edited: %+v", latin["latinAlbum"])
	}
	if !strings.Contains(latin["latinTitle"].Hint, "سلام") {
		t.Errorf("hint %q", latin["latinTitle"].Hint)
	}
}

func TestAPeekListsWhatIsInsideARow(t *testing.T) {
	s := snapshot()
	artist, ok := Peek(s, artistKey("Fairuz"), "")
	if !ok || artist.Title != "Artist" || len(artist.List) != 2 || artist.List[0].Left != "Jerusalem in My Heart" || artist.List[1].Left != "Singles" {
		t.Fatalf("%+v", artist)
	}
	if artist.List[0].Key != albumRowKey("Fairuz", "Jerusalem in My Heart") || artist.List[1].Key != singlesKey("Fairuz") {
		t.Errorf("keys %+v", artist.List)
	}
	album, _ := Peek(s, albumRowKey("Shawn James", "Shadows"), "")
	if len(album.List) != 3 || album.List[0].Left != " 1  The Wanderer" || album.List[2].Left != "12  Along Our Way" {
		t.Errorf("%+v", album.List)
	}
	pl, _ := Peek(s, playlistRowKey("Road"), "")
	if pl.Kind != "playlist" || pl.Path != "Playlists/Road.m3u8" || len(pl.List) != 2 || pl.List[0].Cols[0] != "Shawn James" {
		t.Errorf("%+v", pl)
	}
	needs, _ := Peek(s, keyNeedsWork, "")
	if needs.Title != "Needs work" || len(needs.List) != 1 || needs.List[0].Cols[0] != "no title, no artist" {
		t.Errorf("%+v", needs)
	}
	group, _ := Peek(s, playlistsKey, "")
	if len(group.List) != 1 || group.List[0].Left != "Road" {
		t.Errorf("%+v", group)
	}
	if _, ok := Peek(s, trackKey("t1"), ""); ok {
		t.Error("a track is not a peek")
	}
	if _, ok := Peek(s, artistKey("Nobody"), ""); ok {
		t.Error("found an artist that is not there")
	}
}

func TestAPeekFollowsTheFilter(t *testing.T) {
	artist, ok := Peek(snapshot(), artistKey("Shawn James"), "flow")
	if !ok || len(artist.List) != 1 || !strings.HasPrefix(artist.Info[0], "1 track") {
		t.Errorf("%+v", artist)
	}
}

func tabWithID(v TrackView, id string) (TabView, int) {
	for i, tab := range v.Tabs {
		if tab.ID == id {
			return tab, i
		}
	}
	return TabView{}, -1
}

func rowLabels(v *DeviceView) []string {
	var labels []string
	for _, r := range v.Rows {
		labels = append(labels, r.Label)
	}
	return labels
}

func longNamed() Snapshot {
	s := snapshot()
	s.Tracks = append(s.Tracks, trk("long", "A Title That Runs On Far Past Twenty", "Fairuz", "Jerusalem in My Heart", 2))
	return s
}

func userRegistry(t *testing.T, content string) *device.Registry {
	t.Helper()
	path := filepath.Join(t.TempDir(), "devices.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := device.LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestTheDeviceViewMatchesTheLegacyWolfgangViewForTheWolfgangProfile(t *testing.T) {
	s := longNamed()
	for _, id := range []string{"long", "t4", "u1"} {
		v, _ := Track(s, id, TrackOptions{Wolfgang: true, Profile: "wolfgang-wmp01"})
		legacy, at := tabWithID(v, "wolfgang")
		if at != 3 || legacy.Name != "Wolfgang" || legacy.Wolfgang == nil {
			t.Fatalf("%s: the legacy tab is not the fourth: %+v", id, v.Tabs)
		}
		dev, at := tabWithID(v, "device")
		if at != 4 || dev.Name != "Device" || dev.Device == nil || dev.Wolfgang != nil {
			t.Fatalf("%s: device tab %+v", id, v.Tabs)
		}
		if !reflect.DeepEqual(dev.Device, legacy.Wolfgang) {
			t.Errorf("%s: device view %+v, legacy %+v", id, dev.Device, legacy.Wolfgang)
		}
	}
	v, _ := Track(s, "long", TrackOptions{Profile: "wolfgang-wmp01"})
	dev, _ := tabWithID(v, "device")
	if got := strings.Join(rowLabels(dev.Device), ","); got != "Folder,File,Cut,Artist,Album,Genre,Title" {
		t.Errorf("rows %s", got)
	}
	for _, r := range dev.Device.Rows {
		if r.Label == "Cut" && (len([]rune(r.Value)) != 20 || !strings.Contains(r.Hint, "24 characters") || !strings.Contains(r.Hint, "cut at 20")) {
			t.Errorf("cut row %+v", r)
		}
		if r.Label == "Album" && (r.Value != "Jerusalem" || r.Tail != "· 9 of 12") {
			t.Errorf("album row %+v", r)
		}
	}
}

func TestTheDeviceTabIsThereOnlyWhenAProfileIsAsked(t *testing.T) {
	v, _ := Track(snapshot(), "t1", TrackOptions{Wolfgang: true})
	if _, at := tabWithID(v, "device"); at != -1 || len(v.Tabs) != 4 {
		t.Errorf("tabs %+v", v.Tabs)
	}
	ids := []string{}
	w, _ := Track(snapshot(), "t1", TrackOptions{Profile: "generic"})
	for _, tab := range w.Tabs {
		ids = append(ids, tab.ID)
	}
	if strings.Join(ids, ",") != "details,audio,latin,device" {
		t.Errorf("tab ids %v", ids)
	}
	r := userRegistry(t, "[[profile]]\nname = \"my-player\"\nlatin_names = true\n")
	if _, ok := Track(snapshot(), "t1", TrackOptions{Profile: "nobody", Profiles: r}); ok {
		t.Error("an unknown profile previews something")
	}
}

func TestTheDevicePreviewOfAProfileWithoutAStemMaxHasNoCutRow(t *testing.T) {
	r := userRegistry(t, "[[profile]]\nname = \"my-player\"\nlatin_names = true\n")
	v, _ := Track(longNamed(), "long", TrackOptions{Profile: "my-player", Profiles: r})
	dev, _ := tabWithID(v, "device")
	if got := strings.Join(rowLabels(dev.Device), ","); got != "Folder,File,Artist,Album,Genre,Title" {
		t.Errorf("rows %s", got)
	}
	for _, row := range dev.Device.Rows {
		if row.Label == "Title" && row.Hint != "" {
			t.Errorf("a name that is not cut has nothing to say about the title: %+v", row)
		}
	}
}

func TestTheDevicePreviewOfAProfileWithoutATagWidthCountsNothing(t *testing.T) {
	r := userRegistry(t, "[[profile]]\nname = \"my-player\"\nlatin_names = true\n")
	v, _ := Track(longNamed(), "t4", TrackOptions{Profile: "my-player", Profiles: r})
	dev, _ := tabWithID(v, "device")
	for _, row := range dev.Device.Rows {
		if row.Tail != "" {
			t.Errorf("row %+v has a tail without a tag width", row)
		}
		if row.Label == "Album" && row.Value != "Caught a Black Rabbit" {
			t.Errorf("album %+v, want it whole", row)
		}
	}
}
