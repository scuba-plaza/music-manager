package ytdlp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"music-manager/internal/app"
)

type rawListing struct {
	Title   string `json:"title"`
	Entries []struct {
		ID       string  `json:"id"`
		Title    string  `json:"title"`
		Duration float64 `json:"duration"`
		Channel  string  `json:"channel"`
		Uploader string  `json:"uploader"`
	} `json:"entries"`
}

func (d *Downloader) List(ctx context.Context, url, dir string, limit int) (app.Listing, error) {
	out, err := d.run(ctx,
		"--flat-playlist", "-J", "-q", "--no-warnings",
		"--playlist-end", strconv.Itoa(limit),
		"--", url)
	if err != nil {
		return app.Listing{}, err
	}
	var raw rawListing
	if err := json.Unmarshal(out, &raw); err != nil {
		return app.Listing{}, fmt.Errorf("unreadable yt-dlp output: %w", err)
	}
	listing := app.Listing{Title: raw.Title}
	for _, e := range raw.Entries {
		if e.ID == "" {
			continue
		}
		listing.Tracks = append(listing.Tracks, app.ListedTrack{
			URL:    "https://www.youtube.com/watch?v=" + e.ID,
			Title:  e.Title,
			Length: time.Duration(e.Duration * float64(time.Second)),
		})
	}
	if len(raw.Entries) > 0 {
		listing.Uploader = raw.Entries[0].Channel
		if listing.Uploader == "" {
			listing.Uploader = raw.Entries[0].Uploader
		}
	}
	if _, err := d.run(ctx,
		"--flat-playlist", "--skip-download",
		"--write-thumbnail", "--convert-thumbnails", "jpg",
		"--playlist-items", "0", "-q", "--no-warnings",
		"-o", "pl_thumbnail:"+filepath.Join(dir, "cover"),
		"-o", filepath.Join(dir, "%(id)s"),
		"--", url); err == nil {
		listing.Cover, _ = os.ReadFile(filepath.Join(dir, "cover.jpg"))
	}
	return listing, nil
}
