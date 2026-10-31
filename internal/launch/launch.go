package launch

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"music-manager/internal/device"
	"music-manager/internal/version"
)

const defaultAddr = "127.0.0.1:7878"

type Options struct {
	Library     string
	Addr        string
	AddrSet     bool
	NoOpen      bool
	Config      string
	CheckConfig bool
	Profiles    *device.Registry
}

type Env struct {
	Args   []string
	Getenv func(string) string
	Home   string
	Out    io.Writer
	Err    io.Writer
	Serve  func(ctx context.Context, o Options) error
}

func DefaultLibrary(getenv func(string) string, home string) string {
	if library := getenv("MM_LIBRARY"); library != "" {
		return library
	}
	return filepath.Join(home, "Music", "Library")
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
	fs.StringVar(&o.Library, "library", DefaultLibrary(e.Getenv, e.Home), "library directory (env MM_LIBRARY)")
	fs.StringVar(&o.Addr, "addr", defaultAddr, "address to listen on (only this computer: 127.0.0.1, localhost or ::1)")
	fs.BoolVar(&o.NoOpen, "no-open", false, "do not open the page in a browser")
	fs.StringVar(&o.Config, "config", "", "device profiles file (default $MM_CONFIG, else $XDG_CONFIG_HOME/mm/devices.toml, else ~/.config/mm/devices.toml)")
	fs.BoolVar(&o.CheckConfig, "check-config", false, "check the device profiles file, list the profiles and exit")
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
	explicit := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "addr":
			o.AddrSet = true
		case "config":
			explicit = true
		}
	})
	if !explicit {
		o.Config = device.ConfigPath(e.Getenv, e.Home)
	}
	profiles, err := loadProfiles(o.Config, explicit)
	if err != nil {
		fmt.Fprintln(e.Err, "mm:", err)
		return 1
	}
	o.Profiles = profiles
	if o.CheckConfig {
		for _, p := range profiles.Profiles() {
			fmt.Fprintln(e.Out, p.Name)
		}
		return 0
	}
	if err := e.Serve(ctx, o); err != nil {
		fmt.Fprintln(e.Err, "mm:", err)
		return 1
	}
	return 0
}

func loadProfiles(path string, explicit bool) (*device.Registry, error) {
	registry, err := device.LoadRegistry(path)
	if errors.Is(err, fs.ErrNotExist) && !explicit {
		return device.Builtin(), nil
	}
	return registry, err
}
