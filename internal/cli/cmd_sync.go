package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"music-manager/internal/app"
	"music-manager/internal/render"
)

func syncCmd(open opener) *cobra.Command {
	var opts app.SyncOptions
	var dryRun, yes bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Render the library folder from the records",
		Long: `Make the folder match the records: every track is written to its place
with a fresh tag, its album gets a cover file, and nothing else changes.

Files the records do not account for are only listed. --prune deletes them
(together with directories left empty); since that cannot be undone it
also needs --yes. --target renders into another directory, such as the mounted
player, instead of the library folder.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := open()
			if err != nil {
				return err
			}
			plan, err := a.PlanSync(opts)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(plan.Ops) == 0 {
				if !dryRun {
					if err := plan.Apply(); err != nil { // still collects unused blobs
						return err
					}
				}
				fmt.Fprintln(out, "nothing to do"+collected(plan.Collected))
				return nil
			}

			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			var counts [4]int
			for _, op := range plan.Ops {
				counts[op.Kind]++
				fmt.Fprintf(w, "%s\t%s\n", op.Kind, op.Path)
			}
			w.Flush()

			if dryRun {
				fmt.Fprintln(out, "dry run: nothing written")
				return nil
			}
			if counts[render.Remove] > 0 && !yes {
				return fmt.Errorf("refusing to delete %d files without --yes", counts[render.Remove])
			}
			if err := plan.Apply(); err != nil {
				return err
			}
			summary(out, counts, plan.Collected)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.Target, "target", "", "render into this directory instead of the library folder")
	f.BoolVar(&opts.Prune, "prune", false, "delete files the records do not account for")
	f.BoolVar(&dryRun, "dry-run", false, "show what would change without changing anything")
	f.BoolVar(&yes, "yes", false, "confirm deletions")
	return cmd
}

func collected(n int) string {
	switch n {
	case 0:
		return ""
	case 1:
		return "; collected 1 unused file"
	}
	return fmt.Sprintf("; collected %d unused files", n)
}

func summary(out interface{ Write([]byte) (int, error) }, counts [4]int, unused int) {
	var parts []string
	for _, k := range []struct {
		kind  render.Kind
		label string
	}{{render.Create, "created"}, {render.Update, "updated"}, {render.Remove, "removed"}} {
		if n := counts[k.kind]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, k.label))
		}
	}
	line := ""
	for i, p := range parts {
		if i > 0 {
			line += ", "
		}
		line += p
	}
	if n := counts[render.Stray]; n > 0 {
		if line != "" {
			line += "; "
		}
		line += fmt.Sprintf("%d stray files left alone (--prune removes them)", n)
	}
	line += collected(unused)
	fmt.Fprintln(out, line)
}
