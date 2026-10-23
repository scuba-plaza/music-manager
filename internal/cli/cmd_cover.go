package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func coverCmd(open opener) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cover",
		Short: "Look after the cover images",
	}
	cmd.AddCommand(coverNormalizeCmd(open))
	return cmd
}

func coverNormalizeCmd(open opener) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "normalize",
		Short: "Make every cover an image the player opens",
		Long: `The player's photo viewer opens baseline JPEG files of a modest size, and
fails on PNG, progressive and very large ones. New covers are converted when
they are added. This converts the ones already in the library: each is
rewritten as a JPEG of at most 480 pixels along its longer side, and the
tracks that use it point at the new image. Covers that are fine are left as
they are. The old images are deleted by the next sync.

Run "mm sync" afterwards; the old cover.png files left on the player are
strays, and "mm sync --prune" removes them. --dry-run only reports.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := open()
			if err != nil {
				return err
			}
			rep, err := a.NormalizeCovers(dryRun)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			switch {
			case rep.Converted == 0:
				fmt.Fprintf(out, "all %s already suit the player\n", plural(rep.Images, "cover", "covers"))
			case dryRun:
				fmt.Fprintf(out, "would convert %d of %s (used by %s): %s, then %s; nothing was changed\n",
					rep.Converted, plural(rep.Images, "cover", "covers"), plural(rep.Tracks, "track", "tracks"),
					sizeText(rep.Before), sizeText(rep.After))
			default:
				fmt.Fprintf(out, "converted %d of %s (used by %s): %s, now %s; the old images go at the next sync\n",
					rep.Converted, plural(rep.Images, "cover", "covers"), plural(rep.Tracks, "track", "tracks"),
					sizeText(rep.Before), sizeText(rep.After))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "only report what would change")
	return cmd
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// sizeText writes a size in bytes in kilobytes or megabytes.
func sizeText(n int64) string {
	if n < 1<<20 {
		return fmt.Sprintf("%.0f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
}
