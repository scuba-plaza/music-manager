package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var distFiles embed.FS

func WithFrontend(fsys fs.FS) Option { return func(v *Server) { v.frontend = fsys } }

const (
	cacheRevalidate = "no-cache"
	cacheForever    = "public, max-age=31536000, immutable"
)

const notBuiltPage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>mm: front end not built</title>
</head>
<body>
<h1>The front end is not built</h1>
<p>This program was started without its web page. To build it, run:</p>
<pre><code>pnpm --dir web run build</code></pre>
<p>then start mm again. The API at <code>/api</code> works meanwhile.</p>
</body>
</html>
`

func frontendHandler(fsys fs.FS) http.Handler {
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fs.Stat(fsys, "index.html"); err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(notBuiltPage))
			return
		}
		cache := cacheRevalidate
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if strings.HasPrefix(name, "assets/") {
			if info, err := fs.Stat(fsys, name); err == nil && !info.IsDir() {
				cache = cacheForever
			}
		}
		w.Header().Set("Cache-Control", cache)
		files.ServeHTTP(w, r)
	})
}
