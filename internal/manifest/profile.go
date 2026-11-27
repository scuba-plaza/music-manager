package manifest

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type Profile struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	LatinNames    bool   `json:"latin_names"`
	ComponentMax  int    `json:"component_max"`
	StemMax       int    `json:"stem_max"`
	TagWidth      int    `json:"tag_width"`
	LatinGenre    bool   `json:"latin_genre"`
	CoverFormat   string `json:"cover_format"`
	CoverMaxEdge  int    `json:"cover_max_edge"`
	CoverQuality  int    `json:"cover_quality"`
	SingleCovers  bool   `json:"single_covers"`
	ReplayGain    bool   `json:"replaygain"`
	PlaylistExt   string `json:"playlist_ext"`
	PlaylistStyle string `json:"playlist_style"`
	CardIndex     bool   `json:"card_index,omitempty"`
}

const (
	MaxProfiles         = 8
	MaxProfileNameRunes = 40
	MinComponentMax     = 20
	MaxComponentMax     = 255
	MaxTagWidth         = 255
	MaxCoverEdge        = 4096
	MaxCoverQuality     = 100
	GenericName         = "Generic"
)

var WolfgangWMP01 = Profile{
	ID: "wolfgang-wmp01", Name: "Wolfgang WMP01",
	LatinNames: true, ComponentMax: 100, StemMax: 20, TagWidth: 12, LatinGenre: true,
	CoverFormat: "baseline-jpeg", CoverMaxEdge: 480, CoverQuality: 85, SingleCovers: true,
	ReplayGain: false, PlaylistExt: ".m3u", PlaylistStyle: "root-backslash",
}

var PocketPlayer = Profile{
	ID: "pocket-player", Name: "Pocket player",
	LatinNames: true, ComponentMax: 80, CoverFormat: "keep", ReplayGain: true,
	PlaylistExt: ".m3u8", PlaylistStyle: "relative", CardIndex: true,
}

func (p Profile) Problems() map[string]string {
	bad := map[string]string{}
	name := strings.TrimSpace(p.Name)
	switch {
	case name == "":
		bad["name"] = "Name cannot be empty"
	case utf8.RuneCountInString(name) > MaxProfileNameRunes:
		bad["name"] = fmt.Sprintf("Name can be at most %d characters", MaxProfileNameRunes)
	case strings.EqualFold(name, GenericName):
		bad["name"] = "Generic is the standard format and cannot be changed: pick another name"
	}
	if p.ComponentMax < MinComponentMax || p.ComponentMax > MaxComponentMax {
		bad["component_max"] = fmt.Sprintf("Must be a whole number from %d to %d", MinComponentMax, MaxComponentMax)
	}
	if p.StemMax < 0 || (p.ComponentMax >= MinComponentMax && p.StemMax >= p.ComponentMax) {
		bad["stem_max"] = fmt.Sprintf("Must be 0 for no limit, or below the longest name (%d)", p.ComponentMax)
	}
	switch {
	case p.TagWidth < 0 || p.TagWidth > MaxTagWidth:
		bad["tag_width"] = fmt.Sprintf("Must be a whole number from 0 to %d", MaxTagWidth)
	case p.TagWidth > 0 && !p.LatinNames:
		bad["tag_width"] = "Needs names in Latin letters: turn that on, or use 0"
	}
	switch p.CoverFormat {
	case "keep":
	case "baseline-jpeg":
		if p.CoverMaxEdge < 1 || p.CoverMaxEdge > MaxCoverEdge {
			bad["cover_max_edge"] = fmt.Sprintf("Must be a whole number of pixels from 1 to %d", MaxCoverEdge)
		}
		if p.CoverQuality < 1 || p.CoverQuality > MaxCoverQuality {
			bad["cover_quality"] = fmt.Sprintf("Must be a whole number from 1 to %d", MaxCoverQuality)
		}
	default:
		bad["cover_format"] = "Must be keep or baseline-jpeg"
	}
	if p.PlaylistExt != ".m3u" && p.PlaylistExt != ".m3u8" {
		bad["playlist_ext"] = "Must be .m3u or .m3u8"
	}
	if p.PlaylistStyle != "relative" && p.PlaylistStyle != "root-backslash" {
		bad["playlist_style"] = "Must be relative or root-backslash"
	}
	if len(bad) == 0 {
		return nil
	}
	return bad
}
