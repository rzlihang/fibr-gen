package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// StaticHandler serves static files from a directory with SPA fallback.
// For any path that doesn't match a real file and isn't an API route,
// it serves index.html so client-side routing works.
func StaticHandler(dir string) http.Handler {
	fs := http.Dir(dir)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't serve static files for API routes
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/health" {
			http.NotFound(w, r)
			return
		}

		// Try to serve the requested file
		path := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			http.FileServer(fs).ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for any unmatched route
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}
