package launch

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"music-manager/internal/version"
)

const defaultAddr = "127.0.0.1:7878"

type Options struct {
	Library string
	Addr    string
	AddrSet bool
	NoOpen  bool
	Out     io.Writer
}

type Env struct {
	Args   []string
	Getenv func(string) string
	Home   string
	Out    io.Writer
	Err    io.Writer
	Serve  func(ctx context.Context, o Options) error
}

func Run(ctx context.Context, e Env) int {
	var o Options
	var showVersion bool
	fs := flag.NewFlagSet("mm", flag.ContinueOnError)
	fs.SetOutput(e.Err)
	fs.Usage = func() {
		fmt.Fprintln(e.Err, "usage: mm [flags]")
		fs.PrintDefaults()
	}
	fs.StringVar(&o.Library, "library", "", "library directory (default $MM_LIBRARY, else ~/Music/Library)")
	fs.StringVar(&o.Addr, "addr", defaultAddr, "address to listen on (only this computer: 127.0.0.1, localhost or ::1)")
	fs.BoolVar(&o.NoOpen, "no-open", false, "do not open the page in a browser")
	fs.BoolVar(&showVersion, "version", false, "print the version and exit")

	if err := fs.Parse(e.Args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(e.Err, "mm: unexpected argument %q\n", fs.Arg(0))
		fs.Usage()
		return 2
	}
	if showVersion {
		fmt.Fprintf(e.Out, "mm %s\n", version.String())
		return 0
	}
	if o.Library == "" {
		o.Library = e.Getenv("MM_LIBRARY")
	}
	if o.Library == "" {
		o.Library = filepath.Join(e.Home, "Music", "Library")
	}
	o.Out = e.Out
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "addr" {
			o.AddrSet = true
		}
	})
	if err := e.Serve(ctx, o); err != nil {
		fmt.Fprintln(e.Err, "mm:", err)
		return 1
	}
	return 0
}
