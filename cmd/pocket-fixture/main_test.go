package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestTheFixtureCardIsTheSameOnEveryRun(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	for _, dir := range []string{first, second} {
		if err := generate(dir, false); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(first, "PLAYER"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 20 {
		t.Fatalf("PLAYER holds %d files", len(entries))
	}
	for _, e := range entries {
		a, _ := os.ReadFile(filepath.Join(first, "PLAYER", e.Name()))
		b, err := os.ReadFile(filepath.Join(second, "PLAYER", e.Name()))
		if err != nil || !bytes.Equal(a, b) {
			t.Errorf("PLAYER/%s differs between runs", e.Name())
		}
	}
	header, _ := os.ReadFile(filepath.Join(first, "PLAYER", "HEADER.BIN"))
	if header[8] != 30 {
		t.Errorf("tracks = %d, want 30", header[8])
	}
}

func TestTheArabicCardAddsTwoAlbumsSortedByTheirLatinSpelling(t *testing.T) {
	dir := t.TempDir()
	if err := generate(dir, true); err != nil {
		t.Fatal(err)
	}
	header, _ := os.ReadFile(filepath.Join(dir, "PLAYER", "HEADER.BIN"))
	if header[8] != 36 || header[10] != 9 || header[12] != 8 {
		t.Errorf("tracks, albums, artists = %d, %d, %d, want 36, 9, 8", header[8], header[10], header[12])
	}
	letters, _ := os.ReadFile(filepath.Join(dir, "PLAYER", "ARTAZ.LET"))
	if l, s := letters[2*11], letters[2*18]; l == letters[2*12] || s == letters[2*19] {
		t.Errorf("no artist under L or S: % x", letters)
	}
}
