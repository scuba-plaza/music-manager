package tui

import (
	"testing"

	"music-manager/internal/manifest"
)

func TestPlaylistsAreAGroupThatOpensIntoTheirTracksInPlayOrder(t *testing.T) {
	pls := []manifest.Playlist{
		{Name: "Study", Tracks: []string{"t3", "t1"}}, // not in album order, on purpose
		{Name: "Empty", Tracks: []string{}},
	}

	// Closed, there is just the group, ahead of the artists.
	rows := buildRowsWith(sampleTracks, pls, "", map[string]bool{})
	if rows[0].Label != "Playlists" || rows[0].Kind != kindGroup || rows[0].Count != 2 || rows[0].Expanded {
		t.Errorf("first row = %+v, want the closed group with 2 playlists", rows[0])
	}
	if rows[1].Label != "Needs work" {
		t.Errorf("second row = %q, want the sample's Needs work group", rows[1].Label)
	}

	// Open, the playlists come in the order they were made, tracks in play order.
	open := map[string]bool{playlistsKey: true, playlistRowKey("Study"): true}
	rows = buildRowsWith(sampleTracks, pls, "", open)
	got := labels(rows[:5])
	want := []string{"Playlists", "  Study", "    Flow", "    The Wanderer", "  Empty"}
	if !equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
	study := rows[1]
	if study.Kind != kindPlaylist || study.Count != 2 || !study.Expanded || study.Parent != playlistsKey ||
		!equal(study.IDs, []string{"t3", "t1"}) {
		t.Errorf("playlist row = %+v", study)
	}
	if flow := rows[2]; flow.Kind != kindTrack || flow.Parent != study.Key || flow.Num != "01" || flow.Detail != "Shawn James" || flow.Track.ID != "t3" {
		t.Errorf("track row = %+v", flow)
	}
	if rows[3].Num != "02" {
		t.Errorf("second track number = %q", rows[3].Num)
	}

	// The same track in two playlists is two rows with different keys.
	both := []manifest.Playlist{{Name: "A", Tracks: []string{"t1"}}, {Name: "B", Tracks: []string{"t1"}}}
	rows = buildRowsWith(sampleTracks, both, "", map[string]bool{playlistsKey: true, playlistRowKey("A"): true, playlistRowKey("B"): true})
	seen := map[string]bool{}
	for _, r := range rows {
		if seen[r.Key] {
			t.Errorf("two rows share the key %q", r.Key)
		}
		seen[r.Key] = true
	}

	// No playlists, no group.
	for _, r := range buildRowsWith(sampleTracks, nil, "", map[string]bool{}) {
		if r.Kind == kindGroup && r.Label == "Playlists" {
			t.Error("a group of playlists is shown when there are none")
		}
	}
}

func TestFilteringShowsOnlyMatchingTracksAndPlaylistsThatHaveAny(t *testing.T) {
	pls := []manifest.Playlist{
		{Name: "Study", Tracks: []string{"t3", "t4", "t1"}},
		{Name: "Gym", Tracks: []string{"t4"}},
		{Name: "Empty", Tracks: []string{}},
	}
	rows := buildRowsWith(sampleTracks, pls, "shawn", map[string]bool{})
	var inPlaylists []string
	for _, r := range rows {
		if r.Kind == kindGroup && r.Label == "Playlists" {
			if r.Count != 1 {
				t.Errorf("the group counts %d playlists, want only Study", r.Count)
			}
		}
		if r.Parent == playlistRowKey("Study") {
			inPlaylists = append(inPlaylists, r.Track.ID)
		}
		if r.Key == playlistRowKey("Gym") || r.Key == playlistRowKey("Empty") {
			t.Errorf("%q is shown though none of its tracks match", r.Label)
		}
	}
	if !equal(inPlaylists, []string{"t3", "t1"}) {
		t.Errorf("tracks shown in Study = %v, want Flow and The Wanderer, in play order", inPlaylists)
	}
}
