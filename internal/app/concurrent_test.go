package app_test

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"music-manager/internal/app"
	"music-manager/internal/manifest"
	"music-manager/internal/testmp3"
)

func TestSeveralMmChangingTheLibraryAtOnceLoseNoChange(t *testing.T) {
	a := openLib(t)
	src := t.TempDir()
	for i := 0; i < 60; i++ { // enough records that reading and writing them takes a while
		writeFile(t, filepath.Join(src, fmt.Sprintf("%02d.mp3", i)), testmp3.Silent(10+i))
	}
	if _, err := a.Ingest([]string{src}); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, 60)
	for _, tr := range tracks(t, a) {
		ids = append(ids, tr.ID)
	}
	b, err := app.Open(a.Root()) // the library is open in two places
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		who := a
		if i%2 == 1 {
			who = b
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each of them changes something of its own: a playlist, and a track.
			if err := who.CreatePlaylist(fmt.Sprintf("list %02d", i), ids[i]); err != nil {
				t.Errorf("playlist %d: %v", i, err)
			}
			if _, err := who.Edit(ids[i], app.Edit{Genre: ptr(fmt.Sprintf("genre %02d", i))}); err != nil {
				t.Errorf("edit %d: %v", i, err)
			}
		}()
	}
	wg.Wait()

	m, err := a.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Playlists) != 24 {
		t.Errorf("%d playlists, want the 24 that were made", len(m.Playlists))
	}
	for i := 0; i < 24; i++ {
		if tr, _ := m.Get(ids[i]); tr.Genre != fmt.Sprintf("genre %02d", i) {
			t.Errorf("track %d has the genre %q: its edit was lost", i, tr.Genre)
		}
	}
	if len(m.Tracks) != 60 {
		t.Errorf("%d tracks, want 60", len(m.Tracks))
	}
}

func TestASyncCollectsNoFilesWhileTheRecordsAreBeingChanged(t *testing.T) {
	a := libWithTracks(t, 40)
	plan := mustPlan(t, a, app.SyncOptions{})

	other, err := app.Open(a.Root()) // another mm is in the middle of a change
	if err != nil {
		t.Fatal(err)
	}
	inside, release := make(chan struct{}), make(chan struct{})
	changing := make(chan error, 1)
	go func() {
		changing <- other.Records().Update(func(*manifest.Manifest) error {
			close(inside)
			<-release
			return nil
		})
	}()
	<-inside

	applied := make(chan error, 1)
	go func() { applied <- plan.Apply() }()
	select {
	case err := <-applied:
		t.Fatalf("the sync collected files while the records were being changed (err %v)", err)
	case <-time.After(200 * time.Millisecond):
	}

	close(release)
	if err := <-changing; err != nil {
		t.Fatal(err)
	}
	if err := <-applied; err != nil {
		t.Fatal(err)
	}
}
