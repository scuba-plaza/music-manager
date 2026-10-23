package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"music-manager/internal/app"
)

func reviewCmd(open opener) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Apply a file of proposals you have read and corrected",
	}
	cmd.AddCommand(reviewApplyCmd(open))
	return cmd
}

func reviewApplyCmd(open opener) *cobra.Command {
	var opts app.ReviewOptions
	cmd := &cobra.Command{
		Use:   "apply <file>",
		Short: "Set the values a review file proposes",
		Long: `A review file proposes one field for many tracks at once, with how sure
each proposal is, so that you can read it, correct it and only then apply it:

  ## ARTISTS -> genre

  Fairouz  (10 tracks)  [high ; the old tags]
      Arabic

An artist's value goes to all its tracks, an album's (## ALBUMS) to all the
tracks of the album and a track's (## TRACKS, by ID) to that track; the more
specific one wins. A value of "-" or nothing skips the entry. Entries marked
low are skipped unless you pass --include-low.

All the values are checked first: if one is invalid, nothing is changed.
Applying the same file twice changes nothing the second time.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			entries, err := app.ParseReview(string(text))
			if err != nil {
				return fmt.Errorf("%s: %w", args[0], err)
			}
			a, err := open()
			if err != nil {
				return err
			}
			rep, err := a.ApplyReview(entries, opts)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			verb := "changed"
			if opts.DryRun {
				verb = "would change"
			}
			fmt.Fprintf(out, "%s %s with %s", verb, plural(rep.Changed, "track", "tracks"), plural(rep.Applied, "entry", "entries"))
			if rep.Low > 0 {
				fmt.Fprintf(out, ", %d low left out", rep.Low)
			}
			if rep.Blank > 0 {
				fmt.Fprintf(out, ", %d without a value", rep.Blank)
			}
			fmt.Fprintln(out)
			for _, u := range rep.Unmatched {
				fmt.Fprintf(out, "no track for %s\n", u)
			}
			if opts.DryRun {
				fmt.Fprintln(out, "nothing was changed")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "only report what would change")
	cmd.Flags().BoolVar(&opts.IncludeLow, "include-low", false, "apply the proposals marked low too")
	return cmd
}
