package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"ownwave/api/internal/analytics"
)

// The Outside sources routes (issue #148) save and test API keys: admin only.
var outsideSourceRoutes = []struct{ method, path string }{
	{http.MethodGet, "/api/admin/outside-sources"},
	{http.MethodPost, "/api/admin/outside-sources/settings"},
	{http.MethodPost, "/api/admin/outside-sources/test"},
}

func TestOutsideSourceRoutesAreAdminOnly(t *testing.T) {
	admin := map[string]bool{}
	for _, rt := range analytics.RoutesFor(analytics.AdminOnly) {
		admin[rt.Method+" /api"+rt.Public] = true
	}
	for _, rt := range outsideSourceRoutes {
		require.True(t, admin[rt.method+" "+rt.path], rt.path)
	}
}

func TestOutsideSourceRoutesRejectAnonymousCallers(t *testing.T) {
	py, got := pythonStub(t, http.StatusOK, `{}`)
	h := newTestHandler(analytics.NewStub())
	h.analyticsAPI = newAnalyticsProxy(py.URL)
	r := newRouter(h, t.TempDir())

	for _, rt := range outsideSourceRoutes {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(`{"source":"discogs","key":"fake-key-0000"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code, rt.path)
	}
	require.Empty(t, *got, "nothing reaches Python without an admin")
}
