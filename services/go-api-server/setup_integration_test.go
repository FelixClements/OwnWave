//go:build integration

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"ownwave/api/internal/testdb"
)

func setupStatus(t *testing.T, h *Handler, cookies ...*http.Cookie) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.SetupStatus(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

func register(t *testing.T, h *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.Register(rec, httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(body)))
	return rec
}

func TestSetupStatusShowsTrackCountOnlyToAdmin(t *testing.T) {
	pool := testdb.NewPool(t)
	ctx := context.Background()
	h := NewHandler(pool, Config{SetupToken: "env-setup-token-0123456789"})
	_, err := pool.Exec(ctx, `INSERT INTO tracks (path, title) VALUES ('/m/a.flac', 'a'), ('/m/b.flac', 'b')`)
	require.NoError(t, err)

	// Anyone can ask before the first account exists: no library size.
	fresh := setupStatus(t, h)
	require.Equal(t, false, fresh["has_users"])
	require.Equal(t, true, fresh["setup_token_required"])
	require.NotContains(t, fresh, "track_count")

	rec := register(t, h, `{"username":"admin","password":"password123","setup_token":"env-setup-token-0123456789"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// The admin finishing setup sees the track count; anonymous callers do not.
	withUser := setupStatus(t, h)
	require.Equal(t, true, withUser["has_users"])
	require.Equal(t, false, withUser["setup_token_required"])
	require.Equal(t, false, withUser["setup_completed"])
	require.NotContains(t, withUser, "track_count")

	admin := setupStatus(t, h, rec.Result().Cookies()...)
	require.EqualValues(t, 2, admin["track_count"])
}

func TestRegisterFirstAccountNeedsSetupToken(t *testing.T) {
	pool := testdb.NewPool(t)
	h := NewHandler(pool, Config{SetupToken: "env-setup-token-0123456789"})

	for name, body := range map[string]string{
		"missing": `{"username":"admin","password":"password123"}`,
		"wrong":   `{"username":"admin","password":"password123","setup_token":"guess"}`,
		"invite":  `{"username":"admin","password":"password123","invite_token":"env-setup-token-0123456789"}`,
	} {
		rec := register(t, h, body)
		require.Equal(t, http.StatusForbidden, rec.Code, name)
		require.Contains(t, rec.Body.String(), "invalid setup token", name)
	}
	require.Equal(t, false, setupStatus(t, h)["has_users"])

	rec := register(t, h, `{"username":"admin","password":"password123","setup_token":"env-setup-token-0123456789"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"is_admin":true`)

	// Afterwards the setup token opens nothing; an invite still works.
	rec = register(t, h, `{"username":"bob","password":"password123","setup_token":"env-setup-token-0123456789"}`)
	require.Equal(t, http.StatusForbidden, rec.Code)

	users, err := h.auth.ListUsers(context.Background())
	require.NoError(t, err)
	invite, _, err := h.auth.CreateInvite(context.Background(), users[0].ID, nil, time.Hour)
	require.NoError(t, err)
	rec = register(t, h, `{"username":"bob","password":"password123","invite_token":"`+invite+`"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"is_admin":false`)
}

func TestRegisterFirstAccountWithGeneratedSetupToken(t *testing.T) {
	pool := testdb.NewPool(t)
	h := NewHandler(pool, Config{})
	require.True(t, h.setup.generated)
	require.NotEmpty(t, h.setup.value)

	rec := register(t, h, `{"username":"admin","password":"password123"}`)
	require.Equal(t, http.StatusForbidden, rec.Code)

	rec = register(t, h, `{"username":"admin","password":"password123","setup_token":"`+h.setup.value+`"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestSetupRoutesNeedAdminBeforeFirstAccount(t *testing.T) {
	pool := testdb.NewPool(t)
	h := NewHandler(pool, Config{})
	called := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })

	rec := httptest.NewRecorder()
	h.requireSetupAccess(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/setup/summary", nil))
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.False(t, called)
}
