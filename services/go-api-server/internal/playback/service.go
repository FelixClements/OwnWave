package playback

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Service records playback events and builds station queues for rotation.
type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Record appends listening history and, when stationID is set, updates rotation state atomically.
func (s *Service) Record(ctx context.Context, userID, trackID, stationID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if stationID != "" {
		var owned int
		if err := tx.QueryRow(ctx, `
			SELECT 1 FROM stations WHERE id = $1::uuid AND user_id = $2::uuid
		`, stationID, userID).Scan(&owned); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO listening_history (user_id, track_id, station_id)
		VALUES ($1::uuid, $2, NULLIF($3, '')::uuid)
	`, userID, trackID, stationID); err != nil {
		return err
	}

	if stationID != "" {
		if _, err := tx.Exec(ctx, `
			UPDATE station_tracks
			SET played_at = NOW()
			WHERE station_id = $1 AND track_id = $2
		`, stationID, trackID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// BuildQueue returns a shuffled playable pool, preferring tracks not played within recentHours.
func (s *Service) BuildQueue(ctx context.Context, userID, stationID string, recentHours int) ([]TrackWithFeatures, error) {
	queue, err := s.queryStationQueue(ctx, userID, stationID, recentHours)
	if err != nil {
		return nil, err
	}
	if len(queue) == 0 {
		queue, err = s.queryStationQueue(ctx, userID, stationID, 0)
		if err != nil {
			return nil, err
		}
	}
	rand.Shuffle(len(queue), func(i, j int) { queue[i], queue[j] = queue[j], queue[i] })
	return queue, nil
}

func (s *Service) queryStationQueue(ctx context.Context, userID, stationID string, recentHours int) ([]TrackWithFeatures, error) {
	recentFilter := ""
	args := []any{stationID, userID}
	if recentHours > 0 {
		recentFilter = " AND (st.played_at IS NULL OR st.played_at < NOW() - INTERVAL '1 hour' * $3)"
		args = append(args, recentHours)
	}

	query := fmt.Sprintf(`
		SELECT t.id::text, t.title, a.name, al.title, t.path, t.track_number,
		       t.duration_seconds, t.sample_rate, t.channels,
		       af.bpm, af.key, af.energy, af.valence, af.loudness,
		       COALESCE(af.outro_start_seconds, 0), COALESCE(af.ideal_crossfade_seconds, 0),
		       COALESCE(af.intro_start_seconds, 0), COALESCE(af.outro_end_seconds, 0),
		       st.position,
		       EXISTS (SELECT 1 FROM track_feedback WHERE user_id = $2::uuid AND track_id = t.id AND feedback = 'like') AS liked
		FROM station_tracks st
		JOIN tracks t ON st.track_id = t.id
		JOIN audio_features af ON t.id = af.track_id
		JOIN stations s ON s.id = st.station_id AND s.user_id = $2::uuid
		LEFT JOIN artists a ON t.artist_id = a.id
		LEFT JOIN albums al ON t.album_id = al.id
		LEFT JOIN track_feedback f ON t.id = f.track_id AND f.user_id = $2::uuid AND f.feedback = 'ban'
		WHERE st.station_id = $1 AND f.track_id IS NULL%s
	`, recentFilter)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	queue := make([]TrackWithFeatures, 0)
	seen := make(map[string]bool)
	for rows.Next() {
		var q TrackWithFeatures
		if err := rows.Scan(&q.ID, &q.Title, &q.Artist, &q.Album, &q.Path,
			&q.TrackNumber, &q.DurationSeconds, &q.SampleRate, &q.Channels,
			&q.BPM, &q.Key, &q.Energy, &q.Valence,
			&q.Loudness,
			&q.OutroStartSeconds, &q.IdealCrossfadeSeconds,
			&q.IntroStartSeconds, &q.OutroEndSeconds,
			&q.Position, &q.Liked); err != nil {
			return nil, err
		}
		artist := ""
		if q.Artist != nil {
			artist = *q.Artist
		}
		key := strings.ToLower(strings.TrimSpace(q.Title) + "|" + strings.TrimSpace(artist))
		if seen[key] {
			continue
		}
		seen[key] = true
		queue = append(queue, q)
	}
	return queue, rows.Err()
}
