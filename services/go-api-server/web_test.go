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
	writeWebFile(t, dir, "index.html", "home")
	writeWebFile(t, dir, "404.html", "missing")
	writeWebFile(t, dir, "history.html", "history page")
	writeWebFile(t, dir, "invite.html", "invite page")
	writeWebFile(t, dir, "_next/static/app.js", "js")
	r := newRouter(newTestHandler(analytics.NewStub()), dir)

	require.Equal(t, "home", get(r, "/").Body.String())
	// The UI page and the API endpoint share a name without colliding.
	require.Equal(t, "history page", get(r, "/history").Body.String())
	require.Equal(t, http.StatusUnauthorized, get(r, "/api/history").Code)

	invite := get(r, "/invite/tok123")
	require.Equal(t, http.StatusOK, invite.Code)
	require.Equal(t, "invite page", invite.Body.String())

	static := get(r, "/_next/static/app.js")
	require.Equal(t, "public, max-age=31536000, immutable", static.Header().Get("Cache-Control"))

	missing := get(r, "/nope")
	require.Equal(t, http.StatusNotFound, missing.Code)
	require.Equal(t, "missing", missing.Body.String())
	// Directories are not listed.
	require.Equal(t, http.StatusNotFound, get(r, "/_next/static/").Code)
	require.Equal(t, http.StatusBadRequest, get(r, "/../index.html").Code)
	require.Equal(t, http.StatusNotFound, get(r, "/api/nope").Code)
}
