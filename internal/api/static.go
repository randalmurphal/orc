package api

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:static
var staticFiles embed.FS

// staticFS returns the embedded static files as an fs.FS rooted at "static/".
// Returns nil if the static directory doesn't exist (dev mode without build).
func staticFS() fs.FS {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil
	}
	return sub
}

// staticHandler returns an http.Handler that serves the embedded static files.
// It implements SPA routing: serves index.html for any path that doesn't match a file.
func staticHandler() http.Handler {
	fsys := staticFS()
	if fsys == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Frontend not embedded. Run 'make build' to include it.", http.StatusNotFound)
		})
	}

	fileServer := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")

		// Try to serve the file directly
		if path != "" {
			if _, err := fs.Stat(fsys, path); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// For SPA: serve index.html for all unmatched routes
		// But not for /api/* routes (those should 404)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Serve index.html
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
