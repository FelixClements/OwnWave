//go:build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"ownwave/api/internal/auth"
	"ownwave/api/internal/stations"
	"ownwave/api/internal/testdb"
)

type stationsFixture struct {
	t    *testing.T
	ctx  context.Context
	h    *Handler
	exec func(sql string, args ...any)
	one  func(sql string, args ...any) string
}

func newStationsFixture(t *testing.T) *stationsFixture {
	pool := testdb.NewPool(t)
	ctx := context.Background()
	f := &stationsFixture{t: t, ctx: ctx, h: NewHandler(pool, Config{})}
	f.exec = func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err)
	}
	f.one = func(sql string, args ...any) string {
		t.Helper()
		var v string
		require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&v))
		return v
	}
	f.exec(`INSERT INTO station_genres (id, name, position, is_station) VALUES
		('techno', 'Techno', 1, true), ('house', 'House', 2, true), ('other', 'Other', 99, false)`)
	return f
}

func (f *stationsFixture) user(name string, admin bool) auth.User {
	u := auth.User{Username: name, IsAdmin: admin}
	u.ID = f.one(`INSERT INTO users (username, password_hash, is_admin) VALUES ($1, 'x', $2) RETURNING id::text`, name, admin)
	return u
}

// playlist adds a playlist with n matched tracks of the given genre.
func (f *stationsFixture) playlist(name string, n int, genre string) string {
	id := f.one(`INSERT INTO playlists (spotify_id, name) VALUES ($1, $1) RETURNING id::text`, name)
	for i := 0; i < n; i++ {
		track := f.one(`INSERT INTO tracks (path, title) VALUES ($1, $1) RETURNING id::text`, fmt.Sprintf("/m/%s/%d", name, i))
		sp := fmt.Sprintf("sp-%s-%d", name, i)
		f.exec(`INSERT INTO playlist_items (playlist_id, spotify_track_id, title) VALUES ($1::uuid, $2, $2)`, id, sp)
		f.exec(`INSERT INTO spotify_track_matches (spotify_track_id, track_id, method) VALUES ($1, $2::uuid, 'exact')`, sp, track)
		if genre != "" {
			f.exec(`INSERT INTO track_station_genres (track_id, genre, score, is_primary) VALUES ($1::uuid, $2, 1, true)`, track, genre)
		}
	}
	return id
}

func (f *stationsFixture) call(handler http.HandlerFunc, user auth.User, method, target, id, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	req = req.WithContext(context.WithValue(auth.WithUser(req.Context(), user), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &v), rec.Body.String())
	return v
}

