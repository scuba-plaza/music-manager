package main

import (
	"fmt"
	"os"
	"path/filepath"

	"music-manager/internal/device"
	"music-manager/internal/layout"
	"music-manager/internal/manifest"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: pocket-index <library folder> <card folder>")
		os.Exit(2)
	}
	if err := write(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func write(library, card string) error {
	m, err := manifest.NewStore(filepath.Join(library, "library.json")).Load()
	if err != nil {
		return err
	}
	target := device.TargetOf(manifest.PocketPlayer)
	ix := target.(device.Indexer)
	covers := map[string][]byte{}
	for _, t := range m.Tracks {
		if t.CoverSHA256 == "" || covers[t.CoverSHA256] != nil {
			continue
		}
		if data, err := os.ReadFile(filepath.Join(library, ".mm", "covers", t.CoverSHA256+".img")); err == nil {
			covers[t.CoverSHA256] = data
		}
	}
	folder := filepath.Join(card, ix.IndexFolder())
	if err := os.RemoveAll(folder); err != nil {
		return err
	}
	paths := layout.Assign(m.Tracks, target.Layout())
	for name, data := range ix.Index(m.Tracks, m.Playlists, paths, covers) {
		file := filepath.Join(card, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file, data, 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("%d tracks, %d playlists, %d covers\n", len(m.Tracks), len(m.Playlists), len(covers))
	return nil
}
