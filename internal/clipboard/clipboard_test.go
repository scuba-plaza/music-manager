package clipboard

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"music-manager/internal/testmp3"
)

// fakeSource is a clipboard that holds what it is given, by type.
type fakeSource struct {
	data map[string][]byte
	err  error
}

func (f fakeSource) Types() ([]string, error) {
	var types []string
	for t := range f.data {
		types = append(types, t)
	}
	return types, f.err
}

func (f fakeSource) Read(mime string) ([]byte, error) {
	d, ok := f.data[mime]
	if !ok {
		return nil, errors.New("no " + mime)
	}
	return d, nil
}

func file(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAPictureOnTheClipboardIsReturnedAsItIs(t *testing.T) {
	for name, want := range map[string][]byte{"image/png": testmp3.PNG, "image/jpeg": testmp3.JPEG} {
		got, from, err := Picture(fakeSource{data: map[string][]byte{name: want}})
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s: err %v, %d bytes, want %d", name, err, len(got), len(want))
		}
		if from != "" {
			t.Errorf("%s: it is called %q, but it has no file", name, from)
		}
	}
}

func TestAFileCopiedInAFileManagerIsReadFromWhereItIs(t *testing.T) {
	png := file(t, "my cover.png", testmp3.PNG) // a space, which the list of files writes as %20
	jpg := file(t, "cover.jpg", testmp3.JPEG)
	encoded := strings.ReplaceAll("file://"+png, " ", "%20")

	for name, src := range map[string]fakeSource{
		"a list of addresses":      {data: map[string][]byte{"text/uri-list": []byte("# a comment\r\n" + encoded + "\r\n")}},
		"what the GNOME files do":  {data: map[string][]byte{"x-special/gnome-copied-files": []byte("copy\n" + encoded)}},
		"a path written as text":   {data: map[string][]byte{"text/plain;charset=utf-8": []byte(png + "\n")}},
		"a plain-text type, bare":  {data: map[string][]byte{"text/plain": []byte(png)}},
		"the first of two files":   {data: map[string][]byte{"text/uri-list": []byte(encoded + "\nfile://" + jpg + "\n")}},
		"after a file that is not": {data: map[string][]byte{"text/uri-list": []byte("file://" + file(t, "notes.txt", []byte("hello")) + "\n" + encoded)}},
	} {
		got, from, err := Picture(src)
		if err != nil || !bytes.Equal(got, testmp3.PNG) {
			t.Errorf("%s: err %v, %d bytes", name, err, len(got))
		}
		if from != "my cover.png" {
			t.Errorf("%s: it is called %q", name, from)
		}
	}

	got, from, err := Picture(fakeSource{data: map[string][]byte{"text/uri-list": []byte("file://" + jpg)}})
	if err != nil || !bytes.Equal(got, testmp3.JPEG) || from != "cover.jpg" {
		t.Errorf("a JPEG file: err %v, name %q", err, from)
	}
}

func TestThePictureIsToldByItsBytesNotItsName(t *testing.T) {
	p := file(t, "really-a-png.jpg", testmp3.PNG)
	got, _, err := Picture(fakeSource{data: map[string][]byte{"text/uri-list": []byte("file://" + p)}})
	if err != nil || !bytes.Equal(got, testmp3.PNG) {
		t.Errorf("err %v, %d bytes", err, len(got))
	}
}

func TestNothingToPasteIsToldFromAPictureThatCannotBeUsed(t *testing.T) {
	text := fakeSource{data: map[string][]byte{"text/plain": []byte("just some words")}}
	if _, _, err := Picture(text); !errors.Is(err, ErrNoPicture) {
		t.Errorf("text: err %v, want ErrNoPicture", err)
	}
	if _, _, err := Picture(fakeSource{}); !errors.Is(err, ErrNoPicture) {
		t.Errorf("an empty clipboard: err %v, want ErrNoPicture", err)
	}
	notes := file(t, "notes.txt", []byte("hello"))
	if _, _, err := Picture(fakeSource{data: map[string][]byte{"text/uri-list": []byte("file://" + notes)}}); !errors.Is(err, ErrNoPicture) {
		t.Errorf("a file that is not a picture: err %v, want ErrNoPicture", err)
	}

	// A picture in another format is something the library cannot keep: say so.
	webp := file(t, "cover.webp", append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 20)...))
	_, _, err := Picture(fakeSource{data: map[string][]byte{"text/uri-list": []byte("file://" + webp)}})
	if !errors.Is(err, ErrUnsupported) || !strings.Contains(err.Error(), "cover.webp") || !strings.Contains(err.Error(), "JPEG or PNG") {
		t.Errorf("a WebP file: err %v", err)
	}
	_, _, err = Picture(fakeSource{data: map[string][]byte{"image/webp": []byte("RIFF....WEBP")}})
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("WebP data: err %v", err)
	}
	_, _, err = Picture(fakeSource{data: map[string][]byte{"image/png": []byte("this is not a picture")}})
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("bytes that are not the picture they claim: err %v", err)
	}
	_, _, err = Picture(fakeSource{data: map[string][]byte{"text/uri-list": []byte("file:///nowhere/at/all/cover.png")}})
	if err == nil || errors.Is(err, ErrNoPicture) {
		t.Errorf("a file that is gone: err %v, want its own error", err)
	}
}

