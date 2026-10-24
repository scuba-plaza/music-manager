package web

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// guard protects the server from web pages that are not its own. The program
// can delete files and write to a memory card, and a page the person has open
// in another tab may try to make it do so: by sending requests to
// 127.0.0.1 (a cross-site request), or by getting its own name to point at
// 127.0.0.1 (DNS rebinding). So:
//
//   - the Host must be a name of this computer, which stops rebinding: the
//     request then carries the stranger's name;
//   - a request that changes anything must come from this server's own
//     page (its Origin is this server) and carry the X-MM header, which a
//     web page of another origin cannot send without a preflight request, and
//     none is ever answered.
func guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !localHost(r.Host) {
			fail(w, http.StatusForbidden, errors.New("this address is not allowed"))
			return
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead:
		default:
			if r.Header.Get("X-MM") == "" {
				fail(w, http.StatusForbidden, errors.New("the X-MM header is missing"))
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Host != r.Host {
					fail(w, http.StatusForbidden, errors.New("the request does not come from this page"))
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// localHost reports whether the Host header names this computer.
func localHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	return loopback(host)
}
