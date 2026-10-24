package web

import (
	"os/exec"
	"runtime"
)

// OpenBrowser asks the desktop to show url in the person's browser. It does
// not wait for it, and says if there is no way to ask.
func OpenBrowser(url string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	cmd := exec.Command(name, url)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
