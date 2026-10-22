package main

import (
	"context"
	"os"
	"os/signal"

	"golang.org/x/term"

	"music-manager/internal/app"
	"music-manager/internal/cli"
	"music-manager/internal/musicbrainz"
	"music-manager/internal/prompt"
	"music-manager/internal/tui"
	"music-manager/internal/version"
	"music-manager/internal/ytdlp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	deps := cli.Deps{Downloader: ytdlp.New(), Metadata: musicbrainz.New(userAgent())}
	if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
		deps.Prompter = prompt.New(os.Stdin, os.Stderr)
		deps.RunTUI = func(ctx context.Context, a *app.App) error {
			return tui.Run(ctx, tui.NewLibrary(a), tui.WithPlayer(tui.DetectPlayer()))
		}
	}
	os.Exit(cli.Run(ctx, os.Args[1:], cli.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}, deps))
}

// userAgent identifies this program to MusicBrainz. MusicBrainz asks for a
// contact address; set MM_CONTACT to an email or URL to include one.
func userAgent() string {
	ua := "music-manager/" + version.String()
	if contact := os.Getenv("MM_CONTACT"); contact != "" {
		ua += " ( " + contact + " )"
	}
	return ua
}
