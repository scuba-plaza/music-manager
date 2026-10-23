package app

import (
	"context"
	"time"
)

// Query describes a track to look up. Empty fields are not searched for.
type Query struct{ Title, Artist, Album string }

// Candidate is one way a metadata source says a track could be described: a
// recording as it appears on one particular release.
type Candidate struct {
	RecordingID, ReleaseID string
	Title, Artist, Album   string
	TrackNumber            int
	TrackTotal             int
	Length                 time.Duration
	Score                  int    // 0-100, how well the candidate matches the query
	Date                   string // when the release came out, as precise as known
}

// MetadataSource looks tracks up in a music database.
type MetadataSource interface {
	// Search returns candidates, best first.
	Search(ctx context.Context, q Query) ([]Candidate, error)
	// CoverFront returns the front cover of a release, or nil if it has none.
	CoverFront(ctx context.Context, releaseID string) ([]byte, error)
}
