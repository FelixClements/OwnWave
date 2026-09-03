//go:build integration

package playback_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ownwave/api/internal/playback"
)

type trackSeed struct {
	path     string
	title    string
	artist   string
	album    string
	position int
	playedAt *time.Time
	feedback string
	trackNum int
}

func seedArtist(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO artists (name) VALUES ($1)
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id::text
	`, name).Scan(&id)
	require.NoError(t, err)
	return id
}

func seedAlbum(t *testing.T, pool *pgxpool.Pool, artistID, title string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO albums (artist_id, title) VALUES ($1::uuid, $2)
		ON CONFLICT (artist_id, title) DO UPDATE SET title = EXCLUDED.title
		RETURNING id::text
	`, artistID, title).Scan(&id)
	require.NoError(t, err)
	return id
}

func seedTrackGraph(t *testing.T, pool *pgxpool.Pool, userID, stationID string, seed trackSeed) string {
	t.Helper()
	ctx := context.Background()

	artistID := seedArtist(t, pool, seed.artist)
	albumID := seedAlbum(t, pool, artistID, seed.album)

	var trackID string
	err := pool.QueryRow(ctx, `
		INSERT INTO tracks (path, artist_id, album_id, title, track_number, duration_seconds, sample_rate, channels)
		VALUES ($1, $2::uuid, $3::uuid, $4, $5, 180.0, 44100, 2)
		RETURNING id::text
	`, seed.path, artistID, albumID, seed.title, seed.trackNum).Scan(&trackID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO audio_features (track_id, bpm, key, energy, valence, loudness)
		VALUES ($1::uuid, 120.0, 'C', 0.5, 0.5, -10.0)
	`, trackID)
	require.NoError(t, err)

	if seed.feedback != "" {
		_, err = pool.Exec(ctx, `
			INSERT INTO track_feedback (user_id, track_id, feedback) VALUES ($1::uuid, $2::uuid, $3)
		`, userID, trackID, seed.feedback)
		require.NoError(t, err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO station_tracks (station_id, track_id, position, played_at)
		VALUES ($1::uuid, $2::uuid, $3, $4)
	`, stationID, trackID, seed.position, seed.playedAt)
	require.NoError(t, err)

	return trackID
}

func seedUser(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO users (username, password_hash, is_admin)
		VALUES ($1, 'x', false)
		RETURNING id::text
	`, uuid.NewString()).Scan(&id)
	require.NoError(t, err)
	return id
}

func seedStation(t *testing.T, pool *pgxpool.Pool, userID, name string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO stations (name, user_id) VALUES ($1, $2::uuid) RETURNING id::text
	`, name, userID).Scan(&id)
	require.NoError(t, err)
	return id
}

