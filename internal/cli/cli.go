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
	"music-manager/internal/loudness"
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
	// Loudness measures tracks so that sync can write their ReplayGain tags. Nil
	// means it writes none.
	Loudness loudness.Analyzer
	// RunWeb serves the web interface on a library until ctx is done.
	RunWeb func(ctx context.Context, a *app.App, o WebOptions) error
}

// opener lazily opens the library, so commands that fail on bad flags never
// touch the disk.
type opener func() (*app.App, error)

// Run executes the CLI with args (without the program name) and returns the
// process exit code.
func Run(ctx context.Context, args []string, streams IO, deps Deps) int {
	var library string
	var root *cobra.Command
	open := func() (*app.App, error) {
		if !root.PersistentFlags().Changed("library") && os.Getenv("MM_LIBRARY") == "" {
			if err := checkNotMoved(); err != nil {
				return nil, err
			}
		}
		a, err := app.Open(library)
		if err == nil && deps.Loudness != nil {
			a.UseLoudness(deps.Loudness)
		}
		return a, err
	}
	root = &cobra.Command{
		Use:           "mm",
		Short:         "Manage a music library for an MP3 player",
		Long:          "Manage a music library for an MP3 player: one record per track, rendered into a clean folder by sync.\n\nRun mm web to browse and edit the library in a browser.",
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("mm {{.Version}}\n")
	root.SetIn(streams.In)
	root.SetOut(streams.Out)
	root.SetErr(streams.Err)
	root.SetArgs(args)
	root.PersistentFlags().StringVar(&library, "library", defaultLibrary(), "library directory (env MM_LIBRARY)")

	root.AddCommand(ingestCmd(open), lsCmd(open), showCmd(open), editCmd(open), syncCmd(open), addCmd(open, deps), rmCmd(open), doctorCmd(open), identifyCmd(open, deps), adoptCmd(open), playlistCmd(open), reviewCmd(open), webCmd(open, deps))

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
	return filepath.Join(home, "Music", "Library")
}

// checkNotMoved stops mm from making a new library beside an old one. The default
// folder used to be ~/Music/mp3-player, from when it only fed a player; it is
// ~/Music/Library now that a desktop player plays it. mm makes the folder of a
// library on any command, and an empty Library made before the move would take
// the old folder in as a subfolder of itself when it was moved.
func checkNotMoved() error {
	home, _ := os.UserHomeDir()
	old := filepath.Join(home, "Music", "mp3-player")
	now := filepath.Join(home, "Music", "Library")
	if _, err := os.Stat(filepath.Join(old, "library.json")); err != nil {
		return nil
	}
	if _, err := os.Stat(now); err == nil {
		return nil
	}
	return fmt.Errorf("the library folder is now %s, but a library is still in %s: move it with `mv %s %s`, or name the folder with --library or MM_LIBRARY", now, old, old, now)
}
