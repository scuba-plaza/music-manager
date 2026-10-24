// Package clipboard reads a picture from the clipboard of the desktop, to be
// used as a cover. A picture can be on the clipboard as itself (copied in a
// browser or an image viewer) or as a file (copied in a file manager); both are
// found, and only JPEG and PNG pictures, the ones a cover can be, are accepted.
// The clipboard is read by wl-paste or xclip, whichever is installed.
package clipboard

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"music-manager/internal/coverimg"
)

var (
	// ErrNoPicture says the clipboard holds nothing that is a picture.
	ErrNoPicture = errors.New("the clipboard holds no picture")
	// ErrUnsupported says it holds a picture that cannot be a cover.
	ErrUnsupported = errors.New("only a JPEG or PNG picture can be a cover")
	// ErrNoTool says there is no program to read the clipboard with.
	ErrNoTool = errors.New("reading the clipboard needs wl-paste (wl-clipboard) or xclip")
)

// maxPicture is the most that is read of a picture: a cover is never this big.
const maxPicture = 64 << 20

// Source is a clipboard that can be asked what it holds, by type, and for
// what it holds as each type.
type Source interface {
	// Types lists what the clipboard holds, as MIME types; none if it is empty.
	Types() ([]string, error)
	Read(mime string) ([]byte, error)
}

// Picture returns the picture that the clipboard holds, and the name of its
// file if it was copied as one (it has none if the picture is on the clipboard
// itself). A picture that is on the clipboard as such is taken before a file.
func Picture(src Source) (data []byte, name string, err error) {
	types, err := src.Types()
	if err != nil {
		return nil, "", fmt.Errorf("reading the clipboard: %w", err)
	}
	has := map[string]bool{}
	for _, t := range types {
		has[strings.ToLower(t)] = true
	}

	for _, mime := range []string{"image/png", "image/jpeg", "image/jpg"} {
		if !has[mime] {
			continue
		}
		data, err := src.Read(mime)
		if err != nil {
			return nil, "", fmt.Errorf("reading the clipboard: %w", err)
		}
		if coverimg.Sniff(data) == "" {
			return nil, "", fmt.Errorf("the picture on the clipboard is not a JPEG or PNG: %w", ErrUnsupported)
		}
		return data, "", nil
	}

	var candidates []candidate
	for _, mime := range []string{"text/uri-list", "x-special/gnome-copied-files"} {
		if has[mime] {
			if text, err := src.Read(mime); err == nil {
				candidates = append(candidates, addresses(string(text))...)
			}
		}
	}
	for _, mime := range []string{"text/plain;charset=utf-8", "text/plain", "utf8_string", "string"} {
		if has[mime] {
			if text, err := src.Read(mime); err == nil {
				candidates = append(candidates, paths(string(text))...)
			}
		}
	}
	var failure error
	for _, c := range candidates {
		data, err := os.ReadFile(c.path)
		switch {
		case err != nil && c.sure:
			failure = cmpErr(failure, fmt.Errorf("cannot read %s: %w", filepath.Base(c.path), unwrapPath(err)))
			continue
		case err != nil || len(data) > maxPicture:
			continue
		case coverimg.Sniff(data) != "":
			return data, filepath.Base(c.path), nil
		case otherPictures[strings.ToLower(filepath.Ext(c.path))]:
			failure = cmpErr(failure, fmt.Errorf("%s is not a JPEG or PNG picture: %w", filepath.Base(c.path), ErrUnsupported))
		}
	}
	if failure != nil {
		return nil, "", failure
	}
	for mime := range has {
		if strings.HasPrefix(mime, "image/") {
			return nil, "", fmt.Errorf("the picture on the clipboard is a %s: %w", strings.ToUpper(strings.TrimPrefix(mime, "image/")), ErrUnsupported)
		}
	}
	return nil, "", ErrNoPicture
}

// otherPictures are the extensions of pictures that are not JPEG or PNG.
var otherPictures = map[string]bool{
	".webp": true, ".gif": true, ".bmp": true, ".tif": true, ".tiff": true,
	".avif": true, ".heic": true, ".heif": true, ".jxl": true, ".ico": true,
}

// candidate is a file that may be the picture; sure says the clipboard named it
// as a file, so that it being gone is worth saying.
type candidate struct {
	path string
	sure bool
}

// addresses finds the files in a list of addresses (one per line, # for
// comments), or in what GNOME's files writes (copy or cut, then the addresses).
func addresses(text string) []candidate {
	var out []candidate
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "file://") {
			continue
		}
		rest := strings.TrimPrefix(line, "file://")
		rest = strings.TrimPrefix(rest, "localhost")
		if p, err := url.PathUnescape(rest); err == nil && filepath.IsAbs(p) {
			out = append(out, candidate{path: p, sure: true})
		}
	}
	return out
}

// paths finds the files that text names, one path to a line.
func paths(text string) []candidate {
	var out []candidate
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); filepath.IsAbs(line) {
			out = append(out, candidate{path: line})
		}
	}
	return out
}

// cmpErr keeps the first error.
func cmpErr(first, next error) error {
	if first != nil {
		return first
	}
	return next
}

// unwrapPath is the reason of a file error without the path, which is said
// by the name beside it.
func unwrapPath(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}
