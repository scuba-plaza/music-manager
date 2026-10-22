package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// tuiCmd is "mm tui": the full-screen interface, which needs a terminal.
func tuiCmd(open opener, deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the full-screen interface",
		Long:  "Open the full-screen interface for browsing and editing the library.\nThis is also what plain \"mm\" does when it runs in a terminal.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI(cmd, open, deps)
		},
	}
}

func runTUI(cmd *cobra.Command, open opener, deps Deps) error {
	if deps.RunTUI == nil {
		return errors.New("the interface needs a terminal; run mm in one, or use the commands (see mm --help)")
	}
	a, err := open()
	if err != nil {
		return err
	}
	return deps.RunTUI(cmd.Context(), a)
}
