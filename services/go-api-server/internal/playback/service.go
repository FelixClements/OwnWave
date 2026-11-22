package playback

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Service records playback events. Station queues are built by
// stations.Service.Queue.
type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Record appends listening history; stationID (optional) must be the user's
// own station or a shared one.
func (s *Service) Record(ctx context.Context, userID, trackID, stationID string) error {
	if stationID != "" {
		var visible int
		if err := s.pool.QueryRow(ctx, `
			SELECT 1 FROM stations WHERE id = $1::uuid AND (user_id = $2::uuid OR user_id IS NULL)
		`, stationID, userID).Scan(&visible); err != nil {
			return err
		}
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO listening_history (user_id, track_id, station_id)
		VALUES ($1::uuid, $2, NULLIF($3, '')::uuid)
	`, userID, trackID, stationID)
	return err
}
