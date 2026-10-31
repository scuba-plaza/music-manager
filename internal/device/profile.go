package device

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type Profile struct {
	Name          string `toml:"name"`
	Label         string `toml:"label"`
	LatinNames    bool   `toml:"latin_names"`
	ComponentMax  int    `toml:"component_max"`
	StemMax       int    `toml:"stem_max"`
	TagWidth      int    `toml:"tag_width"`
	LatinGenre    bool   `toml:"latin_genre"`
	CoverFormat   string `toml:"cover_format"`
	CoverMaxEdge  int    `toml:"cover_max_edge"`
	CoverQuality  int    `toml:"cover_quality"`
	SingleCovers  bool   `toml:"single_covers"`
	ReplayGain    bool   `toml:"replaygain"`
	PlaylistExt   string `toml:"playlist_ext"`
	PlaylistStyle string `toml:"playlist_style"`
}

const (
	minComponentMax = 20
	maxCoverQuality = 100
)

var profileName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type document struct {
	Profile []map[string]any `toml:"profile"`
}

func ParseProfiles(data []byte) ([]Profile, error) {
	var doc document
	if err := decodeStrictly(data, &doc); err != nil {
		return nil, errors.New(describeDecodeError(err))
	}
	profiles := make([]Profile, 0, len(doc.Profile))
	for position, table := range doc.Profile {
		p, err := profileFromTable(position, table)
		if err != nil {
			return nil, err
		}
		if err := p.Validate(); err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

func profileFromTable(position int, table map[string]any) (Profile, error) {
	p := Profile{
		ComponentMax:  200,
		CoverFormat:   "keep",
		ReplayGain:    true,
		PlaylistExt:   ".m3u8",
		PlaylistStyle: "relative",
	}
	text, err := toml.Marshal(table)
	if err == nil {
		err = decodeStrictly(text, &p)
	}
	if err != nil {
		return Profile{}, fmt.Errorf("profile %s: %s", describeProfile(position, table), describeDecodeError(err))
	}
	if p.Label == "" {
		p.Label = p.Name
	}
	return p, nil
}

func decodeStrictly(data []byte, target any) error {
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func describeProfile(position int, table map[string]any) string {
	if name, ok := table["name"].(string); ok && name != "" {
		return fmt.Sprintf("%q", name)
	}
	return fmt.Sprintf("#%d", position+1)
}

func describeDecodeError(err error) string {
	var unknown *toml.StrictMissingError
	if errors.As(err, &unknown) {
		return strings.Join(unknown.Errors[0].Key(), ".") + " unknown key"
	}
	var invalid *toml.DecodeError
	if errors.As(err, &invalid) && len(invalid.Key()) > 0 {
		return strings.Join(invalid.Key(), ".") + " has a value of the wrong type"
	}
	return err.Error()
}

func (p Profile) Validate() error {
	fail := func(key, problem string, args ...any) error {
		return fmt.Errorf("profile %q: %s %s", p.Name, key, fmt.Sprintf(problem, args...))
	}
	if !profileName.MatchString(p.Name) {
		return fail("name", "must be lower-case letters, digits and hyphens, starting with a letter or digit")
	}
	if p.ComponentMax < minComponentMax {
		return fail("component_max", "must be at least %d, not %d", minComponentMax, p.ComponentMax)
	}
	if p.StemMax < 0 || p.StemMax >= p.ComponentMax {
		return fail("stem_max", "must be from 0 up to below component_max (%d), not %d", p.ComponentMax, p.StemMax)
	}
	if p.TagWidth < 0 {
		return fail("tag_width", "must not be negative, not %d", p.TagWidth)
	}
	if p.TagWidth > 0 && !p.LatinNames {
		return fail("tag_width", "needs latin_names")
	}
	switch p.CoverFormat {
	case "keep":
	case "baseline-jpeg":
		if p.CoverMaxEdge < 1 {
			return fail("cover_max_edge", "must be at least 1 for baseline-jpeg, not %d", p.CoverMaxEdge)
		}
		if p.CoverQuality < 1 || p.CoverQuality > maxCoverQuality {
			return fail("cover_quality", "must be from 1 to %d for baseline-jpeg, not %d", maxCoverQuality, p.CoverQuality)
		}
	default:
		return fail("cover_format", "must be keep or baseline-jpeg, not %q", p.CoverFormat)
	}
	switch p.PlaylistExt {
	case ".m3u", ".m3u8":
	default:
		return fail("playlist_ext", "must be .m3u or .m3u8, not %q", p.PlaylistExt)
	}
	switch p.PlaylistStyle {
	case "relative", "root-backslash":
	default:
		return fail("playlist_style", "must be relative or root-backslash, not %q", p.PlaylistStyle)
	}
	return nil
}
