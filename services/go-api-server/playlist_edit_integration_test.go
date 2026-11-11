//go:build integration

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"ownwave/api/internal/auth"
	"ownwave/api/internal/playlists"
)

// callItem calls a playlist-item handler with the {id} and {spotifyTrackId} URL params.
func (f *stationsFixture) callItem(handler http.HandlerFunc, user auth.User, method, playlistID, spotifyTrackID, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/admin/playlists/x/items/y", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", playlistID)
	rctx.URLParams.Add("spotifyTrackId", spotifyTrackID)
	req = req.WithContext(context.WithValue(auth.WithUser(req.Context(), user), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func TestPlaylistEditHandlers(t *testing.T) {
	f := newStationsFixture(t)
	admin := f.user("admin", true)
	p := f.playlist("bunker", 2, "techno")
	sp := "sp-bunker-0"
	other := f.one(`INSERT INTO tracks (path, title) VALUES ('/m/other', 'Other') RETURNING id::text`)

	items := func(filter string) map[string]playlists.Item {
		t.Helper()
		rec := f.call(f.h.GetPlaylist, admin, http.MethodGet, "/playlists/x?items="+filter, p, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		out := map[string]playlists.Item{}
		for _, it := range decode[struct{ Items []playlists.Item }](t, rec).Items {
			out[it.SpotifyTrackID] = it
		}
		return out
	}

	for name, c := range map[string]struct {
		handler        http.HandlerFunc
		playlist, item string
		body           string
		want           int
	}{
		"match without track_id": {f.h.SetPlaylistMatch, p, sp, `{}`, http.StatusBadRequest},
		"match with bad uuid":    {f.h.SetPlaylistMatch, p, sp, `{"track_id":"nope"}`, http.StatusBadRequest},
		"match to unknown track": {f.h.SetPlaylistMatch, p, sp, `{"track_id":"00000000-0000-0000-0000-000000000001"}`, http.StatusBadRequest},
		"match unknown item":     {f.h.SetPlaylistMatch, p, "sp-nope", `{"track_id":null}`, http.StatusNotFound},
		"match unknown playlist": {f.h.SetPlaylistMatch, "00000000-0000-0000-0000-000000000001", sp, `{"track_id":null}`, http.StatusNotFound},
		"match bad playlist id":  {f.h.SetPlaylistMatch, "nope", sp, `{"track_id":null}`, http.StatusBadRequest},
		"hide without hidden":    {f.h.SetPlaylistItemHidden, p, sp, `{}`, http.StatusBadRequest},
		"hide unknown item":      {f.h.SetPlaylistItemHidden, p, "sp-nope", `{"hidden":true}`, http.StatusNotFound},
		"reset unknown item":     {f.h.ResetPlaylistMatch, p, "sp-nope", ``, http.StatusNotFound},
		"too long spotify id":    {f.h.ResetPlaylistMatch, p, strings.Repeat("x", 65), ``, http.StatusBadRequest},
	} {
		rec := f.callItem(c.handler, admin, http.MethodPut, c.playlist, c.item, c.body)
		require.Equal(t, c.want, rec.Code, name+": "+rec.Body.String())
	}

	// Pick a file by hand, then mark it as having none.
	rec := f.callItem(f.h.SetPlaylistMatch, admin, http.MethodPut, p, sp, `{"track_id":"`+other+`"}`)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	got := items("matched")[sp]
	require.Equal(t, other, *got.TrackID)
	require.True(t, got.Manual)

	rec = f.callItem(f.h.SetPlaylistMatch, admin, http.MethodPut, p, sp, `{"track_id":null}`)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	require.True(t, items("missing")[sp].Manual)

	// Hide and list the hidden ones.
	rec = f.callItem(f.h.SetPlaylistItemHidden, admin, http.MethodPut, p, "sp-bunker-1", `{"hidden":true}`)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	require.Contains(t, items("hidden"), "sp-bunker-1")
	require.NotContains(t, items("matched"), "sp-bunker-1")

	// Reset: the matcher finds nothing for the fixture's titles, so it stays missing but not manual.
	rec = f.callItem(f.h.ResetPlaylistMatch, admin, http.MethodDelete, p, sp, ``)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	require.False(t, items("")[sp].Manual)

	require.Equal(t, http.StatusBadRequest, f.call(f.h.GetPlaylist, admin, http.MethodGet, "/playlists/x?items=bogus", p, "").Code)
}
