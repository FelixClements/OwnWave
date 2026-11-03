//go:build integration

package playback_test

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ownwave/api/internal/playback"
	"ownwave/api/internal/stations"
	"ownwave/api/internal/testdb"
)

type fixture struct {
	t    *testing.T
	pool *pgxpool.Pool
	ctx  context.Context
	user string
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{t: t, pool: testdb.NewPool(t), ctx: context.Background()}
	f.user = f.newUser()
	return f
}

func (f *fixture) scalar(query string, args ...any) string {
	f.t.Helper()
	var v string
	require.NoError(f.t, f.pool.QueryRow(f.ctx, query, args...).Scan(&v))
	return v
}

func (f *fixture) exec(query string, args ...any) {
	f.t.Helper()
	_, err := f.pool.Exec(f.ctx, query, args...)
	require.NoError(f.t, err)
}

func (f *fixture) newUser() string {
	return f.scalar(`INSERT INTO users (username, password_hash) VALUES ($1, 'x') RETURNING id::text`, uuid.NewString())
}

type track struct {
	title, artist, album string
	genres               []string // station genres; the first is primary
	noFeatures           bool
}

func (f *fixture) addTrack(tr track) string {
	f.t.Helper()
	if tr.artist == "" {
		tr.artist = "Artist " + uuid.NewString()[:8]
	}
	if tr.album == "" {
		tr.album = "Album"
	}
	artistID := f.scalar(`INSERT INTO artists (name) VALUES ($1)
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id::text`, tr.artist)
	albumID := f.scalar(`INSERT INTO albums (artist_id, title) VALUES ($1::uuid, $2)
		ON CONFLICT (artist_id, title) DO UPDATE SET title = EXCLUDED.title RETURNING id::text`, artistID, tr.album)
	id := f.scalar(`INSERT INTO tracks (path, artist_id, album_id, title, duration_seconds)
		VALUES ($1, $2::uuid, $3::uuid, $4, 180) RETURNING id::text`,
		fmt.Sprintf("/music/%s.flac", uuid.NewString()), artistID, albumID, tr.title)
	if !tr.noFeatures {
		f.exec(`INSERT INTO audio_features (track_id, bpm, key, energy, valence, loudness)
			VALUES ($1::uuid, 120, 'C', 0.5, 0.5, -10)`, id)
	}
	for i, g := range tr.genres {
		f.exec(`INSERT INTO track_station_genres (track_id, genre, score, is_primary) VALUES ($1::uuid, $2, 0.5, $3)`,
			id, g, i == 0)
	}
	return id
}

// playlist stores a playlist whose items map Spotify track ids to local
// track ids ("" = no local file).
func (f *fixture) playlist(name string, items map[string]string) string {
	id := f.scalar(`INSERT INTO playlists (spotify_id, name) VALUES ($1, $2) RETURNING id::text`, uuid.NewString(), name)
	for spotifyID, trackID := range items {
		f.exec(`INSERT INTO playlist_items (playlist_id, spotify_track_id, title) VALUES ($1::uuid, $2, $2)`, id, spotifyID)
		f.exec(`INSERT INTO spotify_track_matches (spotify_track_id, track_id, method) VALUES ($1, NULLIF($2, '')::uuid, 'exact')
			ON CONFLICT (spotify_track_id) DO NOTHING`, spotifyID, trackID)
	}
	return id
}

// playlistOf puts every given track in one new playlist.
func (f *fixture) playlistOf(trackIDs ...string) string {
	items := map[string]string{}
	for _, id := range trackIDs {
		items["sp-"+id] = id
	}
	return f.playlist(uuid.NewString(), items)
}

func (f *fixture) station(def stations.Definition) string {
	id := f.scalar(`INSERT INTO stations (user_id, name, genres) VALUES ($1::uuid, $2, $3) RETURNING id::text`,
		f.user, uuid.NewString(), append([]string{}, def.Genres...))
	for _, p := range def.PlaylistIDs {
		f.exec(`INSERT INTO station_playlists (station_id, playlist_id) VALUES ($1::uuid, $2::uuid)`, id, p)
	}
	return id
}

