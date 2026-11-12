//go:build integration

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"ownwave/api/internal/testdb"
)

func TestSetupStatusHidesTrackCountOnceAUserExists(t *testing.T) {
	pool := testdb.NewPool(t)
	ctx := context.Background()
	h := NewHandler(pool, Config{})
	_, err := pool.Exec(ctx, `INSERT INTO tracks (path, title) VALUES ('/m/a.flac', 'a'), ('/m/b.flac', 'b')`)
	require.NoError(t, err)

	status := func() map[string]any {
		t.Helper()
		rec := httptest.NewRecorder()
		h.SetupStatus(rec, httptest.NewRequest(http.MethodGet, "/api/setup/status", nil))
		require.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		return body
	}

	fresh := status()
	require.Equal(t, false, fresh["has_users"])
	require.EqualValues(t, 2, fresh["track_count"])

	// An admin exists but setup is not completed yet: still no track count.
	_, err = pool.Exec(ctx, `INSERT INTO users (username, password_hash, is_admin) VALUES ('admin', 'x', true)`)
	require.NoError(t, err)
	withUser := status()
	require.Equal(t, true, withUser["has_users"])
	require.Equal(t, false, withUser["setup_completed"])
	require.NotContains(t, withUser, "track_count")
}
