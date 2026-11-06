//go:build integration

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"ownwave/api/internal/analytics"
	"ownwave/api/internal/auth"
	"ownwave/api/internal/pipeline"
	"ownwave/api/internal/playlists"
	"ownwave/api/internal/spotify"
	"ownwave/api/internal/stations"
)

// fakeSpotify serves the playlists a test registers by Spotify id.
type fakeSpotify struct{ lists map[string]spotify.Playlist }

func (f *fakeSpotify) FetchPlaylist(_ context.Context, id string) (spotify.Playlist, error) {
	p, ok := f.lists[id]
	if !ok {
		return spotify.Playlist{}, &spotify.FetchError{Err: errors.New("spotify answered 404")}
	}
	p.ID = id
	return p, nil
}

func (f *fakeSpotify) FetchImage(context.Context, string) ([]byte, string, error) {
	return nil, "", errors.New("no images")
}

// withSpotify makes the handler sync playlists from fake.
func (f *stationsFixture) withSpotify(fake *fakeSpotify) {
	f.h.playlists = playlists.NewService(f.h.db.pool, fake)
	f.h.pipeline = pipeline.New(f.h.db.pool, f.h.playlists, f.h.stations)
}

func (f *stationsFixture) refresh() []stations.Change {
	f.t.Helper()
	run, err := f.h.pipeline.Run(f.ctx, "test", true)
	require.NoError(f.t, err)
	return run.Changes
}

// visible lists the stations a user sees, as "name/kind/tracks".
func (f *stationsFixture) visible(user auth.User) []string {
	f.t.Helper()
	list := decode[map[string][]stations.Station](f.t, f.call(f.h.ListStations, user, http.MethodGet, "/stations", "", ""))
	out := []string{}
	for _, st := range list["stations"] {
		out = append(out, fmt.Sprintf("%s/%s/%d", st.Name, st.Kind, st.TrackCount))
	}
	return out
}