func (f *fixture) queueIDs(stationID string, def stations.Definition) []string {
	f.t.Helper()
	queue, err := playback.NewService(f.pool).BuildQueue(f.ctx, f.user, stationID, def, 24)
	require.NoError(f.t, err)
	ids := make([]string, len(queue))
	for i, q := range queue {
		ids[i] = q.ID
		assert.Equal(f.t, i, q.Position)
	}
	return ids
}

func sorted(ids ...string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}

func sortedOrNil(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	return sorted(ids...)
}

func TestBuildQueue_PlaylistPoolsAndGenres(t *testing.T) {
	f := newFixture(t)
	techno := f.addTrack(track{title: "Techno", genres: []string{"techno"}})
	technoHouse := f.addTrack(track{title: "Techno House", genres: []string{"house", "techno"}})
	house := f.addTrack(track{title: "House", genres: []string{"house"}})
	unscored := f.addTrack(track{title: "Unscored"})
	f.addTrack(track{title: "Not in a playlist", genres: []string{"techno"}})
	// Two playlists sharing a track; one item has no local file.
	a := f.playlist("a", map[string]string{"sp-1": techno, "sp-2": technoHouse, "sp-missing": ""})
	b := f.playlist("b", map[string]string{"sp-1": techno, "sp-3": house, "sp-4": unscored})

	cases := map[string]struct {
		def  stations.Definition
		want []string
	}{
		"one playlist":         {stations.Definition{PlaylistIDs: []string{a}}, sorted(techno, technoHouse)},
		"playlists union":      {stations.Definition{PlaylistIDs: []string{a, b}}, sorted(techno, technoHouse, house, unscored)},
		"genre incl secondary": {stations.Definition{PlaylistIDs: []string{a, b}, Genres: []string{"techno"}}, sorted(techno, technoHouse)},
		"several genres":       {stations.Definition{PlaylistIDs: []string{b}, Genres: []string{"house", "techno"}}, sorted(techno, house)},
		"genre not in pool":    {stations.Definition{PlaylistIDs: []string{a}, Genres: []string{"jazz"}}, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, sortedOrNil(f.queueIDs(f.station(tc.def), tc.def)))
		})
	}
}

func TestBuildQueue_ExcludesBansAndFlagsLikes(t *testing.T) {
	f := newFixture(t)
	liked := f.addTrack(track{title: "Liked"})
	banned := f.addTrack(track{title: "Banned"})
	f.exec(`INSERT INTO track_feedback (user_id, track_id, feedback) VALUES ($1::uuid, $2::uuid, 'like'), ($1::uuid, $3::uuid, 'ban')`,
		f.user, liked, banned)
	def := stations.Definition{PlaylistIDs: []string{f.playlistOf(liked, banned)}}

	queue, err := playback.NewService(f.pool).BuildQueue(f.ctx, f.user, f.station(def), def, 24)
	require.NoError(t, err)
	require.Len(t, queue, 1)
	assert.Equal(t, liked, queue[0].ID)
	assert.True(t, queue[0].Liked)
}

func TestBuildQueue_RecentlyPlayedGoLastOldestFirst(t *testing.T) {
	f := newFixture(t)
	fresh := f.addTrack(track{title: "Fresh"})
	playedLongAgo := f.addTrack(track{title: "Long ago"})
	playedHourAgo := f.addTrack(track{title: "Hour ago"})
	playedJustNow := f.addTrack(track{title: "Just now"})
	def := stations.Definition{PlaylistIDs: []string{f.playlistOf(fresh, playedLongAgo, playedHourAgo, playedJustNow)}}
	station := f.station(def)

	now := time.Now()
	for id, at := range map[string]time.Time{
		playedLongAgo: now.Add(-72 * time.Hour),
		playedHourAgo: now.Add(-time.Hour),
		playedJustNow: now.Add(-time.Minute),
	} {
		f.exec(`INSERT INTO listening_history (user_id, track_id, station_id, played_at) VALUES ($1::uuid, $2::uuid, $3::uuid, $4)`,
			f.user, id, station, at)
	}

	got := f.queueIDs(station, def)
	require.Len(t, got, 4)
	assert.ElementsMatch(t, []string{fresh, playedLongAgo}, got[:2], "outside the recent window counts as fresh")
	assert.Equal(t, []string{playedHourAgo, playedJustNow}, got[2:])
}

