package cli

import (
	"fmt"
	"music-manager/internal/app"
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
	return &cobra.Command{
		Use:   "ls [query]",
		Short: "List tracks; words match any title, artist or album, status:<s> filters",
		RunE: func(cmd *cobra.Command, words []string) error {
			a, err := open()
			if err != nil {
				return err
			}
			m, err := a.Manifest()
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tSTATUS\tARTIST\tTITLE\tALBUM\tLENGTH")
			for _, t := range m.Search(strings.Join(words, " ")) {
				artist, title, album := display(t)
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", t.ID, t.Status, artist, title, album, clock(t.PlayingLength()))
			}
			return w.Flush()
		},
	}
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
			row("Length", clock(t.PlayingLength()))
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
			row("Audio", t.AudioSHA256)
			return w.Flush()
		},
	}
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
