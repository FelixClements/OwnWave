package soullite

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testKey = "sl_test_key"
	testID  = "37i9dQZF1DX6J5NfMJS675"
)

// fakeSoulLite records each call and answers from the handlers set per route.
type fakeSoulLite struct {
	mu     sync.Mutex
	calls  []string
	bodies []map[string]string
	routes map[string]http.HandlerFunc
}

func newFake(t *testing.T, routes map[string]http.HandlerFunc) (*fakeSoulLite, *Client) {
	t.Helper()
	f := &fakeSoulLite{routes: routes}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer "+testKey, r.Header.Get("Authorization"))
		route := r.Method + " " + r.URL.Path
		var body map[string]string
		if r.Body != nil && r.ContentLength != 0 {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		}
		f.mu.Lock()
		f.calls = append(f.calls, route)
		f.bodies = append(f.bodies, body)
		f.mu.Unlock()
		h, ok := f.routes[route]
		if !ok {
			http.Error(w, `{"detail":"Not Found"}`, http.StatusNotFound)
			return
		}
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	c, err := New(srv.URL+"/", testKey, srv.Client())
	require.NoError(t, err)
	return f, c
}

func reply(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestNew(t *testing.T) {
	c, err := New("", "", nil)
	require.NoError(t, err)
	require.Nil(t, c, "both empty turns the integration off")

	for _, tc := range []struct{ url, key string }{
		{"http://soullite:8080", ""},
		{"", testKey},
		{"soullite:8080", testKey},
		{"ftp://soullite", testKey},
		{"http://", testKey},
	} {
		_, err := New(tc.url, tc.key, nil)
		require.Error(t, err, "%q %q", tc.url, tc.key)
	}

	c, err = New(" http://soullite:8080/ ", " "+testKey+" ", nil)
	require.NoError(t, err)
	require.Equal(t, "http://soullite:8080", c.BaseURL())
}

func TestFollowStartsSync(t *testing.T) {
	f, c := newFake(t, map[string]http.HandlerFunc{
		"POST /playlists":        reply(http.StatusCreated, `{"id":7,"url":"https://open.spotify.com/playlist/`+testID+`"}`),
		"POST /playlists/7/sync": reply(http.StatusAccepted, `{"job_id":3}`),
	})
	require.NoError(t, c.Follow(context.Background(), testID, "Deep Focus"))
	require.Equal(t, []string{"POST /playlists", "POST /playlists/7/sync"}, f.calls)
	require.Equal(t, map[string]string{
		"url":      "https://open.spotify.com/playlist/" + testID,
		"name":     "Deep Focus",
		"schedule": "daily",
	}, f.bodies[0])
}

func TestFollowAlreadyFollowed(t *testing.T) {
	f, c := newFake(t, map[string]http.HandlerFunc{
		"POST /playlists": reply(http.StatusConflict, `{"detail":"This Playlist is already followed"}`),
	})
	require.NoError(t, c.Follow(context.Background(), testID, ""))
	require.Equal(t, []string{"POST /playlists"}, f.calls, "no extra Sync for a playlist already followed")
	require.NotContains(t, f.bodies[0], "name")
}

func TestFollowErrors(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusUnprocessableEntity, http.StatusInternalServerError} {
		_, c := newFake(t, map[string]http.HandlerFunc{
			"POST /playlists": reply(status, `{"detail":"nope"}`),
		})
		err := c.Follow(context.Background(), testID, "x")
		require.ErrorContains(t, err, "nope", "status %d", status)
		require.NotContains(t, err.Error(), testKey)
	}

	_, c := newFake(t, map[string]http.HandlerFunc{
		"POST /playlists": reply(http.StatusCreated, `{"id":7}`),
		// No sync route: the fake answers 404.
	})
	require.ErrorContains(t, c.Follow(context.Background(), testID, "x"), "sync: HTTP 404")
}

func TestUnfollowMatchesSpotifyID(t *testing.T) {
	f, c := newFake(t, map[string]http.HandlerFunc{
		"GET /playlists": reply(http.StatusOK, `[
			{"id":1,"url":"https://open.spotify.com/playlist/otherPlaylist123"},
			{"id":2,"url":"https://open.spotify.com/playlist/`+testID+`"},
			{"id":3,"url":"not a url"}
		]`),
		"DELETE /playlists/2": reply(http.StatusNoContent, ""),
	})
	require.NoError(t, c.Unfollow(context.Background(), testID))
	require.Equal(t, []string{"GET /playlists", "DELETE /playlists/2"}, f.calls)
}

func TestUnfollowNotFollowed(t *testing.T) {
	f, c := newFake(t, map[string]http.HandlerFunc{
		"GET /playlists": reply(http.StatusOK, `[]`),
	})
	require.NoError(t, c.Unfollow(context.Background(), testID))
	require.Equal(t, []string{"GET /playlists"}, f.calls)
}

func TestUnfollowGoneMeanwhile(t *testing.T) {
	_, c := newFake(t, map[string]http.HandlerFunc{
		"GET /playlists":      reply(http.StatusOK, `[{"id":2,"url":"https://open.spotify.com/playlist/`+testID+`"}]`),
		"DELETE /playlists/2": reply(http.StatusNotFound, `{"detail":"No such Playlist"}`),
	})
	require.NoError(t, c.Unfollow(context.Background(), testID))
}

func TestUnfollowErrors(t *testing.T) {
	_, c := newFake(t, map[string]http.HandlerFunc{
		"GET /playlists": reply(http.StatusUnauthorized, `{"detail":"A valid key is required"}`),
	})
	require.ErrorContains(t, c.Unfollow(context.Background(), testID), "HTTP 401")

	_, c = newFake(t, map[string]http.HandlerFunc{
		"GET /playlists":      reply(http.StatusOK, `[{"id":2,"url":"https://open.spotify.com/playlist/`+testID+`"}]`),
		"DELETE /playlists/2": reply(http.StatusForbidden, `{"detail":"read-only key"}`),
	})
	require.ErrorContains(t, c.Unfollow(context.Background(), testID), "HTTP 403")
}
