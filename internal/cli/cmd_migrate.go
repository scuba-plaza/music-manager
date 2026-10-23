package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"music-manager/internal/manifest"
)

func migrateCmd(open opener) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Bring a library of an older version up to date",
		Long: `Bring a library of an older version up to date.

Version 1 kept every name twice, in Latin letters (maybe set by hand) and as it
really is, and the names the Wolfgang WMP01 shows per track. Version 2 keeps the
true names and nothing for any one device: a Latin form is worked out from the
name when a device needs one, and the 12-character names of the Wolfgang from
those.

The old library.json is kept, byte for byte, in .mm/library.json.v1, so no
name or setting is lost. --dry-run lists the Latin forms that were set by hand
and are no longer used, and the player names that go, and writes nothing.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := open()
			if err != nil {
				return err
			}
			rep, err := a.Migrate(dryRun)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			printMigration(out, rep)
			if dryRun {
				fmt.Fprintln(out, "dry run: nothing written")
				return nil
			}
			fmt.Fprintf(out, "migrated %s to version 2; the old library is kept in %s\n", plural(rep.Tracks, "track", "tracks"), a.MigrationBackup())
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "only list what would change")
	return cmd
}

// printMigration lists the Latin forms that were set by hand, once per name,
// and the player names that go.
func printMigration(out interface{ Write([]byte) (int, error) }, rep *manifest.MigrationReport) {
	fmt.Fprintf(out, "%s in the library.\n", plural(rep.Tracks, "track", "tracks"))
	type key struct{ field, name, held, now string }
	count := map[key]int{}
	var order []key
	for _, h := range rep.HandSet {
		k := key{h.Field, h.Name, h.Held, h.Now}
		if count[k] == 0 {
			order = append(order, k)
		}
		count[k]++
	}
	if len(order) > 0 {
		fmt.Fprintf(out, "\nThese Latin forms were set by hand and are not kept; the Wolfgang gets the worked-out form instead:\n")
		for _, k := range order {
			line := fmt.Sprintf("  %s  %s  %s → %s", k.field, k.name, k.held, k.now)
			if n := count[k]; n > 1 {
				line += fmt.Sprintf(" (%d tracks)", n)
			}
			fmt.Fprintln(out, line)
		}
	}
	var gone []string
	if rep.PlayerArtists > 0 {
		gone = append(gone, "artist names on "+plural(rep.PlayerArtists, "track", "tracks"))
	}
	if rep.PlayerAlbums > 0 {
		gone = append(gone, "album names on "+plural(rep.PlayerAlbums, "track", "tracks"))
	}
	if len(gone) > 0 {
		fmt.Fprintf(out, "\nNames chosen for the player that go (the shortened ones are worked out again): %s.\n", strings.Join(gone, ", "))
	}
	fmt.Fprintln(out)
}
