// Package cli is the command-line front end of the music manager.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"music-manager/internal/app"
	"music-manager/internal/version"
)

// IO bundles the standard streams so tests can substitute them.
type IO struct {
	In       io.Reader
	Out, Err io.Writer
}

// Deps are the collaborators that reach outside the process. A nil
// Prompter means the session is not interactive.
type Deps struct {
	Downloader app.Downloader
	Metadata   app.MetadataSource
	Prompter   app.Prompter
	// RunTUI runs the full-screen interface on a library. It is set only
	// when there is a terminal for it; without it plain "mm" prints help.
	// wolfgang asks for the preview of what a sync for the Wolfgang WMP01 writes.
	RunTUI func(ctx context.Context, a *app.App, wolfgang bool) error
}

// opener lazily opens the library, so commands that fail on bad flags never
// touch the disk.
type opener func() (*app.App, error)

// Run executes the CLI with args (without the program name) and returns the
// process exit code.
func Run(ctx context.Context, args []string, streams IO, deps Deps) int {
	var library string
	var wolfgang bool
	open := func() (*app.App, error) { return app.Open(library) }
	root := &cobra.Command{
		Use:           "mm",
		Short:         "Manage a music library for an MP3 player",
		Long:          "Manage a music library for an MP3 player: one record per track, rendered into a clean folder by sync.\n\nRun mm without a command, in a terminal, to open the full-screen interface.",
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: true,
		// With no subcommand, open the interface if there is a terminal for it.
		RunE: func(cmd *cobra.Command, args []string) error {
			if deps.RunTUI == nil {
				return cmd.Help()
			}
			return runTUI(cmd, open, deps, wolfgang)
		},
	}
	root.SetVersionTemplate("mm {{.Version}}\n")
	root.SetIn(streams.In)
	root.SetOut(streams.Out)
	root.SetErr(streams.Err)
	root.SetArgs(args)
	root.PersistentFlags().StringVar(&library, "library", defaultLibrary(), "library directory (env MM_LIBRARY)")
	wolfgangFlag(root, &wolfgang)

	root.AddCommand(ingestCmd(open), lsCmd(open), showCmd(open), editCmd(open), syncCmd(open), addCmd(open, deps), rmCmd(open), doctorCmd(open), identifyCmd(open, deps), adoptCmd(open), playlistCmd(open), reviewCmd(open), migrateCmd(open), tuiCmd(open, deps))

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(streams.Err, "mm:", err)
		return 1
	}
	return 0
}

func defaultLibrary() string {
	if env := os.Getenv("MM_LIBRARY"); env != "" {
		return env
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Music", "mp3-player")
}