func (f *stationsFixture) callGenre(user auth.User, genre, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, "/admin/genre-stations/"+genre, strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("genre", genre)
	req = req.WithContext(context.WithValue(auth.WithUser(req.Context(), user), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	f.h.AdminUpdateGenreStation(rec, req)
	return rec
}

func TestGenreStationsFollowTheLibrary(t *testing.T) {
	f := newStationsFixture(t)
	admin, alice := f.user("admin", true), f.user("alice", false)
	techno := f.playlist("techno-list", 30, "techno")
	f.playlist("house-list", 10, "house")

	// Only Techno has enough tracks.
	require.Equal(t, []stations.Change{{Genre: "techno", Name: "Techno", Change: stations.ChangeCreated}}, f.refresh())
	require.Equal(t, []string{"Techno/genre/30"}, f.visible(alice))
	require.Empty(t, f.refresh(), "a second refresh changes nothing")
	require.Equal(t, "1", f.one(`SELECT COUNT(*)::text FROM stations`))
	technoID := f.one(`SELECT id::text FROM stations WHERE auto_genre = 'techno'`)

	// More house tracks arrive in another playlist: House appears, Techno
	// stays one station and plays the new playlist's techno tracks too.
	more := f.playlist("more", 15, "house")
	f.addTracks(more, "more-techno", 5, "techno")
	require.Equal(t, []stations.Change{{Genre: "house", Name: "House", Change: stations.ChangeCreated}}, f.refresh())
	require.Equal(t, []string{"House/genre/25", "Techno/genre/35"}, f.visible(alice))
	require.Equal(t, technoID, f.one(`SELECT id::text FROM stations WHERE auto_genre = 'techno'`))

	// Below the show threshold but not below the hide one: still shown.
	f.exec(`DELETE FROM track_station_genres WHERE genre = 'house' AND track_id IN (
		SELECT id FROM tracks WHERE path LIKE '/m/more/%' ORDER BY path LIMIT 3)`)
	require.Empty(t, f.refresh())
	require.Equal(t, []string{"House/genre/22", "Techno/genre/35"}, f.visible(alice))

	// Below the hide threshold: hidden, not deleted, history kept.
	houseID := f.one(`SELECT id::text FROM stations WHERE auto_genre = 'house'`)
	track := f.one(`SELECT id::text FROM tracks LIMIT 1`)
	f.exec(`INSERT INTO listening_history (user_id, track_id, station_id) VALUES ($1::uuid, $2::uuid, $3::uuid)`, alice.ID, track, houseID)
	f.exec(`DELETE FROM track_station_genres WHERE genre = 'house' AND track_id IN (
		SELECT id FROM tracks WHERE path LIKE '/m/more/%' ORDER BY path LIMIT 3 OFFSET 3)`)
	require.Equal(t, []stations.Change{{Genre: "house", Name: "House", Change: stations.ChangeHidden}}, f.refresh())
	require.Equal(t, []string{"Techno/genre/35"}, f.visible(alice))
	require.Equal(t, http.StatusNotFound, f.call(f.h.GetStation, alice, http.MethodGet, "/s", houseID, "").Code)
	require.Equal(t, houseID, f.one(`SELECT station_id::text FROM listening_history`))

	// 22 tracks again: hidden stations need the show threshold to return.
	f.addTracks(more, "back", 3, "house")
	require.Empty(t, f.refresh())
	f.addTracks(more, "back-more", 3, "house")
	require.Equal(t, []stations.Change{{Genre: "house", Name: "House", Change: stations.ChangeShown}}, f.refresh())
	require.Equal(t, houseID, f.one(`SELECT id::text FROM stations WHERE auto_genre = 'house'`), "same station back")

	// A genre station plays the library: a deleted playlist's songs stay.
	require.Equal(t, http.StatusNoContent, f.call(f.h.DeletePlaylist, admin, http.MethodDelete, "/p", techno, "").Code)
	f.refresh()
	require.Equal(t, []string{"House/genre/25", "Techno/genre/35"}, f.visible(alice))
}

func TestGenreStationsPlayTheWholeLibraryByMainGenre(t *testing.T) {
	f := newStationsFixture(t)
	alice := f.user("alice", false)

	// No playlist at all: 25 songs of a main genre are enough for a station.
	song := func(title, artist, genre string, duration float64) string {
		a := f.one(`INSERT INTO artists (name) VALUES ($1)
			ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id::text`, artist)
		id := f.one(`INSERT INTO tracks (path, title, artist_id, duration_seconds) VALUES ($1, $2, $3::uuid, $4) RETURNING id::text`,
			"/m/"+artist+"/"+title+fmt.Sprint(duration), title, a, duration)
		f.exec(`INSERT INTO track_station_genres (track_id, genre, score, is_primary) VALUES ($1::uuid, $2, 1, true)`, id, genre)
		return id
	}
	for i := 0; i < 25; i++ {
		song(fmt.Sprintf("Song %d", i), "Oxia", "techno", 200)
	}
	f.refresh()
	require.Equal(t, []string{"Techno/genre/25"}, f.visible(alice))
	stationID := f.one(`SELECT id::text FROM stations WHERE auto_genre = 'techno'`)

	// A second genre does not put a song on that genre's station.
	second := song("Crossover", "Oxia", "house", 200)
	f.exec(`INSERT INTO track_station_genres (track_id, genre, score, is_primary) VALUES ($1::uuid, 'techno', 0.8, false)`, second)
	f.refresh()
	require.Equal(t, []string{"Techno/genre/25"}, f.visible(alice))

	// Copies count as one song, whichever copy carries the main genre.
	song("Domino", "Oxia", "techno", 300)
	f.refresh()
	require.Equal(t, []string{"Techno/genre/26"}, f.visible(alice))
	copyID := song("Domino (2011 Remaster)", "Oxia", "techno", 301)
	f.refresh()
	require.Equal(t, []string{"Techno/genre/26"}, f.visible(alice))
	require.Equal(t, "1", f.one(`SELECT COUNT(*)::text FROM tracks WHERE copy_of IS NOT NULL`))
	require.Equal(t, "2", f.one(`SELECT COUNT(*)::text FROM tracks WHERE title LIKE 'Domino%'`), "both files stay")

	// A hide of one copy keeps the whole song off.
	rec := f.call(f.h.HideTrack, alice, http.MethodPost, "/h", stationID, `{"track_id":"`+copyID+`"}`)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	require.Equal(t, []string{"Techno/genre/25"}, f.visible(alice))
	hidden := decode[struct {
		Tracks []stations.HiddenTrack `json:"tracks"`
	}](t, f.call(f.h.ListHiddenTracks, alice, http.MethodGet, "/h", stationID, ""))
	require.Len(t, hidden.Tracks, 1)
	require.Contains(t, hidden.Tracks[0].Title, "Domino")
	// Another user still hears it.
	require.Equal(t, []string{"Techno/genre/26"}, f.visible(f.user("bob", false)))

	// It comes back through either copy.
	unhide := httptest.NewRequest(http.MethodDelete, "/h", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", stationID)
	rctx.URLParams.Add("trackId", copyID)
	w := httptest.NewRecorder()
	f.h.UnhideTrack(w, unhide.WithContext(context.WithValue(auth.WithUser(unhide.Context(), alice), chi.RouteCtxKey, rctx)))
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	require.Equal(t, []string{"Techno/genre/26"}, f.visible(alice))

	// Hiding an unknown song is refused, as is a bad id.
	require.Equal(t, http.StatusNotFound, f.call(f.h.HideTrack, alice, http.MethodPost, "/h", stationID, `{"track_id":"`+stationID+`"}`).Code)
	require.Equal(t, http.StatusBadRequest, f.call(f.h.HideTrack, alice, http.MethodPost, "/h", stationID, `{"track_id":"nope"}`).Code)
}

func TestGenreReviewAndCorrections(t *testing.T) {
	f := newStationsFixture(t)
	admin, alice := f.user("admin", true), f.user("alice", false)
	f.playlist("list", 30, "techno")
	f.refresh()
	stationID := f.one(`SELECT id::text FROM stations WHERE auto_genre = 'techno'`)
	hiddenTrack := f.one(`SELECT id::text FROM tracks ORDER BY path LIMIT 1`)
	low := f.one(`SELECT id::text FROM tracks ORDER BY path LIMIT 1 OFFSET 1`)
	f.exec(`UPDATE track_station_genres SET score = 0.2 WHERE track_id = $1::uuid`, low)
	rec := f.call(f.h.HideTrack, alice, http.MethodPost, "/h", stationID, `{"track_id":"`+hiddenTrack+`"}`)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	review := func() []stations.GenreReviewItem {
		rec := f.call(f.h.AdminGenreReview, admin, http.MethodGet, "/r", "", "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decode[struct {
			Items []stations.GenreReviewItem `json:"items"`
		}](t, rec).Items
	}
	items := review()
	require.Len(t, items, 30)
	require.Equal(t, hiddenTrack, items[0].TrackID, "most hidden first")
	require.Equal(t, 1, items[0].HiddenBy)
	require.Equal(t, low, items[1].TrackID, "then the lowest score")
	require.Equal(t, http.StatusBadRequest, f.call(f.h.AdminGenreReview, admin, http.MethodGet, "/r?limit=999", "", "").Code)

	// Bad requests save nothing.
	save := func(body string) int {
		return f.call(f.h.AdminSaveGenreCorrection, admin, http.MethodPost, "/c", "", body).Code
	}
	require.Equal(t, http.StatusBadRequest, save(`{"genre":"techno"}`))
	require.Equal(t, http.StatusBadRequest, save(`{"track_id":"`+low+`","artist":"X","genre":"techno"}`))
	require.Equal(t, http.StatusBadRequest, save(`{"track_id":"`+low+`","genre":"nope"}`))
	require.Equal(t, http.StatusBadRequest, save(`{"track_id":"nope","genre":"techno"}`))
	require.Equal(t, http.StatusBadRequest, save(`{"track_id":"`+stationID+`","genre":"techno"}`), "unknown track")
	require.Equal(t, "0", f.one(`SELECT COUNT(*)::text FROM genre_corrections`))

	// Saving stores the correction and queues scoring; saving again replaces it.
	stub := analytics.NewStub()
	f.h.analytics = stub
	listed := func(rec *httptest.ResponseRecorder) []stations.GenreCorrection {
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decode[struct {
			Corrections []stations.GenreCorrection `json:"corrections"`
		}](t, rec).Corrections
	}
	song := `{"track_id":"` + low + `","genre":"house"}`
	require.Equal(t, http.StatusOK, save(song))
	got := listed(f.call(f.h.AdminSaveGenreCorrection, admin, http.MethodPost, "/c", "", `{"track_id":"`+low+`","genre":"techno"}`))
	require.Len(t, got, 1)
	require.Equal(t, "techno", got[0].Genre)
	require.Equal(t, low, *got[0].TrackID)
	require.Len(t, listed(f.call(f.h.AdminSaveGenreCorrection, admin, http.MethodPost, "/c", "", `{"artist":"Oxia","genre":"house"}`)), 2)
	require.Len(t, listed(f.call(f.h.AdminSaveGenreCorrection, admin, http.MethodPost, "/c", "", `{"artist":"OXIA ","genre":"techno"}`)), 2, "artists ignore case")

	// A song scored from a correction leaves the review list.
	f.exec(`UPDATE track_station_genres SET corrected = TRUE WHERE track_id = $1::uuid`, low)
	require.Len(t, review(), 29)

	// When scoring cannot be queued the admin hears it; the change is saved.
	stub.Errors["RebuildGenres"] = errors.New("worker down")
	require.Equal(t, http.StatusInternalServerError, f.call(f.h.AdminSaveGenreCorrection, admin, http.MethodPost, "/c", "", song).Code)
	require.Equal(t, "house", f.one(`SELECT genre FROM genre_corrections WHERE track_id = $1::uuid`, low))
	delete(stub.Errors, "RebuildGenres")

	require.Len(t, listed(f.call(f.h.AdminDeleteGenreCorrection, admin, http.MethodDelete, "/c", got[0].ID, "")), 1)
}

func TestAddingAPlaylistCreatesNoDuplicateStation(t *testing.T) {
	f := newStationsFixture(t)
	admin := f.user("admin", true)
	fake := &fakeSpotify{lists: map[string]spotify.Playlist{}}
	f.withSpotify(fake)

	// Local files: 60 techno tracks by one artist.
	artist := f.one(`INSERT INTO artists (name) VALUES ('Oxia') RETURNING id::text`)
	var first, second []spotify.Track
	for i := 0; i < 60; i++ {
		title := fmt.Sprintf("Domino %d", i)
		id := f.one(`INSERT INTO tracks (path, artist_id, title, duration_seconds) VALUES ($1, $2::uuid, $1, 200) RETURNING id::text`, title, artist)
		f.exec(`INSERT INTO track_station_genres (track_id, genre, score, is_primary) VALUES ($1::uuid, 'techno', 1, true)`, id)
		tr := spotify.Track{ID: fmt.Sprintf("sp%d", i), Title: title, Artists: []string{"Oxia"}, DurationMS: 200000}
		if i < 30 {
			first = append(first, tr)
		} else {
			second = append(second, tr)
		}
	}
	fake.lists["37i9dQZF1DX6J5NfMJS675"] = spotify.Playlist{Name: "First", Tracks: first}
	fake.lists["37i9dQZF1DX0XUsuxWHRQd"] = spotify.Playlist{Name: "Second", Tracks: second}

	add := func(spotifyID string) {
		rec := f.call(f.h.AddPlaylist, admin, http.MethodPost, "/admin/playlists", "",
			`{"url":"https://open.spotify.com/playlist/`+spotifyID+`"}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	add("37i9dQZF1DX6J5NfMJS675")
	require.Equal(t, []string{"Techno/genre/60"}, f.visible(admin), "the whole library")
	add("37i9dQZF1DX0XUsuxWHRQd")
	require.Equal(t, []string{"Techno/genre/60"}, f.visible(admin), "one station")
	require.Equal(t, "1", f.one(`SELECT COUNT(*)::text FROM stations`))

	// Syncing and re-adding change nothing either.
	secondID := f.one(`SELECT id::text FROM playlists WHERE spotify_id = '37i9dQZF1DX0XUsuxWHRQd'`)
	require.Equal(t, http.StatusOK, f.call(f.h.SyncPlaylist, admin, http.MethodPost, "/p", secondID, "").Code)
	add("37i9dQZF1DX0XUsuxWHRQd")
	require.Equal(t, []string{"Techno/genre/60"}, f.visible(admin))
	require.Equal(t, "1", f.one(`SELECT COUNT(*)::text FROM stations`))

	// The unique index is the last guard, whoever inserts.
	_, err := f.h.db.pool.Exec(f.ctx, `
		INSERT INTO stations (user_id, name, genres, auto_genre, auto_active)
		VALUES (NULL, 'Techno 2', ARRAY['techno'], 'techno', TRUE)`)
	require.Error(t, err)
}

func TestGenreStationAdminControls(t *testing.T) {
	f := newStationsFixture(t)
	admin, alice := f.user("admin", true), f.user("alice", false)
	f.playlist("mix", 30, "techno")
	f.addTracks(f.one(`SELECT id::text FROM playlists`), "mix-house", 30, "house")
	f.refresh()
	technoID := f.one(`SELECT id::text FROM stations WHERE auto_genre = 'techno'`)

	settings := func() stations.GenreStationSettings {
		rec := f.call(f.h.AdminGenreStations, admin, http.MethodGet, "/g", "", "")
		require.Equal(t, http.StatusOK, rec.Code)
		return decode[stations.GenreStationSettings](t, rec)
	}
	state := func(g string) string {
		for _, x := range settings().Genres {
			if x.Genre == g {
				return x.State + "/" + x.Name
			}
		}
		return ""
	}
	update := func(genre, body string) int {
		rec := f.callGenre(admin, genre, body)
		return rec.Code
	}
	require.Equal(t, "active/Techno", state("techno"))
	require.Equal(t, 25, settings().ShowAt)
	require.Equal(t, 20, settings().HideBelow)

	// A rename survives refreshes; "" goes back to the genre's name.
	require.Equal(t, http.StatusOK, update("techno", `{"name":"Warehouse"}`))
	f.refresh()
	require.Equal(t, "active/Warehouse", state("techno"))
	require.Equal(t, http.StatusOK, update("techno", `{"name":""}`))
	require.Equal(t, "active/Techno", state("techno"))
	// Renaming through the station itself works the same way.
	rec := f.call(f.h.UpdateStation, admin, http.MethodPut, "/s", technoID, `{"name":"Bunker"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	f.refresh()
	require.Equal(t, "active/Bunker", state("techno"))

	// Genre stations only change their name, and are not deleted.
	require.Equal(t, http.StatusBadRequest, f.call(f.h.UpdateStation, admin, http.MethodPut, "/s", technoID, `{"genres":["house"]}`).Code)
	require.Equal(t, http.StatusConflict, f.call(f.h.DeleteStation, admin, http.MethodDelete, "/s", technoID, "").Code)
	require.Equal(t, http.StatusForbidden, f.call(f.h.UpdateStation, alice, http.MethodPut, "/s", technoID, `{"name":"Mine"}`).Code)

	// A custom station can play one genre of every playlist: the genre station
	// plays that genre of the whole library, so they are different stations.
	rec = f.call(f.h.CreateStation, alice, http.MethodPost, "/stations", "", `{"name":"My techno","all_playlists":true,"genres":["techno"]}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Equal(t, http.StatusNoContent, f.call(f.h.DeleteStation, alice, http.MethodDelete, "/s", decode[stations.Station](t, rec).ID, "").Code)
	rec = f.call(f.h.CreateStation, alice, http.MethodPost, "/stations", "", `{"name":"Both","all_playlists":true,"genres":["techno","house"]}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	both := decode[stations.Station](t, rec)
	require.True(t, both.AllPlaylists)
	require.Equal(t, stations.KindCustom, both.Kind)
	require.Equal(t, 60, both.TrackCount)

	// Excluding hides the station and keeps it hidden.
	require.Equal(t, http.StatusOK, update("techno", `{"excluded":true}`))
	require.Equal(t, "excluded/Bunker", state("techno"))
	require.Equal(t, []string{"House/genre/30", "Both/custom/60"}, f.visible(alice))
	f.refresh()
	require.Equal(t, []string{"House/genre/30", "Both/custom/60"}, f.visible(alice))
	require.Equal(t, http.StatusOK, update("techno", `{"excluded":false}`))
	require.Equal(t, "active/Bunker", state("techno"))

	// A genre that never had a station can be excluded ahead of time.
	require.Equal(t, http.StatusNotFound, update("other", `{"excluded":true}`), "not a station genre")
	f.exec(`INSERT INTO station_genres (id, name, position, is_station) VALUES ('trance', 'Trance', 3, true)`)
	require.Equal(t, "below_threshold/Trance", state("trance"))
	require.Equal(t, http.StatusOK, update("trance", `{"excluded":true}`))
	f.playlist("trance-list", 30, "trance")
	f.refresh()
	require.Equal(t, "excluded/Trance", state("trance"))

	// Off hides every genre station and deletes nothing; on brings them back.
	off := func(enabled string) {
		rec := f.call(f.h.AdminSetGenreStations, admin, http.MethodPut, "/g", "", `{"enabled":`+enabled+`}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	off("false")
	require.Equal(t, "off/Bunker", state("techno"))
	require.Equal(t, []string{"Both/custom/60"}, f.visible(alice))
	f.refresh()
	require.Equal(t, []string{"Both/custom/60"}, f.visible(alice))
	off("true")
	require.Equal(t, []string{"Bunker/genre/30", "House/genre/30", "Both/custom/60"}, f.visible(alice))
	require.Equal(t, technoID, f.one(`SELECT id::text FROM stations WHERE auto_genre = 'techno'`))
	require.Equal(t, http.StatusBadRequest, f.call(f.h.AdminSetGenreStations, admin, http.MethodPut, "/g", "", `{}`).Code)
}

// The whole chain without manual steps: a new file is scanned and scored
// (simulated here: the analytics worker's writes plus its NOTIFY), the API
// matches it to its playlist item and it plays on its genre station.
func TestNewFileReachesItsGenreStation(t *testing.T) {
	f := newStationsFixture(t)
	admin := f.user("admin", true)
	list := f.playlist("mix", 30, "techno")
	// A playlist track with no local file yet.
	f.exec(`INSERT INTO playlist_items (playlist_id, spotify_track_id, title, artists, duration_ms)
		VALUES ($1::uuid, 'sp-new', 'Drift Away', ARRAY['Amelie Lens'], 200000)`, list)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()
	f.h.pipeline.Start(ctx, f.h.db.pool.Config().ConnString())
	eventually := func(what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for !cond() {
			require.True(t, time.Now().Before(deadline), what)
			time.Sleep(50 * time.Millisecond)
		}
	}
	// The startup run creates the Techno station.
	eventually("startup refresh", func() bool { return strings.Join(f.visible(admin), ",") == "Techno/genre/30" })

	// The worker imports the file and scores its genres, then notifies.
	artist := f.one(`INSERT INTO artists (name) VALUES ('Amelie Lens') RETURNING id::text`)
	track := f.one(`INSERT INTO tracks (path, artist_id, title, duration_seconds)
		VALUES ('/m/new.flac', $1::uuid, 'Drift Away', 200) RETURNING id::text`, artist)
	f.exec(`INSERT INTO track_station_genres (track_id, genre, score, is_primary) VALUES ($1::uuid, 'techno', 1, true)`, track)
	f.exec(`SELECT pg_notify('ownwave_pipeline', 'genres_scored')`)

	require.Equal(t, []string{"Techno/genre/31"}, f.visible(admin), "a genre station plays it as soon as it is scored")
	var status pipeline.Status
	eventually("run after the notify", func() bool {
		status = decode[pipeline.Status](t, f.call(f.h.AdminPipeline, admin, http.MethodGet, "/p", "", ""))
		return len(status.Runs) > 0 && status.Runs[0].Trigger == "genres_scored"
	})
	require.Equal(t, track, f.one(`SELECT track_id::text FROM spotify_track_matches WHERE spotify_track_id = 'sp-new'`))
	last := status.Runs[0]
	require.Equal(t, "genres_scored", last.Trigger)
	require.NotNil(t, last.Matched)
	require.Equal(t, 1, *last.Matched)
}

func TestPipelineRunLog(t *testing.T) {
	f := newStationsFixture(t)
	admin := f.user("admin", true)
	f.playlist("mix", 30, "techno")
	f.exec(`INSERT INTO app_state (key, value) VALUES ('genres_scored', '{"tracks": 30, "scored": 29}')`)

	rec := f.call(f.h.AdminRefreshPipeline, admin, http.MethodPost, "/p", "", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	created := decode[pipeline.Run](t, rec)
	require.Equal(t, []stations.Change{{Genre: "techno", Name: "Techno", Change: stations.ChangeCreated}}, created.Changes)
	for i := 0; i < 3; i++ {
		f.refresh()
	}

	status := decode[pipeline.Status](t, f.call(f.h.AdminPipeline, admin, http.MethodGet, "/p", "", ""))
	// Runs that changed nothing are kept only while newest.
	require.Len(t, status.Runs, 2)
	require.Empty(t, status.Runs[0].Changes)
	require.Equal(t, created.ID, status.Runs[1].ID)
	require.Nil(t, status.LastScan)
	require.NotNil(t, status.GenresScored)
	require.Equal(t, 29, status.GenresScored.Scored)
}
