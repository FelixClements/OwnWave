package playback

import (
	"context"
	"math/rand/v2"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ownwave/api/internal/stations"
)

// QueuePageSize caps one queue response; the client refetches when it reaches
// the end. Recently played tracks sort last, so every track gets its turn.
const QueuePageSize = 200

// Service records playback events and builds live station queues.
type Service struct {
	pool               *pgxpool.Pool
	minGenreConfidence float64
}

func NewService(pool *pgxpool.Pool, minGenreConfidence float64) *Service {
	return &Service{pool: pool, minGenreConfidence: minGenreConfidence}
}

// Record appends listening history; stationID (optional) must belong to the user.
func (s *Service) Record(ctx context.Context, userID, trackID, stationID string) error {
	if stationID != "" {
		var owned int
		if err := s.pool.QueryRow(ctx, `
			SELECT 1 FROM stations WHERE id = $1::uuid AND user_id = $2::uuid
		`, stationID, userID).Scan(&owned); err != nil {
			return err
		}
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO listening_history (user_id, track_id, station_id)
		VALUES ($1::uuid, $2, NULLIF($3, '')::uuid)
	`, userID, trackID, stationID)
	return err
}

// poolFrom selects the analysed tracks matching seed, minus the user's bans.
// args[0] must be the user id; joins are inserted before the WHERE clause.
func (s *Service) poolFrom(seed stations.Seed, args *stations.Args, joins string) string {
	return `
		FROM tracks t
		JOIN audio_features af ON af.track_id = t.id
		` + joins + `
		WHERE NOT EXISTS (
			SELECT 1 FROM track_feedback b
			WHERE b.user_id = $1::uuid AND b.track_id = t.id AND b.feedback = 'ban'
		)
		AND ` + seed.Where(args, s.minGenreConfidence)
}

// PoolSize counts the tracks currently matching seed for the user.
func (s *Service) PoolSize(ctx context.Context, userID string, seed stations.Seed) (int, error) {
	args := stations.Args{userID}
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) `+s.poolFrom(seed, &args, ""), args...).Scan(&n)
	return n, err
}

// BuildQueue returns up to QueuePageSize tracks from the station's live pool:
// tracks not played on this station within recentHours come first in random
// order, followed by recently played ones, least recent first.
func (s *Service) BuildQueue(ctx context.Context, userID, stationID string, seed stations.Seed, recentHours int) ([]TrackWithFeatures, error) {
	args := stations.Args{userID, stationID}
	rows, err := s.pool.Query(ctx, `
		SELECT t.id::text, t.title, a.name, al.title, t.path, t.track_number,
		       t.duration_seconds, t.sample_rate, t.channels,
		       COALESCE(af.bpm, 0), COALESCE(af.key, ''), COALESCE(af.energy, 0), COALESCE(af.valence, 0), af.loudness,
		       COALESCE(af.outro_start_seconds, 0), COALESCE(af.ideal_crossfade_seconds, 0),
		       COALESCE(af.intro_start_seconds, 0), COALESCE(af.outro_end_seconds, 0),
		       EXISTS (SELECT 1 FROM track_feedback l WHERE l.user_id = $1::uuid AND l.track_id = t.id AND l.feedback = 'like'),
		       lp.last_played
		`+s.poolFrom(seed, &args, `
		LEFT JOIN artists a ON t.artist_id = a.id
		LEFT JOIN albums al ON t.album_id = al.id
		LEFT JOIN (
			SELECT track_id, MAX(played_at) AS last_played
			FROM listening_history
			WHERE user_id = $1::uuid AND station_id = $2::uuid
			GROUP BY track_id
		) lp ON lp.track_id = t.id`), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []queued
	for rows.Next() {
		var it queued
		q := &it.track
		if err := rows.Scan(&q.ID, &q.Title, &q.Artist, &q.Album, &q.Path,
			&q.TrackNumber, &q.DurationSeconds, &q.SampleRate, &q.Channels,
			&q.BPM, &q.Key, &q.Energy, &q.Valence, &q.Loudness,
			&q.OutroStartSeconds, &q.IdealCrossfadeSeconds,
			&q.IntroStartSeconds, &q.OutroEndSeconds,
			&q.Liked, &it.lastPlayed); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return orderQueue(items, recentHours, time.Now()), nil
}

type queued struct {
	track      TrackWithFeatures
	lastPlayed *time.Time
}

// orderQueue dedupes by title+artist, then puts fresh tracks (shuffled) ahead
// of recently played ones (least recent first) and truncates to a page.
func orderQueue(items []queued, recentHours int, now time.Time) []TrackWithFeatures {
	cutoff := now.Add(-time.Duration(recentHours) * time.Hour)
	seen := make(map[string]bool, len(items))
	var fresh, recent []queued
	for _, it := range items {
		artist := ""
		if it.track.Artist != nil {
			artist = *it.track.Artist
		}
		key := strings.ToLower(strings.TrimSpace(it.track.Title) + "|" + strings.TrimSpace(artist))
		if seen[key] {
			continue
		}
		seen[key] = true
		if it.lastPlayed == nil || it.lastPlayed.Before(cutoff) {
			fresh = append(fresh, it)
		} else {
			recent = append(recent, it)
		}
	}
	rand.Shuffle(len(fresh), func(i, j int) { fresh[i], fresh[j] = fresh[j], fresh[i] })
	sort.SliceStable(recent, func(i, j int) bool { return recent[i].lastPlayed.Before(*recent[j].lastPlayed) })

	ordered := append(fresh, recent...)
	if len(ordered) > QueuePageSize {
		ordered = ordered[:QueuePageSize]
	}
	queue := make([]TrackWithFeatures, len(ordered))
	for i, it := range ordered {
		queue[i] = it.track
		queue[i].Position = i
	}
	return queue
}
