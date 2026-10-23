package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
)

func editCmd(open opener) *cobra.Command {
	var tf trackFlags
	var gain, fadeIn, fadeOut string
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Change the details of a track",
		Long: `Change the details of a track. Only the flags you pass are changed.

Title, artist and album are the true names, in any script. The --title-latin,
--artist-latin, --album-latin and --genre-latin flags set how each reads in Latin
letters, which a player that cannot show the script gets (mm sync --wolfgang);
where none is set a form is worked out, and "" goes back to it.
Title, artist and album are the true names, in any script. Trims accept 90, 1:30 or 1:30.5. The release date
takes a year, a month or a day (1999, 1999-05, 1999-05-17); pass "" to forget it.
The source link (--source-url) is where the track came from; pass "" to forget it.
A track with a title and an artist counts as identified.

--gain makes the track louder or quieter by about that many dB (3, -2.5,
+6dB), rounded to the nearest step the file allows without being decoded,
1.5 dB, up to 24 dB either way; 0 takes it away. Boosting can clip.
--fade-in and --fade-out take how long the fade lasts (3, 0:02.5), counted
from where the track now starts or back from where it now ends, or the stretch
of the original it covers (0:02-0:05); "" or 0 takes it away. Before a fade-in
and after a fade-out there is silence. The fades are kept as times in the
original, so cutting the track again afterwards does not move them.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			p, err := tf.build(f)
			if err != nil {
				return err
			}
			if f.Changed("gain") {
				db, err := app.ParseGain(gain)
				if err != nil {
					return fmt.Errorf("--gain: %w", err)
				}
				p.edit.GainDB = &db
			}
			fades := f.Changed("fade-in") || f.Changed("fade-out")
			if p.edit == (app.Edit{}) && !p.coverChanged && !fades {
				return errors.New("nothing to change: pass at least one flag, see --help")
			}

			a, err := open()
			if err != nil {
				return err
			}
			if fades {
				m, err := a.Manifest()
				if err != nil {
					return err
				}
				t, ok := m.Get(args[0])
				if !ok {
					return fmt.Errorf("no track with id %q", args[0])
				}
				if p.edit.TrimStart != nil { // fades count from the cuts as they will be
					t.TrimStart = *p.edit.TrimStart
				}
				if p.edit.TrimEnd != nil {
					t.TrimEnd = *p.edit.TrimEnd
				}
				for _, fade := range []struct {
					flag, text string
					in         bool
					dst        **manifest.Span
				}{{"fade-in", fadeIn, true, &p.edit.FadeIn}, {"fade-out", fadeOut, false, &p.edit.FadeOut}} {
					if !f.Changed(fade.flag) {
						continue
					}
					span, err := app.ParseFade(fade.text, fade.in, t)
					if err != nil {
						return fmt.Errorf("--%s: %w", fade.flag, err)
					}
					*fade.dst = &span
				}
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
	tf.registerLatin(cmd.Flags())
	cmd.Flags().StringVar(&tf.sourceURL, "source-url", "", "the link the track came from; \"\" for none")
	cmd.Flags().StringVar(&gain, "gain", "", "make the track louder or quieter by about this many dB, e.g. 3 or -2.5; 0 for no change")
	cmd.Flags().StringVar(&fadeIn, "fade-in", "", "fade in over this long from where the track starts, e.g. 3, or over a stretch, e.g. 0:02-0:05; \"\" for none")
	cmd.Flags().StringVar(&fadeOut, "fade-out", "", "fade out over this long back from where the track ends, or over a stretch; \"\" for none")
	return cmd
}
