// Package settings holds app_state: small JSON values under fixed keys.
//
// Each key has one writing service (ADR 0005). The Key constants below are
// every key in use; a new key gets a constant here and a row in the ADR.
package settings

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Key names an app_state row.
type Key string

const (
	// SetupCompleted ({"completed": bool}) is set once the admin finishes
	// setup. Written by Go, through Service.Set.
	SetupCompleted Key = "setup_completed"
	// AutoStations ({"enabled": bool}) switches genre stations on or off.
	// Written by Go, in stations.Service.SetGenreStationsEnabled.
	AutoStations Key = "auto_stations"
	// GenresScored ({"tracks": n, "scored": n}) is the last genre scoring
	// run. Written by Python (score_genres in tasks.py); Go only reads it.
	GenresScored Key = "genres_scored"
)

// Service reads and writes app_state.
type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Get returns the value stored under key; pgx.ErrNoRows if there is none.
func (s *Service) Get(ctx context.Context, key Key) (map[string]interface{}, error) {
	var value map[string]interface{}
	err := s.pool.QueryRow(ctx, `
		SELECT value
		FROM app_state
		WHERE key = $1
	`, string(key)).Scan(&value)
	if err != nil {
		return nil, err
	}
	return value, nil
}

// Set stores value under key. Only call it for a key Go writes.
func (s *Service) Set(ctx context.Context, key Key, value map[string]interface{}) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO app_state (key, value, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			updated_at = NOW()
	`, string(key), value)
	return err
}
