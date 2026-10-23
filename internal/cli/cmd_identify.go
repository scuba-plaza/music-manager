package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"music-manager/internal/app"
)

func identifyCmd(open opener, deps Deps) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "identify [id...]",
		Short: "Look tracks up on MusicBrainz and fill in their details",
		Long: `Search MusicBrainz for tracks and record the title, artist, album and track
numbers it knows, plus the album's cover art if the track has none.

Without ids, every track that is not identified yet is looked up, using what
the file said about itself (old tags and file name). In a terminal you pick the
right match for each track from a list. With --yes nothing is asked: the best
match is taken if it is a confident one, and the track is left alone otherwise.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, ids []string) error {
			if deps.Metadata == nil {
				return errors.New("no lookup service available")
			}
			a, err := open()
			if err != nil {
				return err
			}
			results, err := a.Identify(cmd.Context(), ids, deps.Metadata, deps.Prompter, app.IdentifyOptions{Yes: yes})
			var applied, skipped, failed int
			out := cmd.OutOrStdout()
			m, _ := a.Manifest()
			for _, r := range results {
				t, _ := m.Get(r.TrackID)
				switch {
				case r.Err != nil:
					failed++
					fmt.Fprintf(out, "%s  failed: %v\n", r.TrackID, r.Err)
				case r.Applied != nil:
					applied++
					c := r.Applied
					fmt.Fprintf(out, "%s  identified: %s - %s", r.TrackID, t.AsciiArtist, t.AsciiTitle)
					if c.Album != "" {
						fmt.Fprintf(out, " (%s)", c.Album)
					}
					fmt.Fprintln(out)
				default:
					skipped++
					artist, title, _ := display(t)
					fmt.Fprintf(out, "%s  skipped: %s - %s (%s)\n", r.TrackID, artist, title, r.Reason)
				}
			}
			if len(results) > 0 {
				fmt.Fprintf(out, "identified %d, skipped %d, failed %d\n", applied, skipped, failed)
			}
			if err != nil {
				return err
			}
			if failed > 0 {
				return fmt.Errorf("%d lookups failed", failed)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "take confident matches without asking, skip the rest")
	return cmd
}
