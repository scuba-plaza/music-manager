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
		if err := generate(dir); err != nil {
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
