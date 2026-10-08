package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

const placeholder = `<!doctype html><meta charset="utf-8"><title>Agent Sentinel</title>
<body style="font-family:system-ui;background:#0b0d12;color:#e6e8ee;padding:3rem;max-width:40rem">
<h1>Agent Sentinel</h1>
<p>The cockpit UI is not built into this binary.</p>
<p>Run <code>make web</code> (or <code>cd web &amp;&amp; npm install &amp;&amp; npm run build</code>) and rebuild.
The JSON API is still available at <a href="/api/session" style="color:#7aa2ff">/api/session</a>
and <a href="/api/events" style="color:#7aa2ff">/api/events</a>.</p>`

// spa serves the embedded single-page app, falling back to index.html for
// unknown paths (client-side routing) and to a friendly placeholder when the
// frontend has not been built.
func (s *Server) spa() http.Handler {
	if s.assets == nil {
		return placeholderHandler()
	}
	if _, err := fs.Stat(s.assets, "index.html"); err != nil {
		return placeholderHandler()
	}
	files := http.FileServerFS(s.assets)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p != "" {
			if _, err := fs.Stat(s.assets, p); err != nil {
				r.URL.Path = "/" // SPA fallback
			}
		}
		files.ServeHTTP(w, r)
	})
}

func placeholderHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(placeholder))
	})
}
