package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/device"
	"music-manager/internal/manifest"
	"music-manager/internal/testmp3"
)

type album struct {
	artist, name, genre, released string
	titles                        []string
}

var albums = []album{
	{"Amber Lanes", "Harbor Lights", "Indie", "2019-04-12", []string{"Harbor Lights", "Paper Boats", "Low Tide", "Lantern", "Drift"}},
	{"Bruno Castell", "Night Train", "Jazz", "2012", []string{"Night Train", "Blue Station", "Midnight Platform", "Last Call"}},
	{"Cora Vale", "Wildflower", "Folk", "2021-09", []string{"Wildflower", "River Song", "Barn Dance", "Hollow"}},
	{"Cora Vale", "Early Days", "Folk", "2016", []string{"Morning", "Bramble"}},
	{"Delta Nine", "Static", "Rock", "2008", []string{"Static", "Wires", "Overdrive", "Burnout", "Signal", "4 AM"}},
	{"Echo Park Strings", "Scores Vol. 1", "Soundtrack", "2018", []string{"Opening Titles", "Chase", "Quiet Room", "Finale"}},
	{"Zed Harlow", "Quartz", "Electronic", "2024-02-02", []string{"Quartz", "Xenon", "Ultraviolet", "Kinetic"}},
	{"Bruno Castell", "", "Jazz", "2023", []string{"Brooklyn"}},
}

var playlists = []struct {
	name   string
	titles []string
}{
	{"Road trip", []string{"Overdrive", "Harbor Lights", "Night Train", "Wildflower", "Kinetic", "Signal", "Drift", "Brooklyn"}},
	{"Focus", []string{"Quiet Room", "Low Tide", "Hollow", "Xenon", "Opening Titles"}},
	{"Night drive", nil},
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: pocket-fixture <card folder>")
		os.Exit(2)
	}
	if err := generate(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(card string) error {
	work, err := os.MkdirTemp("", "pocket-fixture-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	library := filepath.Join(work, "library")
	a, err := app.Open(library)
	if err != nil {
		return err
	}
	source := filepath.Join(work, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		return err
	}

	ids := map[string]string{}
	var order []string
	for _, al := range albums {
		for n, title := range al.titles {
			file := filepath.Join(source, fmt.Sprintf("%02d.mp3", len(order)))
			if err := os.WriteFile(file, testmp3.Silent(100+len(order)), 0o644); err != nil {
				return err
			}
			report, err := a.Ingest([]string{file})
			if err != nil {
				return err
			}
			if len(report.Added) != 1 {
				return fmt.Errorf("%s was not added", title)
			}
			id := report.Added[0].ID
			number, total := n+1, len(al.titles)
			if al.name == "" {
				number, total = 0, 0
			}
			edit := app.Edit{
				Title: &title, Artist: &al.artist, Album: &al.name,
				TrackNumber: &number, TrackTotal: &total,
				Genre: &al.genre, Released: &al.released,
			}
			if _, err := a.Edit(id, edit); err != nil {
				return err
			}
			ids[title] = id
			order = append(order, id)
		}
	}

	first := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	err = manifest.NewStore(filepath.Join(library, "library.json")).Update(func(m *manifest.Manifest) error {
		for i := range m.Tracks {
			for n, id := range order {
				if m.Tracks[i].ID == id {
					m.Tracks[i].AddedAt = first.AddDate(0, 0, 3*n)
					m.Tracks[i].SourceLength = time.Duration(150+(n*37)%180) * time.Second
					m.Tracks[i].TrimStart, m.Tracks[i].TrimEnd = 0, 0
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	for _, p := range playlists {
		var members []string
		for _, title := range p.titles {
			members = append(members, ids[title])
		}
		if err := a.CreatePlaylist(p.name, members...); err != nil {
			return err
		}
	}

	plan, err := a.PlanSync(app.SyncOptions{Target: card, Device: device.TargetOf(manifest.PocketPlayer), Prune: true})
	if err != nil {
		return err
	}
	return plan.ApplyWith(nil)
}
