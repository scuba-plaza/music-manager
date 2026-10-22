// Package blobstore keeps files addressed by the SHA-256 of their content.
package blobstore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Store struct{ dir, ext string }

// New returns a store keeping blobs as <dir>/<sha><ext>.
func New(dir, ext string) *Store { return &Store{dir: dir, ext: ext} }

func (s *Store) path(sha string) string { return filepath.Join(s.dir, sha+s.ext) }

// Put stores r's content and returns its hex SHA-256. Storing content that
// is already present is a no-op. A failed Put leaves nothing behind.
func (s *Store) Put(r io.Reader) (string, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(s.dir, ".put-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), r); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	sha := hex.EncodeToString(h.Sum(nil))
	if s.Has(sha) {
		return sha, nil
	}
	return sha, os.Rename(tmp.Name(), s.path(sha))
}

func (s *Store) Has(sha string) bool {
	_, err := os.Stat(s.path(sha))
	return err == nil
}

func (s *Store) Open(sha string) (*os.File, error) { return os.Open(s.path(sha)) }

// Path returns where the blob lives, for callers that copy it elsewhere.
func (s *Store) Path(sha string) string { return s.path(sha) }

// GC deletes every blob whose SHA is not in keep and returns the deleted SHAs.
func (s *Store) GC(keep map[string]bool) ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, e := range entries {
		sha, ok := strings.CutSuffix(e.Name(), s.ext)
		if !ok || strings.HasPrefix(e.Name(), ".") || keep[sha] {
			continue
		}
		if err := os.Remove(filepath.Join(s.dir, e.Name())); err != nil {
			return removed, err
		}
		removed = append(removed, sha)
	}
	sort.Strings(removed)
	return removed, nil
}
