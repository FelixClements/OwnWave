//go:build integration

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"ownwave/api/internal/auth"
	"ownwave/api/internal/testdb"
)

func TestSharedPlaylistStations(t *testing.T) {
	pool := testdb.NewPool(t)
	ctx := context.Background()
	h := NewHandler(pool, Config{})

	newUser := func(name string, admin bool) auth.User {
		u := auth.User{Username: name, IsAdmin: admin}
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO users (username, password_hash, is_admin) VALUES ($1, 'x', $2) RETURNING id::text`,
			name, admin).Scan(&u.ID))
		return u
	}
	admin, alice := newUser("admin", true), newUser("alice", false)
	var playlistID string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO playlists (spotify_id, name) VALUES ('sp', 'Rock') RETURNING id::text`).Scan(&playlistID))

	call := func(handler http.HandlerFunc, user auth.User, method, stationID, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/stations", strings.NewReader(body))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", stationID)
		req = req.WithContext(context.WithValue(auth.WithUser(req.Context(), user), chi.RouteCtxKey, rctx))
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec
	}

	rec := call(h.CreateStation, admin, http.MethodPost, "", `{"name":"Rock","seed_type":"playlists","playlist_ids":["6f1b8c6e-6a43-4e4b-9b1e-2b3f8d1a0c55"]}`)
	require.Equal(t, http.StatusBadRequest, rec.Code, "unknown playlist")

	rec = call(h.CreateStation, admin, http.MethodPost, "", `{"name":"Rock","seed_type":"playlists","playlist_ids":["`+playlistID+`"]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var created struct {
		StationID string `json:"station_id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	shared := created.StationID

	// Everyone sees and plays the shared station.
	list, err := h.db.ListStations(ctx, alice.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.True(t, list[0].Shared)
	require.Equal(t, http.StatusOK, call(h.GetQueue, alice, http.MethodGet, shared, "").Code)

	// Only admins change it.
	require.Equal(t, http.StatusForbidden, call(h.UpdateStation, alice, http.MethodPut, shared, `{"name":"Mine"}`).Code)
	require.Equal(t, http.StatusForbidden, call(h.DeleteStation, alice, http.MethodDelete, shared, "").Code)
	require.Equal(t, http.StatusOK, call(h.UpdateStation, admin, http.MethodPut, shared, `{"name":"Rock & Roll"}`).Code)

	// A station never switches between personal and shared.
	rec = call(h.UpdateStation, admin, http.MethodPut, shared, `{"seed_type":"mood","min_energy":0.5}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	rec = call(h.CreateStation, alice, http.MethodPost, "", `{"name":"Calm","seed_type":"mood"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	rec = call(h.UpdateStation, alice, http.MethodPut, created.StationID, `{"seed_type":"playlists","playlist_ids":["`+playlistID+`"]}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// Personal stations stay private.
	require.Equal(t, http.StatusNotFound, call(h.DeleteStation, admin, http.MethodDelete, created.StationID, "").Code)
	require.Equal(t, http.StatusNoContent, call(h.DeleteStation, admin, http.MethodDelete, shared, "").Code)
}
