//go:build integration

package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"ownwave/api/internal/testutil/testdb"
)

// A signed-in user who is not an admin can neither read the Outside sources
// page nor save or test a key; an admin's request reaches Python as sent.
func TestOutsideSourceRoutesNeedAnAdmin(t *testing.T) {
	var hits []string
	var lastBody string
	py := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		hits = append(hits, r.Method+" "+r.URL.Path)
		lastBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"saved"}`))
	}))
	t.Cleanup(py.Close)

	pool := testdb.NewPool(t)
	ctx := context.Background()
	h := NewHandler(pool, Config{SetupToken: "env-setup-token-0123456789"})
	h.analyticsAPI = newAnalyticsProxy(py.URL)
	r := newRouter(h, t.TempDir())

	adminToken, admin, err := h.auth.Register(ctx, "alice", "password123", "", "env-setup-token-0123456789")
	require.NoError(t, err)
	invite, _, err := h.auth.CreateInvite(ctx, admin.ID, nil, time.Hour)
	require.NoError(t, err)
	userToken, _, err := h.auth.Register(ctx, "bob", "password123", invite, "")
	require.NoError(t, err)

	send := func(token, method, path string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(`{"source":"lastfm","key":"fake-key-0000"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		r.ServeHTTP(rec, req)
		return rec.Code
	}
	for _, rt := range outsideSourceRoutes {
		require.Equal(t, http.StatusForbidden, send(userToken, rt.method, rt.path), rt.path)
	}
	require.Empty(t, hits)

	for _, rt := range outsideSourceRoutes {
		require.Equal(t, http.StatusOK, send(adminToken, rt.method, rt.path), rt.path)
	}
	require.Equal(t, []string{
		"GET /outside-sources", "POST /outside-sources/settings", "POST /outside-sources/test",
	}, hits)
	require.JSONEq(t, `{"source":"lastfm","key":"fake-key-0000"}`, lastBody)
}
