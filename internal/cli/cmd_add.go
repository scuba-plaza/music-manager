package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"music-manager/internal/app"
	"music-manager/internal/device"
	"music-manager/internal/layout"
)

func addCmd(open opener, deps Deps) *cobra.Command {
	var tf trackFlags
	var noInput, noLookup bool
	cmd := &cobra.Command{
		Use:   "add <url>",
		Short: "Download a track from YouTube and add it to the library",
		Long: `Download the audio at a URL with yt-dlp and add it to the library.

In a terminal you are asked for each detail of the track (title, artist,
album, track number, number of tracks, release date, and how much to cut off
the start and end), with what yt-dlp reported filled in. Any detail given as a
flag is not asked. With --no-input, or when not in a terminal, nothing is
asked: the flags and what yt-dlp reported are used, and the command fails if
the title or artist is still unknown.

The track is also looked up on MusicBrainz, and a confident match supplies the
defaults instead (--no-lookup skips this). If the lookup fails, the add goes on
without it.

The track is only recorded; run "mm sync" to write it into the library folder.`,
		Example: `  mm add https://youtu.be/abc123
  mm add https://youtu.be/abc123 --no-input --artist "Fairuz" --title "Wtni" --album "Hits" --track 3 --of 12
  mm add https://youtu.be/abc123 --trim-start 0:12 --trim-end 0:05`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := tf.build(cmd.Flags())
			if err != nil {
				return err
			}
			if deps.Downloader == nil {
				return errors.New("no downloader available")
			}
			var prompter app.Prompter
			if !noInput && deps.Prompter != nil {
				prompter = deps.Prompter
			}
			a, err := open()
			if err != nil {
				return err
			}
			req := app.AddRequest{URL: args[0], Edit: p.edit, Cover: p.cover,
				OnWarning: func(err error) { fmt.Fprintln(cmd.ErrOrStderr(), "mm:", err) }}
			if !noLookup {
				req.Source = deps.Metadata
			}
			t, err := a.Add(cmd.Context(), req, deps.Downloader, prompter)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "added %s: %s - %s\n", t.ID, t.OriginalArtist, t.OriginalTitle)
			fmt.Fprintf(out, "  will be written to %s\n", layout.PathFor(t, device.Generic.Layout()))
			fmt.Fprintln(out, `  run "mm sync" to put it in the library folder`)
			return nil
		},
	}
	tf.register(cmd.Flags())
	cmd.Flags().BoolVar(&noInput, "no-input", false, "never ask; use the flags and what yt-dlp reported")
	cmd.Flags().BoolVar(&noLookup, "no-lookup", false, "do not search MusicBrainz for the track")
	return cmd
}
