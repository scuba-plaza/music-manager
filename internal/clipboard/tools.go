package clipboard

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Clipboard reads the clipboard of the desktop with a program that is installed.
type Clipboard struct{ src Source }

// Detect finds the program to read the clipboard with: wl-paste under Wayland,
// xclip under X11, or whichever of them there is. Without either, Image says so.
func Detect() *Clipboard {
	wl, _ := exec.LookPath("wl-paste")
	xc, _ := exec.LookPath("xclip")
	switch {
	case wl != "" && (os.Getenv("WAYLAND_DISPLAY") != "" || xc == ""):
		return &Clipboard{src: wlSource{wl}}
	case xc != "":
		return &Clipboard{src: xclipSource{xc}}
	}
	return &Clipboard{}
}

// Image returns the picture the clipboard holds, and a name for it (see Picture).
func (c *Clipboard) Image() ([]byte, string, error) {
	if c == nil || c.src == nil {
		return nil, "", ErrNoTool
	}
	return Picture(c.src)
}

// readTimeout is how long a program may take to answer: a program that owned
// the clipboard and has quit can leave a request waiting.
const readTimeout = 5 * time.Second

func run(bin string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
	defer cancel()
	return exec.CommandContext(ctx, bin, args...).Output()
}

// failure is why a program failed, in its own words if it said any.
func failure(err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if msg := strings.TrimSpace(string(exit.Stderr)); msg != "" {
			return errors.New(msg)
		}
	}
	return err
}

// emptyClipboard reports whether what a program said as it failed is that
// there was nothing on the clipboard to read.
func emptyClipboard(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "Nothing is copied") || strings.Contains(msg, "not available")
}

// types is what the program lists, one per line. An empty clipboard, which
// the programs report as a failure, lists none; any other failure is one.
func types(out []byte, err error) ([]string, error) {
	if err != nil {
		if err = failure(err); emptyClipboard(err) {
			return nil, nil
		}
		return nil, err
	}
	var list []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			list = append(list, line)
		}
	}
	return list, nil
}

// read is what the program wrote, or why it failed.
func read(out []byte, err error) ([]byte, error) {
	if err != nil {
		return nil, failure(err)
	}
	return out, nil
}

// wlSource is the clipboard as wl-paste (wl-clipboard) reads it.
type wlSource struct{ bin string }

func (s wlSource) Types() ([]string, error) { return types(run(s.bin, "--list-types")) }
func (s wlSource) Read(mime string) ([]byte, error) {
	return read(run(s.bin, "--no-newline", "--type", mime))
}

// xclipSource is the clipboard as xclip reads it.
type xclipSource struct{ bin string }

func (s xclipSource) Types() ([]string, error) {
	return types(run(s.bin, "-selection", "clipboard", "-t", "TARGETS", "-o"))
}
func (s xclipSource) Read(mime string) ([]byte, error) {
	return read(run(s.bin, "-selection", "clipboard", "-t", mime, "-o"))
}
