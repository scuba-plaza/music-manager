package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"music-manager/internal/app"
	"music-manager/internal/device"
	"music-manager/internal/manifest"
	"music-manager/internal/render"
	"music-manager/internal/testmp3"
)

func TestSyncToThePocketPlayerWritesTheCardIndexAndPrunesStaleIndexFiles(t *testing.T) {
	a := libWithTracks(t, 100, 101)
	ts := tracks(t, a)
	identify(t, a, ts[0].ID, "Coming Home", "Leon Bridges", "Coming Home", 1, 10)
	if err := a.SetCover(ts[0].ID, testmp3.PNG); err != nil {
		t.Fatal(err)
	}
	card := t.TempDir()
	stale := filepath.Join(card, "PLAYER", "OLD.ORD")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, stale, []byte{1, 2})
	writeFile(t, filepath.Join(card, "notes.txt"), []byte("mine"))

	ops := mustSync(t, a, app.SyncOptions{Target: card, Device: device.TargetOf(manifest.PocketPlayer), Prune: true})

	header := readFile(t, card, "PLAYER/HEADER.BIN")
	if string(header[:4]) != "PKPL" || header[8] != 2 {
		t.Fatalf("header = % x, want PKPL with 2 tracks", header)
	}
	if tints := readFile(t, card, "PLAYER/TINTS.BIN"); len(tints) != 4 {
		t.Errorf("TINTS.BIN = % x, want one tint", tints)
	}
	if !hasOp(ops, render.Op{Kind: render.Remove, Path: "PLAYER/OLD.ORD"}) {
		t.Errorf("ops = %v, want PLAYER/OLD.ORD removed", ops)
	}
	if hasOp(ops, render.Op{Kind: render.Remove, Path: "notes.txt"}) {
		t.Errorf("ops = %v, a file outside the index folder was removed", ops)
	}
	if again := mustSync(t, a, app.SyncOptions{Target: card, Device: device.TargetOf(manifest.PocketPlayer), Prune: true}); len(again) != 0 {
		t.Errorf("second sync ops = %v, want none", again)
	}
}

func TestSyncToOtherTargetsLeavesAFolderCalledPlayerAlone(t *testing.T) {
	a := libWithTracks(t, 100)
	card := t.TempDir()
	if err := os.MkdirAll(filepath.Join(card, "PLAYER"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(card, "PLAYER", "OLD.ORD"), []byte{1})
	ops := mustSync(t, a, app.SyncOptions{Target: card, Prune: true})
	if hasOp(ops, render.Op{Kind: render.Remove, Path: "PLAYER/OLD.ORD"}) {
		t.Errorf("ops = %v, want PLAYER/OLD.ORD left alone", ops)
	}
}

func hasOp(ops []render.Op, want render.Op) bool {
	for _, op := range ops {
		if op == want {
			return true
		}
	}
	return false
}
