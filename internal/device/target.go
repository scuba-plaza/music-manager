package device

import (
	_ "embed"
	"errors"
	"fmt"
	"math"
	"path"
	"strings"

	"music-manager/internal/coverimg"
	"music-manager/internal/layout"
	"music-manager/internal/manifest"
	"music-manager/internal/mp3"
	"music-manager/internal/translit"
)

//go:embed builtin.toml
var builtinTOML []byte

var builtinProfiles = mustParseProfiles(builtinTOML)

var Generic = builtinTarget("generic")

var Wolfgang = builtinTarget("wolfgang-wmp01")

func mustParseProfiles(data []byte) []Profile {
	profiles, err := ParseProfiles(data)
	if err != nil {
		panic(err)
	}
	return profiles
}

func builtinTarget(name string) Target {
	for _, p := range builtinProfiles {
		if p.Name == name {
			return p.Target()
		}
	}
	panic("builtin.toml has no profile " + name)
}

type profileTarget struct{ p Profile }

func (p Profile) Target() Target { return profileTarget{p} }

func (t profileTarget) Name() string { return t.p.Name }

func (t profileTarget) Profile() Profile { return t.p }

func (t profileTarget) Layout() layout.Rules {
	rules := layout.Rules{ComponentMax: t.p.ComponentMax, StemMax: t.p.StemMax}
	if t.p.LatinNames {
		rules.Latin, rules.Fold = true, translit.ToASCII
	}
	return rules
}

func (t profileTarget) Tags(tr manifest.Track) mp3.TagSet {
	if tr.Status != manifest.Identified {
		return mp3.TagSet{Extra: extras(tr)}
	}
	tags := mp3.TagSet{
		Title: tr.Title, Artist: tr.Artist, Album: tr.Album,
		Genre: tr.Genre,
		Track: tr.TrackNumber, Total: tr.TrackTotal,
		Date:   tr.Released,
		Length: tr.PlayingLength(),
		Extra:  extras(tr),
	}
	if t.p.TagWidth > 0 {
		tags.Artist, tags.Album = PlayerTags(tr, t.p.TagWidth)
	}
	if t.p.LatinGenre {
		tags.Genre = tr.LatinGenre()
	}
	return tags
}

func (t profileTarget) Cover(stored []byte) ([]byte, string, error) {
	if t.p.CoverFormat == "baseline-jpeg" {
		data, err := coverimg.Baseline(stored, t.p.CoverMaxEdge, t.p.CoverQuality)
		return data, "image/jpeg", err
	}
	mime := coverimg.Sniff(stored)
	if mime == "" {
		return nil, "", errors.New("cover must be a JPEG or PNG image")
	}
	return stored, mime, nil
}

func (t profileTarget) CoverFile(storedMIME string) string {
	if t.p.CoverFormat == "keep" && storedMIME == "image/png" {
		return "cover.png"
	}
	return "cover.jpg"
}

func (t profileTarget) SingleCovers() bool { return t.p.SingleCovers }

func (t profileTarget) ReplayGain() bool { return t.p.ReplayGain }

func (t profileTarget) PlaylistFile(p manifest.Playlist) string {
	name := p.Name
	if t.p.LatinNames {
		name = p.LatinName()
	}
	return layout.PlaylistPath(name, t.p.PlaylistExt, t.Layout())
}

func (t profileTarget) Playlist(p manifest.Playlist, tracks map[string]manifest.Track, paths map[string]string) (string, []byte) {
	if t.p.PlaylistStyle == "root-backslash" {
		return t.PlaylistFile(p), rootBackslashPlaylist(p, paths)
	}
	return t.PlaylistFile(p), relativePlaylist(p, tracks, paths)
}

func rootBackslashPlaylist(p manifest.Playlist, paths map[string]string) []byte {
	var b strings.Builder
	for _, id := range p.Tracks {
		if file, ok := paths[id]; ok {
			b.WriteString(`\` + strings.ReplaceAll(file, "/", `\`) + "\r\n")
		}
	}
	return []byte(b.String())
}

func relativePlaylist(p manifest.Playlist, tracks map[string]manifest.Track, paths map[string]string) []byte {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	for _, id := range p.Tracks {
		file, ok := paths[id]
		if !ok {
			continue
		}
		t := tracks[id]
		seconds := int(math.Round(t.PlayingLength().Seconds()))
		if seconds <= 0 {
			seconds = -1
		}
		name := strings.TrimSuffix(path.Base(file), path.Ext(file))
		if t.Status == manifest.Identified {
			name = t.Artist + " - " + t.Title
		}
		fmt.Fprintf(&b, "#EXTINF:%d,%s\n../%s\n", seconds, name, file)
	}
	return []byte(b.String())
}
