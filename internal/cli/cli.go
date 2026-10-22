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
}

// opener lazily opens the library, so commands that fail on bad flags never
// touch the disk.
type opener func() (*app.App, error)

// Run executes the CLI with args (without the program name) and returns the
// process exit code.
func Run(ctx context.Context, args []string, streams IO, deps Deps) int {
	var library string
	root := &cobra.Command{
		Use:           "mm",
		Short:         "Manage a music library for an MP3 player",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetIn(streams.In)
	root.SetOut(streams.Out)
	root.SetErr(streams.Err)
	root.SetArgs(args)
	root.PersistentFlags().StringVar(&library, "library", defaultLibrary(), "library directory (env MM_LIBRARY)")

	open := func() (*app.App, error) { return app.Open(library) }
	root.AddCommand(ingestCmd(open), lsCmd(open), showCmd(open), editCmd(open), syncCmd(open), addCmd(open, deps), rmCmd(open), doctorCmd(open), identifyCmd(open, deps))

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
