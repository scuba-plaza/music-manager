//go:build live

package ytdlp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"music-manager/internal/mp3"
	"music-manager/internal/testmp3"
	"music-manager/internal/ytdlp"
)

// Run with: go test -tags live ./internal/ytdlp
// It needs the real yt-dlp and ffmpeg. The audio comes from a throwaway local
// web server, which yt-dlp treats as a direct link to a media file.
func TestLiveDownloadOfADirectLink(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write(testmp3.Silent(400))
	}))
	defer srv.Close()

	dir := t.TempDir()
	got, err := ytdlp.New().Download(context.Background(), srv.URL+"/some-song.mp3", dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("info: %+v", got.Info)
	data, err := os.ReadFile(got.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, dur, err := mp3.Parse(data); err != nil || dur == 0 {
		t.Errorf("downloaded file is not usable audio: %v, %v", dur, err)
	}
}
