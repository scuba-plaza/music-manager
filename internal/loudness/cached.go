package loudness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Analyzer measures audio, a whole MP3 file as it is played.
type Analyzer interface {
	Measure(audio []byte) (Measurement, error)
}

// Cached remembers, under dir, what inner measured: one small file per audio,
// named by the SHA-256 of its bytes, so that the same bytes are never measured
// twice and an audio that changed is measured again. That the audio is too quiet
// to measure is remembered too; any other failure is not.
func Cached(dir string, inner Analyzer) Analyzer { return cached{dir: dir, inner: inner} }

type cached struct {
	dir   string
	inner Analyzer
}

type entry struct {
	LUFS   float64 `json:"lufs"`
	PeakDB float64 `json:"peak_db"`
	Silent bool    `json:"silent,omitempty"`
}

func (c cached) Measure(audio []byte) (Measurement, error) {
	sum := sha256.Sum256(audio)
	path := filepath.Join(c.dir, hex.EncodeToString(sum[:])+".json")

	if data, err := os.ReadFile(path); err == nil {
		var e entry
		if json.Unmarshal(data, &e) == nil { // a damaged entry is as good as none
			if e.Silent {
				return Measurement{}, ErrUnmeasurable
			}
			return Measurement{LUFS: e.LUFS, PeakDB: e.PeakDB}, nil
		}
	}

	m, err := c.inner.Measure(audio)
	switch {
	case err == nil:
		c.store(path, entry{LUFS: m.LUFS, PeakDB: m.PeakDB})
	case errors.Is(err, ErrUnmeasurable):
		c.store(path, entry{Silent: true})
	}
	return m, err
}

// store is best effort: an entry that cannot be written only means the audio is
// measured again next time.
func (c cached) store(path string, e entry) {
	data, err := json.Marshal(e)
	if err != nil || os.MkdirAll(c.dir, 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(c.dir, ".put-*")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return
	}
	if tmp.Close() != nil {
		return
	}
	os.Rename(tmp.Name(), path)
}
