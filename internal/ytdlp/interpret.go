package ytdlp

import (
	"regexp"
	"strings"

	"music-manager/internal/app"
)

var noise = map[string]bool{
	"official video": true, "official music video": true, "official audio": true, "official lyric video": true,
	"official visualizer": true, "lyric video": true, "lyrics": true, "audio": true, "video": true,
	"visualizer": true, "music video": true, "mv": true, "hd": true, "hq": true, "4k": true, "1080p": true,
}

var trailingGroup = regexp.MustCompile(`\s*[(\[]([^()\[\]]*)[)\]]\s*$`)

func tidyTitle(title string) string {
	title = strings.TrimSpace(title)
	for {
		found := trailingGroup.FindStringSubmatchIndex(title)
		if found == nil || !noise[strings.ToLower(strings.TrimSpace(title[found[2]:found[3]]))] {
			return title
		}
		rest := strings.TrimSpace(title[:found[0]])
		if rest == "" {
			return title
		}
		title = rest
	}
}

func splitTitle(title string) (artist, song string, ok bool) {
	for _, separator := range []string{" - ", " – ", " — "} {
		left, right, found := strings.Cut(title, separator)
		left, right = strings.TrimSpace(left), strings.TrimSpace(right)
		if found && left != "" && right != "" {
			return left, tidyTitle(right), true
		}
	}
	return "", "", false
}

func firstOf(one string, many []string) string {
	if s := strings.TrimSpace(one); s != "" {
		return s
	}
	for _, s := range many {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

func interpret(raw rawInfo) app.Info {
	info := app.Info{
		Uploader: raw.Uploader, Album: raw.Album, TrackNumber: int(raw.TrackNumber),
		Released: released(raw.ReleaseDate, raw.ReleaseYear), Genre: firstOf(raw.Genre, raw.Genres),
	}
	artist := firstOf("", raw.Artists)
	if artist == "" {
		artist = strings.TrimSpace(raw.Artist)
	}
	track := strings.TrimSpace(raw.Track)
	switch {
	case artist != "":
		info.Artist, info.Source = artist, app.FromMetadata
		info.Title = track
		if info.Title == "" {
			info.Title = tidyTitle(raw.Title)
		}
	case track != "":
		info.Title = track
	default:
		if left, right, ok := splitTitle(strings.TrimSpace(raw.Title)); ok {
			info.Artist, info.Title, info.Source = left, right, app.FromTitle
		} else {
			info.Title = tidyTitle(raw.Title)
		}
	}
	return info
}
