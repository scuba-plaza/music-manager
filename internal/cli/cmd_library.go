package cli

import (
	"fmt"
	"music-manager/internal/app"
	"music-manager/internal/device"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"music-manager/internal/manifest"
	"music-manager/internal/timecode"
)

func ingestCmd(open opener) *cobra.Command {
	return &cobra.Command{
		Use:   "ingest <file|dir>...",
		Short: "Add MP3 files to the library as unidentified tracks",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, paths []string) error {
			a, err := open()
			if err != nil {
				return err
			}
			rep, err := a.Ingest(paths)
			if err != nil {
				return err
			}
			for _, f := range rep.Failed {
				fmt.Fprintf(cmd.ErrOrStderr(), "skipped %s: %v\n", f.Path, f.Err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %d, %d duplicate, %d failed\n",
				len(rep.Added), len(rep.Duplicates), len(rep.Failed))
			return nil
		},
	}
}

func lsCmd(open opener) *cobra.Command {
	var sortBy string
	var reverse bool
	cmd := &cobra.Command{
		Use:   "ls [query]",
		Short: "List tracks; words match any title, artist, album or genre, status:<s> and genre:<g> filter",
		Long: `List tracks. Words match any title, artist, album or genre; status:<s> filters by
status and genre:<g> by genre (genre:folk finds Folk and Folk Rock).

--sort puts the tracks in order by artist, album, title, track, length,
released, added or edited. Text and numbers sort smallest first; the three
dates sort newest first. Tracks with no value for the key always come last.
--reverse turns the order around. Sorting by a date shows that date.`,
		Example: `  mm ls --sort added            the tracks added last, first
  mm ls --sort released --reverse   the oldest recordings first
  mm ls --sort title shawn`,
		RunE: func(cmd *cobra.Command, words []string) error {
			var key app.SortKey
			if cmd.Flags().Changed("sort") {
				var err error
				if key, err = app.ParseSortKey(sortBy); err != nil {
					return fmt.Errorf("--sort: %w", err)
				}
			}
			a, err := open()
			if err != nil {
				return err
			}
			m, err := a.Manifest()
			if err != nil {
				return err
			}
			found := m.Search(strings.Join(words, " "))
			if key != "" {
				found = app.SortTracks(found, key, reverse)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			header := "ID\tSTATUS\tARTIST\tTITLE\tALBUM\tLENGTH"
			if key.IsDate() {
				header += "\t" + strings.ToUpper(string(key))
			}
			fmt.Fprintln(w, header)
			for _, t := range found {
				artist, title, album := display(t)
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s", t.ID, t.Status, artist, title, album, clock(t.PlayingLength()))
				if key.IsDate() {
					v := app.SortValue(t, key)
					if v == "" {
						v = "-"
					}
					fmt.Fprint(w, "\t"+v)
				}
				fmt.Fprintln(w)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&sortBy, "sort", "", "order by artist, album, title, track, length, released, added or edited")
	cmd.Flags().BoolVar(&reverse, "reverse", false, "turn the order around")
	return cmd
}

func showCmd(open opener) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show everything recorded about a track",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := open()
			if err != nil {
				return err
			}
			m, err := a.Manifest()
			if err != nil {
				return err
			}
			t, ok := m.Get(args[0])
			if !ok {
				return fmt.Errorf("no track with id %q", args[0])
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			row := func(k, v string) {
				if v != "" {
					fmt.Fprintf(w, "%s\t%s\n", k, v)
				}
			}
			row("ID", t.ID)
			row("Status", string(t.Status))
			row("Title", named(t.Title, t.OriginalTitle))
			row("Artist", named(t.Artist, t.OriginalArtist))
			row("Album", named(t.Album, t.OriginalAlbum))
			if t.TrackNumber > 0 {
				n := fmt.Sprint(t.TrackNumber)
				if t.TrackTotal > 0 {
					n += fmt.Sprintf("/%d", t.TrackTotal)
				}
				row("Track", n)
			}
			row("Released", t.Released)
			row("Genre", t.Genre)
			row("Player artist", t.PlayerArtist)
			row("Player album", t.PlayerAlbum)
			if t.Status == manifest.Identified {
				if artist, album := device.PlayerTags(t); artist != t.Artist || album != t.Album {
					row("On the player", artist+" · "+album)
				}
			}
			row("Length", clock(t.PlayingLength()))
			if t.GainSteps != 0 {
				row("Volume", app.VolumeText(t.GainSteps)+" ("+plural(max(t.GainSteps, -t.GainSteps), "step", "steps")+")")
			}
			row("Fade in", app.FadeText(t.FadeIn, true, t))
			row("Fade out", app.FadeText(t.FadeOut, false, t))
			if t.TrimStart > 0 || t.TrimEnd > 0 {
				row("Trim", fmt.Sprintf("start %s, end %s (of %s)",
					timecode.Format(t.TrimStart), timecode.Format(t.TrimEnd), timecode.Format(t.SourceLength.Round(time.Millisecond))))
			}
			row("Source", t.SourceURL)
			if t.Status == manifest.Unidentified {
				row("Hint title", t.Hint.Title)
				row("Hint artist", t.Hint.Artist)
				row("Hint album", t.Hint.Album)
				row("Hint file", t.Hint.Filename)
			}
			row("Added", stamp(t.AddedAt))
			row("Edited", stamp(t.UpdatedAt))
			row("Audio", t.AudioSHA256)
			return w.Flush()
		},
	}
}

// stamp formats a moment in local time, or gives "" for an unknown one.
func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04")
}

// named shows a name with its original when the two differ.
func named(ascii, orig string) string {
	if orig == "" || orig == ascii {
		return ascii
	}
	return fmt.Sprintf("%s (original: %s)", ascii, orig)
}

// display is app.Display under a shorter name.
var display = app.Display

// clock formats d as m:ss, rounding down.
func clock(d time.Duration) string {
	s := int(d / time.Second)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}
