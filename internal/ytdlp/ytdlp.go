// Package ytdlp downloads audio with the yt-dlp command-line tool.
package ytdlp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"music-manager/internal/app"
)

// Downloader runs yt-dlp.
type Downloader struct {
	Bin string // the executable; "yt-dlp" by default
}

func New() *Downloader { return &Downloader{Bin: "yt-dlp"} }

// Download fetches the audio of url as <dir>/<video id>.mp3.
func (d *Downloader) Download(ctx context.Context, url, dir string) (app.Downloaded, error) {
	bin, err := exec.LookPath(d.Bin)
	if err != nil {
		return app.Downloaded{}, fmt.Errorf("%s not found in PATH: install it to download music", d.Bin)
	}
	cmd := exec.CommandContext(ctx, bin,
		"--no-playlist", "-q", "--no-warnings",
		"-x", "--audio-format", "mp3", "--audio-quality", "0",
		"--print-json",
		"-o", filepath.Join(dir, "%(id)s.%(ext)s"),
		url)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.WaitDelay = 2 * time.Second // a killed run may leave children holding the pipes
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return app.Downloaded{}, ctx.Err()
		}
		if msg := lastLine(stderr.String()); msg != "" {
			return app.Downloaded{}, fmt.Errorf("%s: %s", d.Bin, msg)
		}
		return app.Downloaded{}, fmt.Errorf("%s: %w", d.Bin, err)
	}

	info, err := parseInfo(stdout.Bytes())
	if err != nil {
		return app.Downloaded{}, err
	}
	path := filepath.Join(dir, info.id+".mp3")
	if _, err := os.Stat(path); err != nil {
		return app.Downloaded{}, fmt.Errorf("%s did not produce %s", d.Bin, path)
	}
	return app.Downloaded{Path: path, Info: info.Info}, nil
}

type parsed struct {
	id string
	app.Info
}

// parseInfo reads the JSON object yt-dlp prints for the video, ignoring any
// lines around it.
func parseInfo(out []byte) (parsed, error) {
	var raw struct {
		ID          string   `json:"id"`
		Title       string   `json:"title"`
		Track       string   `json:"track"`
		Artist      string   `json:"artist"`
		Artists     []string `json:"artists"`
		Uploader    string   `json:"uploader"`
		Album       string   `json:"album"`
		TrackNumber float64  `json:"track_number"`
		ReleaseDate string   `json:"release_date"`
		ReleaseYear float64  `json:"release_year"`
	}
	var found bool
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "{") {
			if err := json.Unmarshal([]byte(line), &raw); err != nil {
				return parsed{}, fmt.Errorf("unreadable yt-dlp output: %w", err)
			}
			found = true
		}
	}
	if !found || raw.ID == "" {
		return parsed{}, errors.New("yt-dlp printed no video information")
	}
	title := raw.Track
	if title == "" {
		title = raw.Title
	}
	artist := raw.Artist // "A, B" when there are several: a track has one artist, the first
	if len(raw.Artists) > 0 && strings.TrimSpace(raw.Artists[0]) != "" {
		artist = strings.TrimSpace(raw.Artists[0])
	}
	return parsed{id: raw.ID, Info: app.Info{
		Title: title, Artist: artist, Uploader: raw.Uploader, Album: raw.Album, TrackNumber: int(raw.TrackNumber),
		Released: released(raw.ReleaseDate, raw.ReleaseYear),
	}}, nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// released is when the track came out, from yt-dlp's release_date
// (20150904) or release_year, or "" if it does not say. The upload date is
// when the video went up, which says nothing about the music.
func released(date string, year float64) string {
	if len(date) == 8 {
		if d, err := app.NormalizeDate(date[:4] + "-" + date[4:6] + "-" + date[6:]); err == nil {
			return d
		}
	}
	if year > 0 {
		if d, err := app.NormalizeDate(strconv.Itoa(int(year))); err == nil {
			return d
		}
	}
	return ""
}
