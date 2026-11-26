//go:build integration

package main

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"ownwave/api/internal/stations"
)

// decidedSong adds a song the genre engine decided: genre "" is an unsure
// song (no station genre row), style "" none.
func (f *stationsFixture) decidedSong(path, genre, style string) string {
	f.t.Helper()
	id := f.one(`INSERT INTO tracks (path, title) VALUES ($1, $1) RETURNING id::text`, path)
	if genre == "" {
		f.exec(`INSERT INTO track_genre_decisions (track_id, source, unsure_reason, catalog_version)
			VALUES ($1::uuid, 'unsure', 'classifier_unsure', 'test')`, id)
		return id
	}
	f.exec(`INSERT INTO track_station_genres (track_id, genre, score, is_primary) VALUES ($1::uuid, $2, 1, true)`, id, genre)
	f.exec(`INSERT INTO track_genre_decisions (track_id, genre, style, source, confidence, catalog_version)
		VALUES ($1::uuid, $2, NULLIF($3, ''), 'strong_sources', 1, 'test')`, id, genre, style)
	return id
}

func (f *stationsFixture) decidedSongs(prefix string, n int, genre, style string) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = f.decidedSong(fmt.Sprintf("/m/%s/%d", prefix, i), genre, style)
	}
	return ids
}

func (f *stationsFixture) browse(node string) stations.BrowseNode {
	f.t.Helper()
	rec := f.call(f.h.Browse, f.user("browser-"+node, false), http.MethodGet, "/browse?node="+node, "", "")
	require.Equal(f.t, http.StatusOK, rec.Code, rec.Body.String())
	return decode[stations.BrowseNode](f.t, rec)
}

func titles(items []stations.BrowseItem) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it.Title)
	}
	return out
}

func TestStyleStations(t *testing.T) {
	f := newStationsFixture(t)
	admin, alice := f.user("admin", true), f.user("alice", false)
	other := f.one(`SELECT id FROM station_genres WHERE parent_id = 'house' AND id <> 'deep_house' ORDER BY position LIMIT 1`)
	otherName := f.one(`SELECT name FROM station_genres WHERE id = $1`, other)

	deep := f.decidedSongs("deep", 30, "house", "deep_house")
	f.decidedSongs("plain", 5, "house", "")     // a sure genre but no style: on House only
	f.decidedSongs("other", 22, "house", other) // a style below the threshold
	f.decidedSongs("unsure", 40, "", "")        // unsure songs join no station
	f.decidedSongs("techno", 10, "techno", "")

	changes := f.refresh()
	require.ElementsMatch(t, []stations.Change{
		{Genre: "house", Name: "House & Disco", Change: stations.ChangeCreated},
		{Genre: "deep_house", Name: "Deep House", Change: stations.ChangeCreated},
	}, changes)
	require.Equal(t, []string{"Deep House/genre/30", "House & Disco/genre/57"}, f.visible(alice))
	require.Empty(t, f.refresh(), "a second refresh changes nothing")

	// A style station knows its genre; a genre station has none.
	list := decode[map[string][]stations.Station](t, f.call(f.h.ListStations, alice, http.MethodGet, "/stations", "", ""))["stations"]
	parents := map[string]string{}
	for _, st := range list {
		parents[st.Name] = st.ParentGenre
		require.Equal(t, stations.KindGenre, st.Kind)
	}
	require.Equal(t, map[string]string{"Deep House": "house", "House & Disco": ""}, parents)

	// The genre page: the genre's station, then its style stations. The
	// genres folder lists the genre once, not its styles.
	require.Equal(t, []string{"House & Disco", "Deep House"}, titles(f.browse("genre:house").Items))
	require.Equal(t, []string{"Deep House"}, titles(f.browse("genre:deep_house").Items))
	genres := f.browse("genres").Items
	require.Equal(t, []string{"House & Disco"}, titles(genres))
	require.Equal(t, "2 stations", genres[0].Subtitle)

	// The admin view: genres in order, each followed by its styles with songs.
	settings := decode[stations.GenreStationSettings](t, f.call(f.h.AdminGenreStations, admin, http.MethodGet, "/", "", ""))
	require.Equal(t, stations.StyleStationShowAt, settings.StyleShowAt)
	require.Equal(t, stations.StyleStationHideBelow, settings.StyleHideBelow)
	byGenre := map[string]stations.GenreStation{}
	for _, g := range settings.Genres {
		byGenre[g.Genre] = g
	}
	require.Equal(t, stations.GenreStation{
		Genre: "deep_house", GenreName: "Deep House", Kind: stations.CatalogStyle, Parent: "house",
		StationID: byGenre["deep_house"].StationID, Name: "Deep House", Tracks: 30, State: stations.StateActive,
	}, byGenre["deep_house"])
	require.Equal(t, 22, byGenre[other].Tracks)
	require.Equal(t, stations.StateBelowThreshold, byGenre[other].State)
	require.Equal(t, 57, byGenre["house"].Tracks)
	require.Equal(t, stations.CatalogGenre, byGenre["house"].Kind)
	for _, g := range settings.Genres {
		if g.Kind == stations.CatalogStyle {
			require.True(t, g.Tracks > 0 || g.StationID != "", "styles without songs are left out: %s", g.Genre)
		}
	}
	require.Equal(t, "true", f.one(`SELECT (COUNT(*) > 100)::text FROM station_genres WHERE kind = 'style'`), "the catalog has styles")
	lastGenre := ""
	for _, g := range settings.Genres {
		if g.Kind == stations.CatalogGenre {
			lastGenre = g.Genre
		} else {
			require.Equal(t, lastGenre, g.Parent, "styles follow their genre: %s", g.Genre)
		}
	}

	// Three more songs of the other style reach the show threshold.
	f.decidedSongs("other-more", 3, "house", other)
	require.Equal(t, []stations.Change{{Genre: other, Name: otherName, Change: stations.ChangeCreated}}, f.refresh())

	// The style hides below its hide threshold; the genre keeps the songs.
	f.exec(`UPDATE track_genre_decisions SET style = NULL WHERE track_id = ANY($1::uuid[])`, deep[:11])
	require.Equal(t, []stations.Change{{Genre: "deep_house", Name: "Deep House", Change: stations.ChangeHidden}}, f.refresh())
	require.Contains(t, f.visible(alice), "House & Disco/genre/60")
	require.NotContains(t, f.visible(alice), "Deep House/genre/19")

	// A custom station may pick a style as its genre filter.
	pl := f.one(`INSERT INTO playlists (spotify_id, name) VALUES ('mix', 'mix') RETURNING id::text`)
	for i, track := range deep[10:14] { // one without a style, three with
		sp := fmt.Sprintf("sp-mix-%d", i)
		f.exec(`INSERT INTO playlist_items (playlist_id, spotify_track_id, title) VALUES ($1::uuid, $2, $2)`, pl, sp)
		f.exec(`INSERT INTO spotify_track_matches (spotify_track_id, track_id, method) VALUES ($1, $2::uuid, 'exact')`, sp, track)
	}
	rec := f.call(f.h.CreateStation, alice, http.MethodPost, "/stations", "", `{"name":"Deep","playlist_ids":["`+pl+`"],"genres":["deep_house"]}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Equal(t, 3, decode[stations.Station](t, rec).TrackCount)
}