func TestStationsCRUDAndPermissions(t *testing.T) {
	f := newStationsFixture(t)
	admin, alice := f.user("admin", true), f.user("alice", false)
	rock := f.playlist("rock", 3, "techno")

	bad := map[string]string{
		"no name":          `{"playlist_ids":["` + rock + `"]}`,
		"no playlists":     `{"name":"x"}`,
		"unknown playlist": `{"name":"x","playlist_ids":["6f1b8c6e-6a43-4e4b-9b1e-2b3f8d1a0c55"]}`,
		"unknown genre":    `{"name":"x","playlist_ids":["` + rock + `"],"genres":["polka"]}`,
		"non-station":      `{"name":"x","playlist_ids":["` + rock + `"],"genres":["other"]}`,
	}
	for name, body := range bad {
		rec := f.call(f.h.CreateStation, admin, http.MethodPost, "/stations", "", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, name+": "+rec.Body.String())
	}

	rec := f.call(f.h.CreateStation, admin, http.MethodPost, "/stations", "", `{"name":" Techno ","playlist_ids":["`+rock+`"],"genres":["techno","techno"]}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	shared := decode[stations.Station](t, rec)
	require.True(t, shared.Shared)
	require.Equal(t, "Techno", shared.Name)
	require.Equal(t, []string{"techno"}, shared.Genres)
	require.Equal(t, 3, shared.TrackCount)

	// Everyone sees and plays the shared station; only admins change it.
	list := decode[map[string][]stations.Station](t, f.call(f.h.ListStations, alice, http.MethodGet, "/stations", "", ""))
	require.Len(t, list["stations"], 1)
	require.Equal(t, http.StatusOK, f.call(f.h.GetQueue, alice, http.MethodGet, "/q", shared.ID, "").Code)
	require.Equal(t, http.StatusForbidden, f.call(f.h.UpdateStation, alice, http.MethodPut, "/s", shared.ID, `{"name":"Mine"}`).Code)
	require.Equal(t, http.StatusForbidden, f.call(f.h.DeleteStation, alice, http.MethodDelete, "/s", shared.ID, "").Code)

	rec = f.call(f.h.UpdateStation, admin, http.MethodPut, "/s", shared.ID, `{"genres":[]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	updated := decode[stations.Station](t, rec)
	require.Equal(t, "Techno", updated.Name, "name kept")
	require.Empty(t, updated.Genres)

	// A playlist in use cannot be deleted.
	require.Equal(t, http.StatusConflict, f.call(f.h.DeletePlaylist, admin, http.MethodDelete, "/p", rock, "").Code)

	// Non-admins create personal stations that stay private.
	rec = f.call(f.h.CreateStation, alice, http.MethodPost, "/stations", "", `{"name":"Mine","playlist_ids":["`+rock+`"]}`)
	require.Equal(t, http.StatusCreated, rec.Code)
	mine := decode[stations.Station](t, rec)
	require.False(t, mine.Shared)
	require.Equal(t, http.StatusNotFound, f.call(f.h.DeleteStation, admin, http.MethodDelete, "/s", mine.ID, "").Code)
	require.Equal(t, http.StatusNoContent, f.call(f.h.DeleteStation, alice, http.MethodDelete, "/s", mine.ID, "").Code)
	require.Equal(t, http.StatusNoContent, f.call(f.h.DeleteStation, admin, http.MethodDelete, "/s", shared.ID, "").Code)
	require.Equal(t, http.StatusNoContent, f.call(f.h.DeletePlaylist, admin, http.MethodDelete, "/p", rock, "").Code)
}

func TestStationSuggestionsAndGenreCounts(t *testing.T) {
	f := newStationsFixture(t)
	admin := f.user("admin", true)
	techno := f.playlist("techno-list", 30, "techno")
	mixed := f.playlist("mixed-list", 10, "house")
	f.playlist("tiny", 3, "techno")

	type suggestions struct {
		Suggestions []stations.Suggestion `json:"suggestions"`
	}
	names := func() []string {
		s := decode[suggestions](t, f.call(f.h.ListStationSuggestions, admin, http.MethodGet, "/s", "", ""))
		var out []string
		for _, x := range s.Suggestions {
			out = append(out, fmt.Sprintf("%s/%s/%d", x.Name, x.Genre, x.TrackCount))
		}
		return out
	}
	// "techno-list" alone is 100% techno, so its genre suggestion would be
	// redundant; all playlists together (43 tracks) get Techno (33) and the
	// whole pool. house (10) and "tiny" (3) are too small.
	require.Equal(t, []string{"All playlists//43", "Techno/techno/33", "techno-list//30"}, names())

	rec := f.call(f.h.DismissStationSuggestion, admin, http.MethodPost, "/d", "",
		`{"playlist_ids":["`+techno+`"],"genre":""}`)
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, []string{"All playlists//43", "Techno/techno/33"}, names())

	// Accepting = creating the station; its suggestion disappears.
	all := f.one(`SELECT string_agg(id::text, '","') FROM playlists`)
	rec = f.call(f.h.CreateStation, admin, http.MethodPost, "/stations", "", `{"name":"Techno","playlist_ids":["`+all+`"],"genres":["techno"]}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Equal(t, []string{"All playlists//43"}, names())

	counts := decode[stations.PoolGenres](t, f.call(f.h.ListStationGenres, admin, http.MethodGet,
		"/stations/genres?playlist_ids="+techno+","+mixed, "", ""))
	require.Equal(t, 40, counts.Tracks)
	require.Equal(t, []stations.GenreCount{
		{Genre: stations.Genre{ID: "techno", Name: "Techno"}, Tracks: 30},
		{Genre: stations.Genre{ID: "house", Name: "House"}, Tracks: 10},
	}, counts.Genres)
	require.Equal(t, http.StatusBadRequest, f.call(f.h.ListStationGenres, admin, http.MethodGet, "/stations/genres", "", "").Code)
}

func TestBrowseTreeAndVoiceSearch(t *testing.T) {
	f := newStationsFixture(t)
	admin := f.user("admin", true)
	technoList := f.playlist("Techno Bunker", 3, "techno")
	houseList := f.playlist("House Party", 2, "house")
	f.exec(`UPDATE playlists SET name = spotify_id`)
	// Two tracks by one artist in the house list.
	artist := f.one(`INSERT INTO artists (name) VALUES ('Amelie Lens') RETURNING id::text`)
	f.exec(`UPDATE tracks SET artist_id = $1::uuid WHERE path LIKE '/m/House Party/%'`, artist)

	create := func(body string) stations.Station {
		rec := f.call(f.h.CreateStation, admin, http.MethodPost, "/stations", "", body)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		return decode[stations.Station](t, rec)
	}
	techno := create(`{"name":"Bunker Techno","playlist_ids":["` + technoList + `"],"genres":["techno"]}`)
	house := create(`{"name":"Party","playlist_ids":["` + houseList + `"],"genres":["house"]}`)
	mixed := create(`{"name":"Everything","playlist_ids":["` + technoList + `","` + houseList + `"]}`)

	browse := func(node string) (int, stations.BrowseNode) {
		rec := f.call(f.h.Browse, admin, http.MethodGet, "/browse?node="+node, "", "")
		if rec.Code != http.StatusOK {
			return rec.Code, stations.BrowseNode{}
		}
		return rec.Code, decode[stations.BrowseNode](t, rec)
	}
	ids := func(n stations.BrowseNode) []string {
		var out []string
		for _, it := range n.Items {
			out = append(out, it.ID)
		}
		return out
	}

	_, root := browse("")
	require.Equal(t, []string{"stations", "recent", "genres", "playlists"}, ids(root))

	_, all := browse("stations")
	require.Len(t, all.Items, 3)
	item := all.Items[0] // sorted by name: Bunker Techno
	require.Equal(t, "station:"+techno.ID, item.ID)
	require.True(t, item.Playable)
	require.Equal(t, "3 tracks · Techno", item.Subtitle)
	require.Equal(t, "/api/stations/"+techno.ID+"/cover", item.ArtworkURL)

	_, genres := browse("genres")
	require.Equal(t, []string{"genre:techno", "genre:house"}, ids(genres))
	_, technoNode := browse("genre:techno")
	require.Equal(t, "Techno", technoNode.Title)
	require.Equal(t, []string{"station:" + techno.ID}, ids(technoNode))

	_, lists := browse("playlists")
	require.Equal(t, []string{"playlist:" + houseList, "playlist:" + technoList}, ids(lists))
	_, inList := browse("playlist:" + houseList)
	require.ElementsMatch(t, []string{"station:" + house.ID, "station:" + mixed.ID}, ids(inList))

	// Recently played: most recent first.
	track := f.one(`SELECT id::text FROM tracks LIMIT 1`)
	f.exec(`INSERT INTO listening_history (user_id, track_id, station_id, played_at) VALUES
		($1::uuid, $2::uuid, $3::uuid, NOW() - INTERVAL '1 hour'), ($1::uuid, $2::uuid, $4::uuid, NOW())`,
		admin.ID, track, techno.ID, mixed.ID)
	_, recent := browse("recent")
	require.Equal(t, []string{"station:" + mixed.ID, "station:" + techno.ID}, ids(recent))

	for _, bad := range []string{"nope", "genre:polka", "playlist:not-a-uuid", "playlist:6f1b8c6e-6a43-4e4b-9b1e-2b3f8d1a0c55"} {
		code, _ := browse(bad)
		require.Equal(t, http.StatusNotFound, code, bad)
	}

	search := func(q string) []string {
		rec := f.call(f.h.SearchStations, admin, http.MethodGet, "/search/stations?q="+strings.ReplaceAll(q, " ", "+"), "", "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var out []string
		for _, h := range decode[struct {
			Results []stations.SearchHit `json:"results"`
		}](t, rec).Results {
			out = append(out, h.Reason+":"+h.Title)
		}
		return out
	}
	require.Equal(t, []string{"station:Everything"}, search("play everything on OwnWave"))
	// Name match beats genre match; "Everything" plays the Techno Bunker playlist.
	require.Equal(t, []string{"station:Bunker Techno", "playlist:Everything"}, search("techno radio"))
	require.Equal(t, []string{"genre:Party", "playlist:Everything"}, search("play house music"))
	require.Equal(t, []string{"genre:Party", "playlist:Everything"}, search("house party"))
	require.Equal(t, []string{"artist:Party", "artist:Everything"}, search("amelie lens"), "the station with the larger share first")
	require.Empty(t, search("polka"))
}

func TestCleanQuery(t *testing.T) {
	for in, want := range map[string]string{
		"play techno radio on OwnWave": "techno",
		"Play Hard Techno":             "Hard Techno",
		"listen to drum and bass mix":  "drum and bass",
		"house":                        "house",
	} {
		require.Equal(t, want, stations.CleanQuery(in), in)
	}
}
