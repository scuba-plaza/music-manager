package web

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"music-manager/internal/app"
)

//go:embed testdata/playlist_picker_probe.js
var playlistPickerProbe []byte

type probeBox struct{ X, Y, W, H float64 }

type probeCell struct {
	probeBox
	Class string
	Text  string
	Lines int
}

type probeItem struct {
	Selected bool
	Cells    []probeCell
}

type pickerLayout struct {
	Error    string
	Viewport struct{ W, H float64 }
	Title    string
	Dialog   probeBox
	Columns  int
	Items    []probeItem
}

func findBrowser(t *testing.T) string {
	t.Helper()
	if path := os.Getenv("MM_BROWSER"); path != "" {
		return path
	}
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "chrome"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	t.Skip("no Chromium or Chrome found: set MM_BROWSER to its path to check how the dialogs look")
	return ""
}

func withProbe(t *testing.T, next http.Handler, probe []byte, reports chan<- []byte) http.Handler {
	t.Helper()
	index, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		t.Fatal(err)
	}
	page := bytes.Replace(index, []byte("</body>"), []byte(`<script type="module" src="/probe.js"></script></body>`), 1)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(page)
		case "/probe.js":
			w.Header().Set("Content-Type", "text/javascript")
			w.Write(probe)
		case "/probe-result":
			body, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
			select {
			case reports <- body:
			default:
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func tail(b []byte, n int) string {
	if len(b) > n {
		b = b[len(b)-n:]
	}
	return string(b)
}

func measurePicker(t *testing.T, browser string, a *app.App, width, height int) pickerLayout {
	t.Helper()
	reports := make(chan []byte, 1)
	srv := httptest.NewServer(withProbe(t, New(a).Handler(), playlistPickerProbe, reports))
	defer srv.Close()
	profile, err := os.MkdirTemp("", "mm-browser-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(profile)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, browser,
		"--headless", "--no-sandbox", "--disable-gpu", "--hide-scrollbars", "--no-first-run",
		"--user-data-dir="+profile,
		fmt.Sprintf("--window-size=%d,%d", width, height),
		srv.URL+"/")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.WaitDelay = 3 * time.Second
	if err := cmd.Start(); err != nil {
		t.Fatalf("%s: %v", browser, err)
	}
	exited := make(chan struct{})
	var waitErr error
	go func() {
		waitErr = cmd.Wait()
		close(exited)
	}()
	defer func() {
		cancel()
		<-exited
	}()
	select {
	case report := <-reports:
		var got pickerLayout
		if err := json.Unmarshal(report, &got); err != nil {
			t.Fatalf("the layout report is not JSON: %v\n%s", err, report)
		}
		return got
	case <-exited:
		t.Fatalf("the browser quit before the page reported a layout: %v\n%s", waitErr, tail(stderr.Bytes(), 2000))
	case <-ctx.Done():
		cancel()
		<-exited
		t.Fatalf("the page did not report a layout in time\n%s", tail(stderr.Bytes(), 2000))
	}
	return pickerLayout{}
}

func cellOf(item probeItem, class string) *probeCell {
	for i := range item.Cells {
		if item.Cells[i].Class == class {
			return &item.Cells[i]
		}
	}
	return nil
}

func near(a, b float64) bool { return math.Abs(a-b) <= 1 }

func checkPickerLayout(t *testing.T, got pickerLayout, names, counts []string) {
	t.Helper()
	if got.Error != "" {
		t.Fatalf("the page could not be measured: %s", got.Error)
	}
	if got.Title != "Add to a playlist" {
		t.Errorf("the dialog is titled %q", got.Title)
	}
	d := got.Dialog
	if d.X < 0 || d.Y < 0 || d.X+d.W > got.Viewport.W+1 || d.Y+d.H > got.Viewport.H+1 {
		t.Errorf("the dialog at %+v does not fit the %vx%v window", d, got.Viewport.W, got.Viewport.H)
	}
	if len(got.Items) != len(names) {
		t.Fatalf("the dialog lists %d entries, want %d", len(got.Items), len(names))
	}
	first := got.Items[0]
	for i, item := range got.Items {
		row := fmt.Sprintf("row %d (%s)", i+1, names[i])
		if item.Selected != (i == 0) {
			t.Errorf("%s: selected is %v", row, item.Selected)
		}
		if len(item.Cells) != got.Columns {
			t.Errorf("%s has %d cells for a grid of %d columns, so every cell after it lands in the wrong column", row, len(item.Cells), got.Columns)
		}
		if len(item.Cells) == 0 {
			continue
		}
		for c, cell := range item.Cells {
			if !near(cell.Y, item.Cells[0].Y) || !near(cell.H, item.Cells[0].H) {
				t.Errorf("%s: cell %d (%q) is at y=%.0f h=%.0f, but the row starts at y=%.0f h=%.0f", row, c+1, cell.Text, cell.Y, cell.H, item.Cells[0].Y, item.Cells[0].H)
			}
			if c < len(first.Cells) && (!near(cell.X, first.Cells[c].X) || !near(cell.W, first.Cells[c].W)) {
				t.Errorf("%s: column %d is at x=%.0f w=%.0f, but in the first row it is at x=%.0f w=%.0f", row, c+1, cell.X, cell.W, first.Cells[c].X, first.Cells[c].W)
			}
			if cell.X < d.X-1 || cell.X+cell.W > d.X+d.W+1 {
				t.Errorf("%s: cell %d (%q) at x=%.0f w=%.0f sticks out of the dialog (x=%.0f w=%.0f)", row, c+1, cell.Text, cell.X, cell.W, d.X, d.W)
			}
			if c > 0 {
				before := item.Cells[c-1]
				if before.X+before.W > cell.X+1 {
					t.Errorf("%s: cell %d (%q) overlaps the one before it", row, c+1, cell.Text)
				}
			}
		}
		if i > 0 {
			before := got.Items[i-1]
			if len(before.Cells) > 0 && item.Cells[0].Y < before.Cells[0].Y+before.Cells[0].H-1 {
				t.Errorf("%s starts at y=%.0f, inside the row above, which ends at y=%.0f", row, item.Cells[0].Y, before.Cells[0].Y+before.Cells[0].H)
			}
		}
		label, count := cellOf(item, "t"), cellOf(item, "d")
		if label == nil || count == nil {
			t.Errorf("%s has no name or no count cell", row)
			continue
		}
		if label.Text != names[i] {
			t.Errorf("%s: the name reads %q", row, label.Text)
		}
		if count.Text != counts[i] {
			t.Errorf("%s: the count reads %q, want %q", row, count.Text, counts[i])
		}
		if label.W < d.W/2 {
			t.Errorf("%s: the name has %.0fpx of a %.0fpx dialog", row, label.W, d.W)
		}
		if count.X < label.X+label.W-1 {
			t.Errorf("%s: the count (x=%.0f) is not to the right of the name (ends at x=%.0f)", row, count.X, label.X+label.W)
		}
		if i != 1 && label.Lines != 1 {
			t.Errorf("%s: the name is broken over %d lines", row, label.Lines)
		}
		if count.Lines > 1 {
			t.Errorf("%s: the count is broken over %d lines", row, count.Lines)
		}
	}
}

func TestPlaylistPickerLooksRight(t *testing.T) {
	browser := findBrowser(t)
	long := "A rather long playlist name that goes on and on and on for a while"
	a, tracks := seed(t,
		seedTrack{"Blue in Green", "Miles Davis", "Kind of Blue", 3},
		seedTrack{"So What", "Miles Davis", "Kind of Blue", 1},
		seedTrack{"Giant Steps", "John Coltrane", "Giant Steps", 1},
	)
	if err := a.CreatePlaylist("Late night", tracks[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := a.CreatePlaylist(long, tracks[1].ID, tracks[2].ID); err != nil {
		t.Fatal(err)
	}

	names := []string{"Late night", long, "New playlist…"}
	counts := []string{"1", "2", ""}
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1200, 800}, {"narrow", 500, 700}} {
		t.Run(size.name, func(t *testing.T) {
			checkPickerLayout(t, measurePicker(t, browser, a, size.width, size.height), names, counts)
		})
	}
}
