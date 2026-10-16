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

const minConfidence = 0.3

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
	bpm, energy, valence float64
	genres               map[string]float64 // "main/sub" -> confidence
	cluster              *int
	vector               string // pgvector literal, e.g. "[1,0,...]"
}

func (f *fixture) addTrack(tr track) string {
	f.t.Helper()
	if tr.artist == "" {
		tr.artist = "Artist"
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

	var vector any
	if tr.vector != "" {
		vector = tr.vector
	}
	f.exec(`INSERT INTO audio_features (track_id, bpm, key, energy, valence, loudness, feature_vector)
		VALUES ($1::uuid, $2, 'C', $3, $4, -10, $5::vector)`, id, tr.bpm, tr.energy, tr.valence, vector)

	for genre, conf := range tr.genres {
		main, sub := splitGenre(genre)
		f.exec(`INSERT INTO track_genres (track_id, main_genre, sub_genre, confidence, source)
			VALUES ($1::uuid, $2, $3, $4, 'test')`, id, main, sub, conf)
	}
	if tr.cluster != nil {
		f.exec(`INSERT INTO track_clusters (track_id, cluster_id) VALUES ($1::uuid, $2)`, id, *tr.cluster)
	}
	return id
}

func splitGenre(g string) (string, string) {
	for i := range g {
		if g[i] == '/' {
			return g[:i], g[i+1:]
		}
	}
	return g, g
}

func (f *fixture) station(seed stations.Seed) string {
	raw, err := seed.JSON()
	require.NoError(f.t, err)
	return f.scalar(`INSERT INTO stations (user_id, name, seed_features) VALUES ($1::uuid, $2, $3::jsonb) RETURNING id::text`,
		f.user, uuid.NewString(), raw)
}

func (f *fixture) queueIDs(stationID string, seed stations.Seed) []string {
	f.t.Helper()
	svc := playback.NewService(f.pool, minConfidence)
	queue, err := svc.BuildQueue(f.ctx, f.user, stationID, seed, 24)
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

func vec(first float64) string {
	v := "[" + fmt.Sprint(first)
	for i := 1; i < 33; i++ {
		v += ",0.1"
	}
	return v + "]"
}

func ptr[T any](v T) *T { return &v }

func TestBuildQueue_SeedTypes(t *testing.T) {
	f := newFixture(t)
	cluster := 7
	rock := f.addTrack(track{title: "Rock", artist: "Rocker", album: "Loud", bpm: 140, energy: 0.9, valence: 0.6,
		genres: map[string]float64{"rock/punk": 0.9}, cluster: &cluster})
	rockWeak := f.addTrack(track{title: "Maybe Rock", bpm: 100, energy: 0.5, valence: 0.5,
		genres: map[string]float64{"rock/indie": 0.1}})
	jazz := f.addTrack(track{title: "Jazz", artist: "Cat", bpm: 90, energy: 0.2, valence: 0.3,
		genres: map[string]float64{"jazz/bebop": 0.8}})
	plain := f.addTrack(track{title: "Plain", artist: "Rocker", album: "Other", bpm: 120, energy: 0.6, valence: 0.9})

	artistID := f.scalar(`SELECT artist_id::text FROM tracks WHERE id = $1::uuid`, rock)
	albumID := f.scalar(`SELECT album_id::text FROM tracks WHERE id = $1::uuid`, rock)

	cases := map[string]struct {
		seed stations.Seed
		want []string
	}{
		"whole library":         {stations.Seed{}, sorted(rock, rockWeak, jazz, plain)},
		"genre uses confidence": {stations.Seed{Type: "genre", MainGenre: "rock"}, sorted(rock)},
		"sub genre":             {stations.Seed{Type: "sub_genre", MainGenre: "jazz", SubGenre: "bebop"}, sorted(jazz)},
		"uncategorized":         {stations.Seed{Type: "uncategorized"}, sorted(rockWeak, plain)},
		"artist":                {stations.Seed{Type: "artist", ArtistID: artistID}, sorted(rock, plain)},
		"album":                 {stations.Seed{Type: "album", AlbumID: albumID}, sorted(rock)},
		"cluster":               {stations.Seed{Type: "cluster", ClusterID: &cluster}, sorted(rock)},
		"mood range":            {stations.Seed{Type: "mood", MinEnergy: ptr(0.4), MaxValence: ptr(0.7)}, sorted(rock, rockWeak)},
		"bpm range with genre":  {stations.Seed{Type: "genre", MainGenre: "rock", MaxBPM: ptr(120.0)}, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := f.queueIDs(f.station(tc.seed), tc.seed)
			assert.Equal(t, tc.want, sortedOrNil(got))
		})
	}
}

func sortedOrNil(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	return sorted(ids...)
}

func TestBuildQueue_TrackSeedUsesNearestNeighbours(t *testing.T) {
	f := newFixture(t)
	seedTrack := f.addTrack(track{title: "Seed", vector: vec(1)})
	near := f.addTrack(track{title: "Near", vector: vec(0.9)})
	noVector := f.addTrack(track{title: "No vector"})

	seed := stations.Seed{Type: "track", TrackID: seedTrack}
	got := f.queueIDs(f.station(seed), seed)
	assert.Equal(t, sorted(seedTrack, near), sorted(got...))
	assert.NotContains(t, got, noVector)
}

func TestBuildQueue_ExcludesBansAndFlagsLikes(t *testing.T) {
	f := newFixture(t)
	liked := f.addTrack(track{title: "Liked"})
	banned := f.addTrack(track{title: "Banned"})
	f.exec(`INSERT INTO track_feedback (user_id, track_id, feedback) VALUES ($1::uuid, $2::uuid, 'like'), ($1::uuid, $3::uuid, 'ban')`,
		f.user, liked, banned)

	svc := playback.NewService(f.pool, minConfidence)
	station := f.station(stations.Seed{})
	queue, err := svc.BuildQueue(f.ctx, f.user, station, stations.Seed{}, 24)
	require.NoError(t, err)
	require.Len(t, queue, 1)
	assert.Equal(t, liked, queue[0].ID)
	assert.True(t, queue[0].Liked)

	// Another user's ban does not affect this user.
	other := f.newUser()
	f.exec(`INSERT INTO track_feedback (user_id, track_id, feedback) VALUES ($1::uuid, $2::uuid, 'ban')`, other, liked)
	n, err := svc.PoolSize(f.ctx, f.user, stations.Seed{})
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}

func TestBuildQueue_RecentlyPlayedGoLastOldestFirst(t *testing.T) {
	f := newFixture(t)
	fresh := f.addTrack(track{title: "Fresh"})
	playedLongAgo := f.addTrack(track{title: "Long ago"})
	playedHourAgo := f.addTrack(track{title: "Hour ago"})
	playedJustNow := f.addTrack(track{title: "Just now"})
	station := f.station(stations.Seed{})

	now := time.Now()
	for id, at := range map[string]time.Time{
		playedLongAgo: now.Add(-72 * time.Hour),
		playedHourAgo: now.Add(-time.Hour),
		playedJustNow: now.Add(-time.Minute),
	} {
		f.exec(`INSERT INTO listening_history (user_id, track_id, station_id, played_at) VALUES ($1::uuid, $2::uuid, $3::uuid, $4)`,
			f.user, id, station, at)
	}

	got := f.queueIDs(station, stations.Seed{})
	require.Len(t, got, 4)
	assert.ElementsMatch(t, []string{fresh, playedLongAgo}, got[:2], "outside the recent window counts as fresh")
	assert.Equal(t, []string{playedHourAgo, playedJustNow}, got[2:])
}

func TestBuildQueue_PagesAndDedupes(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < playback.QueuePageSize+10; i++ {
		f.addTrack(track{title: fmt.Sprintf("Track %d", i)})
	}
	f.addTrack(track{title: "Track 0", album: "Deluxe"}) // same title+artist as an existing track

	station := f.station(stations.Seed{})
	assert.Len(t, f.queueIDs(station, stations.Seed{}), playback.QueuePageSize)

	n, err := playback.NewService(f.pool, minConfidence).PoolSize(f.ctx, f.user, stations.Seed{})
	require.NoError(t, err)
	assert.Equal(t, playback.QueuePageSize+11, n)
}

func TestRecord_WritesHistoryAndChecksStationOwner(t *testing.T) {
	f := newFixture(t)
	trackID := f.addTrack(track{title: "History"})
	station := f.station(stations.Seed{})
	svc := playback.NewService(f.pool, minConfidence)

	require.NoError(t, svc.Record(f.ctx, f.user, trackID, ""))
	require.NoError(t, svc.Record(f.ctx, f.user, trackID, station))
	assert.Equal(t, "2", f.scalar(`SELECT COUNT(*)::text FROM listening_history WHERE track_id = $1::uuid`, trackID))
	assert.Equal(t, "1", f.scalar(`SELECT COUNT(*)::text FROM listening_history WHERE station_id = $1::uuid`, station))

	other := f.newUser()
	require.Error(t, svc.Record(f.ctx, other, trackID, station))
	assert.Equal(t, "2", f.scalar(`SELECT COUNT(*)::text FROM listening_history WHERE track_id = $1::uuid`, trackID))
}
