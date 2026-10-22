package blobstore_test

import (
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	"music-manager/internal/blobstore"
)

// sha256 of "hello", an independent known-good value.
const helloSHA = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

func TestPutStoresContentUnderItsHash(t *testing.T) {
	s := blobstore.New(t.TempDir(), ".mp3")
	sha, err := s.Put(strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if sha != helloSHA {
		t.Errorf("sha = %s, want %s", sha, helloSHA)
	}
	if !s.Has(sha) || s.Has("0000") {
		t.Error("Has should be true for the stored blob only")
	}
	r, err := s.Open(sha)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if got, _ := io.ReadAll(r); string(got) != "hello" {
		t.Errorf("content = %q", got)
	}
}

func TestPutSameContentTwiceStoresOneBlob(t *testing.T) {
	dir := t.TempDir()
	s := blobstore.New(dir, ".mp3")
	for i := 0; i < 2; i++ {
		if _, err := s.Put(strings.NewReader("hello")); err != nil {
			t.Fatal(err)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("%d files in store, want 1", len(entries))
	}
}

type failingReader struct{ n int }

func (f *failingReader) Read(p []byte) (int, error) {
	if f.n == 0 {
		return 0, errors.New("disk on fire")
	}
	f.n--
	return copy(p, "x"), nil
}

func TestFailedPutLeavesNothingBehind(t *testing.T) {
	dir := t.TempDir()
	s := blobstore.New(dir, ".mp3")
	if _, err := s.Put(&failingReader{n: 5}); err == nil {
		t.Fatal("Put succeeded, want error")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("%d files left behind", len(entries))
	}
}

func TestGCRemovesExactlyTheUnreferencedBlobs(t *testing.T) {
	s := blobstore.New(t.TempDir(), ".mp3")
	var shas []string
	for _, c := range []string{"one", "two", "three"} {
		sha, err := s.Put(strings.NewReader(c))
		if err != nil {
			t.Fatal(err)
		}
		shas = append(shas, sha)
	}
	removed, err := s.GC(map[string]bool{shas[0]: true, shas[2]: true, "not-in-store": true})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(removed, []string{shas[1]}) {
		t.Errorf("removed = %v, want [%s]", removed, shas[1])
	}
	if !s.Has(shas[0]) || s.Has(shas[1]) || !s.Has(shas[2]) {
		t.Error("store does not hold exactly the kept blobs")
	}
}