func TestAPictureWinsOverTheFileOfTheSameCopy(t *testing.T) {
	p := file(t, "cover.jpg", testmp3.JPEG)
	got, _, err := Picture(fakeSource{data: map[string][]byte{
		"image/png":     testmp3.PNG,
		"text/uri-list": []byte("file://" + p),
	}})
	if err != nil || !bytes.Equal(got, testmp3.PNG) {
		t.Errorf("err %v, %d bytes", err, len(got))
	}
}

// ---- the programs that read the clipboard

// fakeTools puts programs called wl-paste and xclip (the ones named) in a
// directory that is alone on the PATH. They answer from the files of a folder:
// one per type, with the / of the type written as %. It returns that folder.
func fakeTools(t *testing.T, tools ...string) string {
	t.Helper()
	bin, clip := t.TempDir(), t.TempDir()
	script := `#!/bin/sh
list() { for f in "$FAKE_CLIP"/*; do [ -e "$f" ] && basename "$f" | tr % /; done; }
mime=""
while [ $# -gt 0 ]; do
  case "$1" in
    --list-types) list; [ -n "$(list)" ] || { echo "Nothing is copied" >&2; exit 1; }; exit 0 ;;
    --type|-t) mime="$2"; shift ;;
  esac
  shift
done
if [ "$mime" = TARGETS ]; then list; exit 0; fi
cat "$FAKE_CLIP/$(echo "$mime" | tr / %)"
`
	for _, name := range []string{"cat", "basename", "tr"} { // what the script needs, on a PATH of its own
		p, err := exec.LookPath(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(p, filepath.Join(bin, name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, tool := range tools {
		if err := os.WriteFile(filepath.Join(bin, tool), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("FAKE_CLIP", clip)
	return clip
}

func put(t *testing.T, clip, mime string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(clip, strings.ReplaceAll(mime, "/", "%")), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAPictureIsReadWithWlPasteOnWayland(t *testing.T) {
	clip := fakeTools(t, "wl-paste")
	wl, _ := exec.LookPath("wl-paste")
	broken := filepath.Join(filepath.Dir(wl), "xclip") // present, but the one to use is wl-paste
	if err := os.WriteFile(broken, []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", ":0")
	put(t, clip, "image/png", testmp3.PNG)
	put(t, clip, "text/html", []byte("<img>"))

	got, _, err := Detect().Image()
	if err != nil || !bytes.Equal(got, testmp3.PNG) {
		t.Errorf("err %v, %d bytes", err, len(got))
	}
}

func TestAFileCopiedInAFileManagerIsFoundWithXclipOnX11(t *testing.T) {
	clip := fakeTools(t, "xclip")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", ":0")
	p := file(t, "cover.jpg", testmp3.JPEG)
	put(t, clip, "text/uri-list", []byte("file://"+p+"\r\n"))

	got, from, err := Detect().Image()
	if err != nil || !bytes.Equal(got, testmp3.JPEG) || from != "cover.jpg" {
		t.Errorf("err %v, %d bytes, called %q", err, len(got), from)
	}
}

func TestAnEmptyClipboardHoldsNoPicture(t *testing.T) {
	fakeTools(t, "wl-paste")
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	if _, _, err := Detect().Image(); !errors.Is(err, ErrNoPicture) {
		t.Errorf("err %v, want ErrNoPicture", err)
	}
}

func TestWithoutAProgramToReadItTheClipboardSaysWhatIsNeeded(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, _, err := Detect().Image()
	if !errors.Is(err, ErrNoTool) || !strings.Contains(err.Error(), "wl-paste") || !strings.Contains(err.Error(), "xclip") {
		t.Errorf("err %v", err)
	}
}

func TestAProgramThatFailsIsNotTakenForAnEmptyClipboard(t *testing.T) {
	bin := t.TempDir()
	broken := "#!/bin/sh\necho 'Failed to connect to a Wayland server' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "wl-paste"), []byte(broken), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	_, _, err := Detect().Image()
	if err == nil || errors.Is(err, ErrNoPicture) || !strings.Contains(err.Error(), "Failed to connect to a Wayland server") {
		t.Errorf("err %v: the failure was not reported", err)
	}
}