func TestBuildQueue_PagesAndDedupes(t *testing.T) {
	f := newFixture(t)
	var ids []string
	for i := 0; i < playback.QueuePageSize+10; i++ {
		ids = append(ids, f.addTrack(track{title: fmt.Sprintf("Track %d", i), artist: "Same"}))
	}
	// Same title+artist as an existing track: played once.
	ids = append(ids, f.addTrack(track{title: "Track 0", artist: "Same", album: "Deluxe"}))
	def := stations.Definition{PlaylistIDs: []string{f.playlistOf(ids...)}}
	assert.Len(t, f.queueIDs(f.station(def), def), playback.QueuePageSize)
}

func TestBuildQueue_SpacesOutArtists(t *testing.T) {
	f := newFixture(t)
	var ids []string
	artists := []string{"A", "A", "A", "B", "C", "D", "E", "F", "G", "H"}
	for _, artist := range artists {
		ids = append(ids, f.addTrack(track{title: uuid.NewString(), artist: artist}))
	}
	def := stations.Definition{PlaylistIDs: []string{f.playlistOf(ids...)}}
	station := f.station(def)
	artistOf := map[string]string{}
	for i, id := range ids {
		artistOf[id] = artists[i]
	}
	for run := 0; run < 20; run++ {
		got := f.queueIDs(station, def)
		last := -10
		for i, id := range got {
			if artistOf[id] == "A" {
				assert.Greater(t, i-last, 3, "artist A repeats within 3 tracks: %v", got)
				last = i
			}
		}
	}
}

func TestRecord_WritesHistoryAndChecksStationOwner(t *testing.T) {
	f := newFixture(t)
	trackID := f.addTrack(track{title: "History"})
	station := f.station(stations.Definition{PlaylistIDs: []string{f.playlistOf(trackID)}})
	svc := playback.NewService(f.pool)

	require.NoError(t, svc.Record(f.ctx, f.user, trackID, ""))
	require.NoError(t, svc.Record(f.ctx, f.user, trackID, station))
	assert.Equal(t, "2", f.scalar(`SELECT COUNT(*)::text FROM listening_history WHERE track_id = $1::uuid`, trackID))
	assert.Equal(t, "1", f.scalar(`SELECT COUNT(*)::text FROM listening_history WHERE station_id = $1::uuid`, station))

	other := f.newUser()
	require.Error(t, svc.Record(f.ctx, other, trackID, station))
	assert.Equal(t, "2", f.scalar(`SELECT COUNT(*)::text FROM listening_history WHERE track_id = $1::uuid`, trackID))

	shared := f.scalar(`INSERT INTO stations (name) VALUES ('Shared') RETURNING id::text`)
	require.NoError(t, svc.Record(f.ctx, other, trackID, shared))
}

func TestBuildQueue_IncludesUnanalysedTracks(t *testing.T) {
	f := newFixture(t)
	analysed := f.addTrack(track{title: "Analysed"})
	unanalysed := f.addTrack(track{title: "Not yet", noFeatures: true})
	def := stations.Definition{PlaylistIDs: []string{f.playlistOf(analysed, unanalysed)}}
	assert.Equal(t, sorted(analysed, unanalysed), sorted(f.queueIDs(f.station(def), def)...))
}
