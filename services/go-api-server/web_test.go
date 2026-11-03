package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"ownwave/api/internal/analytics"
)

func writeWebFile(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
}

func get(r http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestRouterServesAPIUnderPrefix(t *testing.T) {
	r := newRouter(newTestHandler(analytics.NewStub()), t.TempDir())

	require.Equal(t, http.StatusUnauthorized, get(r, "/api/tracks").Code)
	require.Equal(t, http.StatusNotFound, get(r, "/tracks").Code)
	require.Equal(t, http.StatusOK, get(r, "/health").Code)
}

func TestRouterSetsSecurityHeaders(t *testing.T) {
	h := newTestHandler(analytics.NewStub())
	rec := get(newRouter(h, t.TempDir()), "/api/tracks")
	require.Contains(t, rec.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'")
	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	require.Empty(t, rec.Header().Get("Strict-Transport-Security"), "no HSTS over plain HTTP")

	h.cookieSecure = true
	rec = get(newRouter(h, t.TempDir()), "/health")
	require.Equal(t, "max-age=31536000; includeSubDomains", rec.Header().Get("Strict-Transport-Security"))
}

func TestRouterServesWebUI(t *testing.T) {
	dir := t.TempDir()
	writeWebFile(t, dir, "index.html", "app shell")
	writeWebFile(t, dir, "favicon.svg", "<svg/>")
	writeWebFile(t, dir, "assets/index-abc123.js", "js")
	r := newRouter(newTestHandler(analytics.NewStub()), dir)

	home := get(r, "/")
	require.Equal(t, "app shell", home.Body.String())
	require.Equal(t, "no-cache", home.Header().Get("Cache-Control"))

	// Client-side routes get the app shell; the API keeps its own answers.
	for _, page := range []string{"/library", "/station/6f1b8c6e-6a43-4e4b-9b1e-2b3f8d1a0c55", "/invite/tok123"} {
		rec := get(r, page)
		require.Equal(t, http.StatusOK, rec.Code, page)
		require.Equal(t, "app shell", rec.Body.String(), page)
	}
	require.Equal(t, http.StatusUnauthorized, get(r, "/api/history").Code)
	require.Equal(t, http.StatusNotFound, get(r, "/api/nope").Code)

	asset := get(r, "/assets/index-abc123.js")
	require.Equal(t, "js", asset.Body.String())
	require.Equal(t, "public, max-age=31536000, immutable", asset.Header().Get("Cache-Control"))
	require.Equal(t, http.StatusNotFound, get(r, "/assets/index-old999.js").Code, "a stale asset is a 404, not HTML")
	require.Equal(t, http.StatusNotFound, get(r, "/assets/").Code)
	require.Equal(t, "<svg/>", get(r, "/favicon.svg").Body.String())
	require.Equal(t, http.StatusBadRequest, get(r, "/../index.html").Code)
}
