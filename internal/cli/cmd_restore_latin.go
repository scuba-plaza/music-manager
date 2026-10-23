package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"music-manager/internal/manifest"
)

func restoreLatinCmd(open opener) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "restore-latin",
		Short: "Put back the Latin forms of names that a migration left out",
		Long: `Put back the Latin forms that were set by hand in the version 1 library.

"mm migrate" keeps them now. A library migrated by an earlier version dropped
them, but the old library.json is kept, byte for byte, in .mm/library.json.v1,
and this reads the forms from it. A form is put back for a title, an artist or
an album only where the track still has the name it was set for and has no
form of its own for it; the rest is left alone and listed. --dry-run lists what
would be put back and writes nothing.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := open()
			if err != nil {
				return err
			}
			rep, err := a.RestoreLatin(dryRun)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(rep.Restored) > 0 {
				header := "Latin forms restored:"
				if dryRun {
					header = "Latin forms that would be restored:"
				}
				fmt.Fprintln(out, header)
				printForms(out, rep.Restored)
			}
			if left := len(rep.Renamed) + len(rep.Kept) + len(rep.Gone); left > 0 {
				fmt.Fprintf(out, "\nleft alone %d: the name was changed since (%d), the track has a form of its own (%d), the track is gone (%d)\n",
					left, len(rep.Renamed), len(rep.Kept), len(rep.Gone))
			}
			switch {
			case len(rep.Restored) == 0:
				fmt.Fprintln(out, "nothing to restore")
			case dryRun:
				fmt.Fprintln(out, "dry run: nothing written")
			default:
				fmt.Fprintf(out, "\nrestored %s on %s\n", plural(len(rep.Restored), "form", "forms"), plural(tracksOf(rep.Restored), "track", "tracks"))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "only list what would be put back")
	return cmd
}

// printForms lists the forms once per name, as "mm migrate" does, with how many
// tracks have it.
func printForms(out interface{ Write([]byte) (int, error) }, hs []manifest.HandSetName) {
	type key struct{ field, name, held string }
	count := map[key]int{}
	var order []key
	for _, h := range hs {
		k := key{h.Field, h.Name, h.Held}
		if count[k] == 0 {
			order = append(order, k)
		}
		count[k]++
	}
	for _, k := range order {
		line := fmt.Sprintf("  %s  %s  %s", k.field, k.name, k.held)
		if n := count[k]; n > 1 {
			line += fmt.Sprintf(" (%d tracks)", n)
		}
		fmt.Fprintln(out, line)
	}
}

// tracksOf is how many different tracks the forms are of.
func tracksOf(hs []manifest.HandSetName) int {
	ids := map[string]bool{}
	for _, h := range hs {
		ids[h.ID] = true
	}
	return len(ids)
}
