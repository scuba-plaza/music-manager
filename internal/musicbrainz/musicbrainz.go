// Package musicbrainz looks tracks up in the MusicBrainz database and
// fetches cover art from the Cover Art Archive.
package musicbrainz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"music-manager/internal/app"
)

const (
	maxCandidates = 50
	maxRetries    = 3
	maxBody       = 16 << 20
)

// Client talks to MusicBrainz. It spaces its requests out as the service's
// rules require and identifies itself with UserAgent. Its fields can be
// replaced for tests.
type Client struct {
	BaseURL   string // https://musicbrainz.org by default
	CoverURL  string // https://coverartarchive.org by default
	UserAgent string
	HTTP      *http.Client
	Interval  time.Duration // minimum time between requests
	Sleep     func(time.Duration)
	Now       func() time.Time

	mu   sync.Mutex
	last time.Time
}

// New returns a client that sends userAgent, which MusicBrainz asks to be
// "AppName/version (contact)".
func New(userAgent string) *Client {
	return &Client{
		BaseURL:   "https://musicbrainz.org",
		CoverURL:  "https://coverartarchive.org",
		UserAgent: userAgent,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		Interval:  1200 * time.Millisecond, // the service allows one request a second on average
		Sleep:     time.Sleep,
		Now:       time.Now,
	}
}

// get fetches a URL, waiting for its turn and retrying while the service
// says it is busy.
func (c *Client) get(ctx context.Context, rawURL, accept string) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		c.waitTurn()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.UserAgent)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusServiceUnavailable || attempt == maxRetries {
			return resp, nil
		}
		wait := time.Duration(attempt+1) * 2 * time.Second
		if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil {
			wait = time.Duration(secs) * time.Second
		}
		resp.Body.Close()
		c.Sleep(wait)
	}
}

// waitTurn blocks until Interval has passed since the previous request.
func (c *Client) waitTurn() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.last.IsZero() {
		if d := c.Interval - c.Now().Sub(c.last); d > 0 {
			c.Sleep(d)
		}
	}
	c.last = c.Now()
}

// Search finds recordings matching q, one candidate per release each is on.
func (c *Client) Search(ctx context.Context, q app.Query) ([]app.Candidate, error) {
	query := buildQuery(q)
	if query == "" {
		return nil, errors.New("musicbrainz: nothing to search for")
	}
	params := url.Values{"query": {query}, "limit": {"25"}, "fmt": {"json"}}
	resp, err := c.get(ctx, c.BaseURL+"/ws/2/recording?"+params.Encode(), "application/json")
	if err != nil {
		return nil, fmt.Errorf("musicbrainz: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz: HTTP %d", resp.StatusCode)
	}
	var body searchResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&body); err != nil {
		return nil, fmt.Errorf("musicbrainz: unreadable response: %w", err)
	}
	return body.candidates(), nil
}

// CoverFront downloads the front cover of a release, scaled to 500px, or
// returns nil if the Cover Art Archive has none.
func (c *Client) CoverFront(ctx context.Context, releaseID string) ([]byte, error) {
	resp, err := c.get(ctx, c.CoverURL+"/release/"+url.PathEscape(releaseID)+"/front-500", "")
	if err != nil {
		return nil, fmt.Errorf("cover art: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return io.ReadAll(io.LimitReader(resp.Body, maxBody))
	case http.StatusNotFound:
		return nil, nil
	}
	return nil, fmt.Errorf("cover art: HTTP %d", resp.StatusCode)
}

func buildQuery(q app.Query) string {
	var parts []string
	for _, f := range []struct{ field, value string }{{"recording", q.Title}, {"artist", q.Artist}, {"release", q.Album}} {
		if v := strings.TrimSpace(f.value); v != "" {
			parts = append(parts, f.field+`:"`+escape(v)+`"`)
		}
	}
	return strings.Join(parts, " AND ")
}

func escape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}

type searchResponse struct {
	Recordings []struct {
		ID     string `json:"id"`
		Score  int    `json:"score"`
		Title  string `json:"title"`
		Length int    `json:"length"`
		Credit []struct {
			Name       string `json:"name"`
			JoinPhrase string `json:"joinphrase"`
		} `json:"artist-credit"`
		Releases []struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Status string `json:"status"`
			Date   string `json:"date"`
			Count  int    `json:"track-count"`
			Media  []struct {
				Count int `json:"track-count"`
				Track []struct {
					Number string `json:"number"`
				} `json:"track"`
			} `json:"media"`
		} `json:"releases"`
	} `json:"recordings"`
}

type ranked struct {
	app.Candidate
	official bool
}

func (r *searchResponse) candidates() []app.Candidate {
	var all []ranked
	for _, rec := range r.Recordings {
		var artist strings.Builder
		for _, cr := range rec.Credit {
			artist.WriteString(cr.Name + cr.JoinPhrase)
		}
		for _, rel := range rec.Releases {
			c := app.Candidate{
				RecordingID: rec.ID, ReleaseID: rel.ID,
				Title: rec.Title, Artist: artist.String(), Album: rel.Title,
				TrackTotal: rel.Count, Length: time.Duration(rec.Length) * time.Millisecond,
				Score: rec.Score, Date: rel.Date,
			}
			if len(rel.Media) > 0 && len(rel.Media[0].Track) > 0 {
				c.TrackNumber, _ = strconv.Atoi(rel.Media[0].Track[0].Number)
				c.TrackTotal = rel.Media[0].Count
			}
			all = append(all, ranked{c, rel.Status == "Official"})
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		a, b := all[i], all[j]
		switch {
		case a.Score != b.Score:
			return a.Score > b.Score
		case a.official != b.official:
			return a.official
		case (a.TrackNumber > 0) != (b.TrackNumber > 0):
			return a.TrackNumber > 0 // a candidate that says which track this is beats one that does not
		case a.Date != b.Date && a.Date != "" && b.Date != "":
			return a.Date < b.Date
		case (a.Date == "") != (b.Date == ""):
			return a.Date != ""
		}
		return false
	})
	out := make([]app.Candidate, 0, min(len(all), maxCandidates))
	for _, r := range all[:min(len(all), maxCandidates)] {
		out = append(out, r.Candidate)
	}
	return out
}
