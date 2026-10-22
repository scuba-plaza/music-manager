package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"music-manager/internal/app"
)

func rmCmd(open opener) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>...",
		Short: "Remove tracks from the library",
		Long: `Remove the records of tracks. Their stored audio is deleted by the next sync,
and their rendered files by "mm sync --prune".`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, ids []string) error {
			a, err := open()
			if err != nil {
				return err
			}
			m, err := a.Manifest()
			if err != nil {
				return err
			}
			for _, id := range ids { // check everything first, so a typo removes nothing
				if _, ok := m.Get(id); !ok {
					return fmt.Errorf("no track with id %q; nothing was removed", id)
				}
			}
			for _, id := range ids {
				t, err := a.Remove(id)
				if err != nil {
					return err
				}
				artist, title, _ := display(t)
				fmt.Fprintf(cmd.OutOrStdout(), "removed %s: %s - %s\n", t.ID, artist, title)
			}
			return nil
		},
	}
}

func doctorCmd(open opener) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the library for damage and loose ends",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := open()
			if err != nil {
				return err
			}
			problems, err := a.Doctor()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(problems) == 0 {
				fmt.Fprintln(out, "no problems found")
				return nil
			}
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			errors := 0
			for _, p := range problems {
				if p.Severity == app.Error {
					errors++
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", p.Severity, p.TrackID, p.Message)
			}
			w.Flush()
			if errors > 0 {
				return fmt.Errorf("%d errors found", errors)
			}
			return nil
		},
	}
}