func TestRecord_WritesListeningHistory(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	userID := seedUser(t, pool)
	stationID := seedStation(t, pool, userID, "record-history")
	trackID := seedTrackGraph(t, pool, userID, stationID, trackSeed{
		path:     fmt.Sprintf("/music/%s.flac", uuid.NewString()),
		title:    "History Track",
		artist:   "Test Artist",
		album:    "Test Album",
		position: 1,
	})

	svc := playback.NewService(pool)
	require.NoError(t, svc.Record(ctx, userID, trackID, ""))

	var count int
	err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM listening_history
		WHERE track_id = $1::uuid AND station_id IS NULL
	`, trackID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRecord_UpdatesStationTracksPlayedAt(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	userID := seedUser(t, pool)
	stationID := seedStation(t, pool, userID, "record-station")
	trackID := seedTrackGraph(t, pool, userID, stationID, trackSeed{
		path:     fmt.Sprintf("/music/%s.flac", uuid.NewString()),
		title:    "Station Track",
		artist:   "Rotation Artist",
		album:    "Rotation Album",
		position: 1,
	})

	svc := playback.NewService(pool)
	require.NoError(t, svc.Record(ctx, userID, trackID, stationID))

	var historyStationID *string
	err := pool.QueryRow(ctx, `
		SELECT station_id::text FROM listening_history WHERE track_id = $1::uuid
	`, trackID).Scan(&historyStationID)
	require.NoError(t, err)
	require.NotNil(t, historyStationID)
	assert.Equal(t, stationID, *historyStationID)

	var playedAt *time.Time
	err = pool.QueryRow(ctx, `
		SELECT played_at FROM station_tracks WHERE station_id = $1::uuid AND track_id = $2::uuid
	`, stationID, trackID).Scan(&playedAt)
	require.NoError(t, err)
	assert.NotNil(t, playedAt)
}

func TestRecord_TransactionAtomic(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	userID := seedUser(t, pool)
	stationID := seedStation(t, pool, userID, "atomic-station")
	trackID := seedTrackGraph(t, pool, userID, stationID, trackSeed{
		path:     fmt.Sprintf("/music/%s.flac", uuid.NewString()),
		title:    "Atomic Track",
		artist:   "Atomic Artist",
		album:    "Atomic Album",
		position: 1,
	})

	nonexistentStationID := uuid.NewString()

	svc := playback.NewService(pool)
	err := svc.Record(ctx, userID, trackID, nonexistentStationID)
	require.Error(t, err)

	var count int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM listening_history WHERE track_id = $1::uuid
	`, trackID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestBuildQueue_ExcludesRecentlyPlayed(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	userID := seedUser(t, pool)
	stationID := seedStation(t, pool, userID, "recent-queue")

	recent := time.Now().Add(-1 * time.Hour)
	recentTrackID := seedTrackGraph(t, pool, userID, stationID, trackSeed{
		path:     fmt.Sprintf("/music/%s.flac", uuid.NewString()),
		title:    "Recent Track",
		artist:   "Queue Artist A",
		album:    "Queue Album",
		position: 1,
		playedAt: &recent,
	})
	unplayedTrackID := seedTrackGraph(t, pool, userID, stationID, trackSeed{
		path:     fmt.Sprintf("/music/%s.flac", uuid.NewString()),
		title:    "Fresh Track",
		artist:   "Queue Artist B",
		album:    "Queue Album",
		position: 2,
	})

	svc := playback.NewService(pool)
	queue, err := svc.BuildQueue(ctx, userID, stationID, 24)
	require.NoError(t, err)

	ids := queueTrackIDs(queue)
	assert.Contains(t, ids, unplayedTrackID)
	assert.NotContains(t, ids, recentTrackID)
}

func TestBuildQueue_FallbackWhenAllPlayed(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	userID := seedUser(t, pool)
	stationID := seedStation(t, pool, userID, "fallback-queue")

	recent := time.Now().Add(-1 * time.Hour)
	trackID := seedTrackGraph(t, pool, userID, stationID, trackSeed{
		path:     fmt.Sprintf("/music/%s.flac", uuid.NewString()),
		title:    "Played Track",
		artist:   "Fallback Artist",
		album:    "Fallback Album",
		position: 1,
		playedAt: &recent,
	})

	svc := playback.NewService(pool)
	queue, err := svc.BuildQueue(ctx, userID, stationID, 24)
	require.NoError(t, err)
	require.NotEmpty(t, queue)
	assert.Contains(t, queueTrackIDs(queue), trackID)
}

func TestBuildQueue_ExcludesBanned(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	userID := seedUser(t, pool)
	stationID := seedStation(t, pool, userID, "ban-queue")

	bannedTrackID := seedTrackGraph(t, pool, userID, stationID, trackSeed{
		path:     fmt.Sprintf("/music/%s.flac", uuid.NewString()),
		title:    "Banned Track",
		artist:   "Ban Artist",
		album:    "Ban Album",
		position: 1,
		feedback: "ban",
	})
	allowedTrackID := seedTrackGraph(t, pool, userID, stationID, trackSeed{
		path:     fmt.Sprintf("/music/%s.flac", uuid.NewString()),
		title:    "Allowed Track",
		artist:   "Allow Artist",
		album:    "Allow Album",
		position: 2,
	})

	svc := playback.NewService(pool)
	queue, err := svc.BuildQueue(ctx, userID, stationID, 0)
	require.NoError(t, err)

	ids := queueTrackIDs(queue)
	assert.Contains(t, ids, allowedTrackID)
	assert.NotContains(t, ids, bannedTrackID)
}

func TestBuildQueue_DeduplicatesTitleArtist(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	userID := seedUser(t, pool)
	stationID := seedStation(t, pool, userID, "dedup-queue")

	sharedTitle := "Shared Title"
	sharedArtist := "Shared Artist"

	trackOneID := seedTrackGraph(t, pool, userID, stationID, trackSeed{
		path:     fmt.Sprintf("/music/%s.flac", uuid.NewString()),
		title:    sharedTitle,
		artist:   sharedArtist,
		album:    "Dedup Album One",
		position: 1,
	})
	trackTwoID := seedTrackGraph(t, pool, userID, stationID, trackSeed{
		path:     fmt.Sprintf("/music/%s.flac", uuid.NewString()),
		title:    sharedTitle,
		artist:   sharedArtist,
		album:    "Dedup Album Two",
		position: 2,
	})

	svc := playback.NewService(pool)
	queue, err := svc.BuildQueue(ctx, userID, stationID, 0)
	require.NoError(t, err)

	matches := 0
	for _, q := range queue {
		if q.Title == sharedTitle && q.Artist != nil && *q.Artist == sharedArtist {
			matches++
		}
	}
	assert.Equal(t, 1, matches)
	assert.True(t, containsTrackID(queue, trackOneID) || containsTrackID(queue, trackTwoID))
}

func TestBuildQueue_BanIsPerUser(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	userA := seedUser(t, pool)
	userB := seedUser(t, pool)
	stationA := seedStation(t, pool, userA, "a-station")
	stationB := seedStation(t, pool, userB, "b-station")

	bannedForA := seedTrackGraph(t, pool, userA, stationA, trackSeed{
		path:     fmt.Sprintf("/music/%s.flac", uuid.NewString()),
		title:    "Shared Banned",
		artist:   "Shared Artist",
		album:    "Shared Album",
		position: 1,
		feedback: "ban",
	})
	_, err := pool.Exec(ctx, `
		INSERT INTO station_tracks (station_id, track_id, position)
		SELECT $1::uuid, track_id, position FROM station_tracks WHERE station_id = $2::uuid
	`, stationB, stationA)
	require.NoError(t, err)

	allowed := seedTrackGraph(t, pool, userB, stationB, trackSeed{
		path:     fmt.Sprintf("/music/%s.flac", uuid.NewString()),
		title:    "B Only",
		artist:   "B Artist",
		album:    "B Album",
		position: 2,
	})

	svc := playback.NewService(pool)
	queueA, err := svc.BuildQueue(ctx, userA, stationA, 0)
	require.NoError(t, err)
	assert.NotContains(t, queueTrackIDs(queueA), bannedForA)

	queueB, err := svc.BuildQueue(ctx, userB, stationB, 0)
	require.NoError(t, err)
	idsB := queueTrackIDs(queueB)
	assert.Contains(t, idsB, bannedForA)
	assert.Contains(t, idsB, allowed)
}

func TestBuildQueue_WrongUserSeesEmpty(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	owner := seedUser(t, pool)
	other := seedUser(t, pool)
	stationID := seedStation(t, pool, owner, "owned")
	_ = seedTrackGraph(t, pool, owner, stationID, trackSeed{
		path:     fmt.Sprintf("/music/%s.flac", uuid.NewString()),
		title:    "Owned Track",
		artist:   "Owner",
		album:    "Album",
		position: 1,
	})

	svc := playback.NewService(pool)
	queue, err := svc.BuildQueue(ctx, other, stationID, 0)
	require.NoError(t, err)
	assert.Empty(t, queue)
}

func TestRecord_HistoryIsPerUser(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	userA := seedUser(t, pool)
	userB := seedUser(t, pool)
	stationA := seedStation(t, pool, userA, "hist-a")
	trackID := seedTrackGraph(t, pool, userA, stationA, trackSeed{
		path:     fmt.Sprintf("/music/%s.flac", uuid.NewString()),
		title:    "Hist Track",
		artist:   "Hist Artist",
		album:    "Hist Album",
		position: 1,
	})

	svc := playback.NewService(pool)
	require.NoError(t, svc.Record(ctx, userA, trackID, ""))

	var countA, countB int
	err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM listening_history WHERE user_id = $1::uuid AND track_id = $2::uuid
	`, userA, trackID).Scan(&countA)
	require.NoError(t, err)
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM listening_history WHERE user_id = $1::uuid AND track_id = $2::uuid
	`, userB, trackID).Scan(&countB)
	require.NoError(t, err)
	assert.Equal(t, 1, countA)
	assert.Equal(t, 0, countB)
}

func queueTrackIDs(queue []playback.TrackWithFeatures) []string {
	ids := make([]string, len(queue))
	for i, q := range queue {
		ids[i] = q.ID
	}
	return ids
}

func containsTrackID(queue []playback.TrackWithFeatures, trackID string) bool {
	for _, q := range queue {
		if q.ID == trackID {
			return true
		}
	}
	return false
}
