package cli

import (
	"errors"
	"io"

	"github.com/spf13/cobra"
)

// WebOptions say how to serve the web interface.
type WebOptions struct {
	// Addr is the address to listen on; AddrSet says it was asked for, in
	// which case it is used as it is, and not replaced by a free port if taken.
	Addr    string
	AddrSet bool
	// NoOpen says not to open the page in a browser.
	NoOpen bool
	// Wolfgang asks for the preview of what a sync for the Wolfgang WMP01 writes.
	Wolfgang bool
	// Out is where to say the address.
	Out io.Writer
}

// DefaultWebAddr is where the web interface is served, so that it can be bookmarked.
const DefaultWebAddr = "127.0.0.1:7878"

// webCmd is "mm web": the interface in a browser.
func webCmd(open opener, deps Deps) *cobra.Command {
	o := WebOptions{}
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Open the interface in a browser",
		Long: "Serve the interface on this computer and open it in a browser.\n" +
			"It listens on 127.0.0.1 only, and stops with Ctrl+C. Several mm can run at once; they\n" +
			"take turns changing the library.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if deps.RunWeb == nil {
				return errors.New("the web interface is not available")
			}
			a, err := open()
			if err != nil {
				return err
			}
			o.AddrSet = cmd.Flags().Changed("addr")
			o.Out = cmd.OutOrStdout()
			return deps.RunWeb(cmd.Context(), a, o)
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.Addr, "addr", DefaultWebAddr, "address to listen on (only this computer: 127.0.0.1, localhost or ::1)")
	f.BoolVar(&o.NoOpen, "no-open", false, "do not open the page in a browser")
	f.BoolVar(&o.Wolfgang, "wolfgang", false, "also show what a sync for the Wolfgang WMP01 will write")
	return cmd
}
