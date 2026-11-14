package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAResetIsRefusedWithoutTheHeaderOrFromAnotherOrigin(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string]string
	}{
		{"no header", nil},
		{"another origin", map[string]string{"X-MM": "1", "Origin": "https://evil.example"}},
		{"another port", map[string]string{"X-MM": "1", "Origin": "http://127.0.0.1:1"}},
		{"an origin that is not a URL", map[string]string{"X-MM": "1", "Origin": "://"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7979/__e2e/reset", nil)
			for k, v := range c.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			(&seededServers{}).serveReset(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Errorf("status %d, want 403", rec.Code)
			}
		})
	}
}
