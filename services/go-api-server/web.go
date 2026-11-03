package main

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
)

// mountWeb serves the web UI build (a single-page app) in dir at /: files
// by name, and index.html for every other path so the client router can
// handle it. It does nothing when dir has no index.html (tests, `go run`
// during development).
func mountWeb(r chi.Router, dir string) {
	root := os.DirFS(dir)
	if _, err := fs.Stat(root, "index.html"); err != nil {
		return
	}
	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		name, ok := webFile(root, req.URL.Path)
		if !ok {
			// A path under /assets/ names a build file: a miss is a real 404,
			// not a page (stale tabs after a deploy must not get HTML as JS).
			if strings.HasPrefix(req.URL.Path, "/assets/") {
				http.NotFound(w, req)
				return
			}
			name = "index.html"
		}
		if strings.HasPrefix(req.URL.Path, "/assets/") {
			// Build output names are content-hashed.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		http.ServeFileFS(w, req, root, name)
	})
}

// webFile maps a URL path to a file of the build. "/" and directories are
// never files (the app shell answers them).
func webFile(root fs.FS, urlPath string) (string, bool) {
	name := strings.TrimPrefix(path.Clean("/"+urlPath), "/")
	if name == "" || name == "index.html" {
		return "", false
	}
	info, err := fs.Stat(root, name)
	if err != nil || info.IsDir() {
		return "", false
	}
	return name, true
}
