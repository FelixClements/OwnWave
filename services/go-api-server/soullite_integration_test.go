//go:build integration

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"ownwave/api/internal/soullite"
	"ownwave/api/internal/spotify"
)

const soulLiteTestPlaylist = "37i9dQZF1DX6J5NfMJS675"

// soulLiteFixture puts a fake SoulLite in front of the handler. Every call it
// gets is sent on calls as "METHOD /path"; down makes it answer 500.
func soulLiteFixture(t *testing.T) (*stationsFixture, chan string, *atomic.Bool) {
	f := newStationsFixture(t)
	f.withSpotify(&fakeSpotify{lists: map[string]spotify.Playlist{
		soulLiteTestPlaylist: {Name: "Deep Focus"},
	}})
	calls := make(chan string, 16)
	down := new(atomic.Bool)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sl_test" {
			calls <- "missing API key"
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		calls <- r.Method + " " + r.URL.Path
		if down.Load() {
			http.Error(w, `{"detail":"boom"}`, http.StatusInternalServerError)
			return
		}
		switch r.Method + " " + r.URL.Path {
		case "POST /playlists":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":7,"url":"https://open.spotify.com/playlist/` + soulLiteTestPlaylist + `"}`))
		case "POST /playlists/7/sync":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":1}`))
		case "GET /playlists":
			_, _ = w.Write([]byte(`[{"id":7,"url":"https://open.spotify.com/playlist/` + soulLiteTestPlaylist + `"}]`))
		case "DELETE /playlists/7":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	client, err := soullite.New(srv.URL, "sl_test", srv.Client())
	require.NoError(t, err)
	f.h.soullite = client
	return f, calls, down
}

// expectCalls waits for the background calls, in order.
func expectCalls(t *testing.T, calls chan string, want ...string) {
	t.Helper()
	for _, w := range want {
		select {
		case got := <-calls:
			require.Equal(t, w, got)
		case <-time.After(5 * time.Second):
			t.Fatalf("SoulLite never got %s", w)
		}
	}
	select {
	case extra := <-calls:
		t.Fatalf("unexpected SoulLite call %s", extra)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSoulLiteFollowsAddedAndDeletedPlaylists(t *testing.T) {
	f, calls, _ := soulLiteFixture(t)
	admin := f.user("admin", true)

	rec := f.call(f.h.AddPlaylist, admin, http.MethodPost, "/admin/playlists", "",
		`{"url":"https://open.spotify.com/intl-de/playlist/`+soulLiteTestPlaylist+`?si=x"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	expectCalls(t, calls, "POST /playlists", "POST /playlists/7/sync")

	id := f.one(`SELECT id::text FROM playlists WHERE spotify_id = $1`, soulLiteTestPlaylist)
	rec = f.call(f.h.DeletePlaylist, admin, http.MethodDelete, "/admin/playlists", id, "")
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	expectCalls(t, calls, "GET /playlists", "DELETE /playlists/7")
}

func TestSoulLiteDownNeverFailsOwnWave(t *testing.T) {
	f, calls, down := soulLiteFixture(t)
	admin := f.user("admin", true)
	down.Store(true)

	rec := f.call(f.h.AddPlaylist, admin, http.MethodPost, "/admin/playlists", "",
		`{"url":"https://open.spotify.com/playlist/`+soulLiteTestPlaylist+`"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	expectCalls(t, calls, "POST /playlists")

	id := f.one(`SELECT id::text FROM playlists WHERE spotify_id = $1`, soulLiteTestPlaylist)
	rec = f.call(f.h.DeletePlaylist, admin, http.MethodDelete, "/admin/playlists", id, "")
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	expectCalls(t, calls, "GET /playlists")
}

func TestSoulLiteCatchUpSendsEveryPlaylist(t *testing.T) {
	f, calls, _ := soulLiteFixture(t)
	f.exec(`INSERT INTO playlists (spotify_id, name) VALUES ($1, 'Deep Focus')`, soulLiteTestPlaylist)

	f.h.followAllInSoulLite(context.Background())
	expectCalls(t, calls, "POST /playlists", "POST /playlists/7/sync")
}

func TestSoulLiteOffSendsNothing(t *testing.T) {
	f, calls, _ := soulLiteFixture(t)
	f.h.soullite = nil
	admin := f.user("admin", true)

	rec := f.call(f.h.AddPlaylist, admin, http.MethodPost, "/admin/playlists", "",
		`{"url":"https://open.spotify.com/playlist/`+soulLiteTestPlaylist+`"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	f.h.followAllInSoulLite(context.Background())
	expectCalls(t, calls)
}
