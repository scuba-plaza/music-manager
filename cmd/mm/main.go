package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"music-manager/internal/app"
	"music-manager/internal/launch"
	"music-manager/internal/loudness"
	"music-manager/internal/musicbrainz"
	"music-manager/internal/version"
	"music-manager/internal/web"
	"music-manager/internal/ytdlp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	home, _ := os.UserHomeDir()
	os.Exit(launch.Run(ctx, launch.Env{
		Args:   os.Args[1:],
		Getenv: os.Getenv,
		Home:   home,
		Out:    os.Stdout,
		Err:    os.Stderr,
		Serve:  serve,
	}))
}

func serve(ctx context.Context, o launch.Options) error {
	lookup := musicbrainz.New(userAgent())
	if url := os.Getenv("MM_MUSICBRAINZ_URL"); url != "" {
		lookup.BaseURL = strings.TrimRight(url, "/")
	}
	if url := os.Getenv("MM_COVERART_URL"); url != "" {
		lookup.CoverURL = strings.TrimRight(url, "/")
	}
	a, err := app.Open(o.Library)
	if err != nil {
		return err
	}
	a.UseLoudness(loudness.NewFFmpeg())
	ln, err := web.Listen(o.Addr, !o.AddrSet)
	if err != nil {
		return err
	}
	url := web.URL(ln)
	fmt.Fprintf(os.Stdout, "mm is at %s  (Ctrl+C stops it)\n", url)
	if !o.NoOpen {
		if err := web.OpenBrowser(url); err != nil {
			fmt.Fprintln(os.Stdout, "mm: cannot open a browser:", err)
		}
	}
	server := web.New(a, web.WithSource(lookup), web.WithDownloader(ytdlp.New()), web.WithWolfgang())
	return server.Serve(ctx, ln)
}

func userAgent() string {
	ua := "music-manager/" + version.String()
	if contact := os.Getenv("MM_CONTACT"); contact != "" {
		ua += " ( " + contact + " )"
	}
	return ua
}
