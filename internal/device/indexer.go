package device

import (
	"music-manager/internal/manifest"
	"music-manager/internal/pocket"
)

type Indexer interface {
	IndexFolder() string
	Index(tracks []manifest.Track, playlists []manifest.Playlist, paths map[string]string) map[string][]byte
}

type indexedTarget struct{ profileTarget }

func (indexedTarget) IndexFolder() string { return pocket.Folder }

func (indexedTarget) Index(tracks []manifest.Track, playlists []manifest.Playlist, paths map[string]string) map[string][]byte {
	return pocket.Files(pocket.Library{Tracks: tracks, Playlists: playlists, Paths: paths})
}
