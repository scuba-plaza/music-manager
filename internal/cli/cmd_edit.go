package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"music-manager/internal/app"
)

func editCmd(open opener) *cobra.Command {
	var tf trackFlags
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Change the details of a track",
		Long: `Change the details of a track. Only the flags you pass are changed.

Title, artist and album take the original text; their ASCII forms are derived
automatically. Use --title-ascii (etc.) to set one by hand, or pass "" to go
back to the derived form. Trims accept 90, 1:30 or 1:30.5. The release date
takes a year, a month or a day (1999, 1999-05, 1999-05-17); pass "" to forget it.
The source link (--source-url) is where the track came from; pass "" to forget it.
The player keeps only 12 characters of the artist and of the album: mm shortens
long ones itself, and --player-artist and --player-album say what to show
instead; pass "" to go back to the shortened real name.
A track with a title and an artist counts as identified.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := tf.build(cmd.Flags())
			if err != nil {
				return err
			}
			if p.edit == (app.Edit{}) && !p.coverChanged {
				return errors.New("nothing to change: pass at least one flag, see --help")
			}

			a, err := open()
			if err != nil {
				return err
			}
			if p.edit != (app.Edit{}) {
				if _, err := a.Edit(args[0], p.edit); err != nil {
					return err
				}
			}
			if p.coverChanged {
				if err := a.SetCover(args[0], p.cover); err != nil {
					return err
				}
			}
			m, err := a.Manifest()
			if err != nil {
				return err
			}
			t, _ := m.Get(args[0])
			artist, title, _ := display(t)
			fmt.Fprintf(cmd.OutOrStdout(), "updated %s: %s - %s (%s)\n", t.ID, artist, title, t.Status)
			return nil
		},
	}
	tf.register(cmd.Flags())
	cmd.Flags().StringVar(&tf.sourceURL, "source-url", "", "the link the track came from; \"\" for none")
	cmd.Flags().StringVar(&tf.playerArtist, "player-artist", "", "the artist as the player shows it (it keeps 12 characters); \"\" to shorten the real one")
	cmd.Flags().StringVar(&tf.playerAlbum, "player-album", "", "the album as the player shows it (it keeps 12 characters); \"\" to shorten the real one")
	return cmd
}
