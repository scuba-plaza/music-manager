package device_test

import (
	"testing"

	"music-manager/internal/device"
	"music-manager/internal/manifest"
)

func TestThePocketPlayerTargetWritesTheCardIndex(t *testing.T) {
	pocket, ok := device.TargetOf(manifest.PocketPlayer).(device.Indexer)
	if !ok {
		t.Fatal("the Pocket player target writes no index")
	}
	tracks := []manifest.Track{{ID: "t1", Status: manifest.Identified, Title: "Song", Artist: "Someone"}}
	files := pocket.Index(tracks, nil, map[string]string{"t1": "Someone/Song.mp3"}, nil)
	if _, ok := files["PLAYER/HEADER.BIN"]; !ok {
		t.Fatalf("index files = %v, want PLAYER/HEADER.BIN among them", len(files))
	}
	if pocket.IndexFolder() != "PLAYER" {
		t.Fatalf("index folder = %q", pocket.IndexFolder())
	}
}

func TestOtherTargetsWriteNoIndex(t *testing.T) {
	for _, target := range []device.Target{device.Generic, device.TargetOf(manifest.WolfgangWMP01)} {
		if _, ok := target.(device.Indexer); ok {
			t.Errorf("%s writes a card index", target.ID())
		}
	}
}
