package main

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
)

// mountWeb serves the statically exported web UI in dir at /. It does
// nothing when dir has no index.html (tests, `go run` during development).
func mountWeb(r chi.Router, dir string) {
	root := os.DirFS(dir)
	if _, err := fs.Stat(root, "index.html"); err != nil {
		return
	}

	// One page handles every invite link; it reads the token from the URL.
	r.Get("/invite/*", func(w http.ResponseWriter, req *http.Request) {
		http.ServeFileFS(w, req, root, "invite.html")
	})
	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		name, ok := webFile(root, req.URL.Path)
		if !ok {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			page, _ := fs.ReadFile(root, "404.html")
			w.Write(page)
			return
		}
		if strings.HasPrefix(req.URL.Path, "/_next/static/") {
			// Build output names are content-hashed.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.ServeFileFS(w, req, root, name)
	})
}

// webFile maps a URL path to an exported file: assets by name, pages as
// <path>.html (/ is index.html). Directories are never served.
func webFile(root fs.FS, urlPath string) (string, bool) {
	name := strings.TrimPrefix(path.Clean("/"+urlPath), "/")
	if name == "" {
		name = "index"
	}
	for _, candidate := range []string{name, name + ".html"} {
		if info, err := fs.Stat(root, candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}
