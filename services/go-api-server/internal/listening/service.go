// Package listening records what each user hears and how they react to it:
// listening history, likes and bans, and songs hidden on a station.
//
// It is the only writer of listening_history, track_feedback and
// station_hidden_tracks (ADR 0005). Other packages, such as the station
// queue, may read those tables in SQL.
package listening

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ownwave/api/internal/library"
)

// ErrNotFound means the track to hide does not exist.
var ErrNotFound = errors.New("not found")

// Service writes and reads a user's listening data.
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

type HistoryEntry struct {
	TrackID   string    `json:"track_id"`
	Title     string    `json:"title"`
	Artist    *string   `json:"artist"`
	Album     *string   `json:"album"`
	StationID *string   `json:"station_id"`
	PlayedAt  time.Time `json:"played_at"`
}

func (s *Service) ListHistory(ctx context.Context, userID string, limit int) ([]HistoryEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT t.id::text, t.title, a.name, al.title, h.station_id::text, h.played_at
		FROM listening_history h
		JOIN tracks t ON h.track_id = t.id
		LEFT JOIN artists a ON t.artist_id = a.id
		LEFT JOIN albums al ON t.album_id = al.id
		WHERE h.user_id = $1::uuid
		ORDER BY h.played_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]HistoryEntry, 0)
	for rows.Next() {
		var e HistoryEntry
		if err := rows.Scan(&e.TrackID, &e.Title, &e.Artist, &e.Album, &e.StationID, &e.PlayedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// PlayedCount is the number of distinct tracks the user has played on a station.
func (s *Service) PlayedCount(ctx context.Context, userID, stationID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT track_id) FROM listening_history
		WHERE user_id = $1::uuid AND station_id = $2::uuid
	`, userID, stationID).Scan(&n)
	return n, err
}

func (s *Service) RecordFeedback(ctx context.Context, userID, trackID, feedback string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO track_feedback (user_id, track_id, feedback)
		VALUES ($1::uuid, $2, $3)
		ON CONFLICT (user_id, track_id, feedback) DO UPDATE SET created_at = NOW()
	`, userID, trackID, feedback)
	return err
}

// DeleteFeedback takes a like or ban off the whole song, through any of its copies.
func (s *Service) DeleteFeedback(ctx context.Context, userID, trackID, feedback string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM track_feedback f
		USING songs fs, songs ts
		WHERE f.user_id = $1::uuid AND f.feedback = $3 AND fs.track_id = f.track_id
		  AND ts.track_id = $2::uuid AND fs.song_id = ts.song_id
	`, userID, trackID, feedback)
	return err
}

func (s *Service) ListFeedback(ctx context.Context, userID, feedback string, limit int) ([]library.Track, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT t.id::text, t.title, a.name, al.title
		FROM track_feedback f
		JOIN tracks t ON f.track_id = t.id
		LEFT JOIN artists a ON t.artist_id = a.id
		LEFT JOIN albums al ON t.album_id = al.id
		WHERE f.user_id = $1::uuid AND f.feedback = $2
		ORDER BY f.created_at DESC
		LIMIT $3
	`, userID, feedback, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tracks := make([]library.Track, 0)
	for rows.Next() {
		var t library.Track
		if err := rows.Scan(&t.ID, &t.Title, &t.Artist, &t.Album); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

// HiddenTrack is a song a user hid on a station.
type HiddenTrack struct {
	TrackID string  `json:"track_id"`
	Title   string  `json:"title"`
	Artist  *string `json:"artist,omitempty"`
}

// HideTrack keeps a song off one station for one user. Other stations and
// other users are not affected; a ban is the way to never hear it anywhere.
// An unknown track is ErrNotFound.
func (s *Service) HideTrack(ctx context.Context, userID, stationID, trackID string) error {
	var song string
	err := s.pool.QueryRow(ctx, `SELECT song_id::text FROM songs WHERE track_id = $1::uuid`, trackID).Scan(&song)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO station_hidden_tracks (user_id, station_id, track_id)
		VALUES ($1::uuid, $2::uuid, $3::uuid)
		ON CONFLICT DO NOTHING
	`, userID, stationID, song)
	return err
}

// UnhideTrack brings a song back to the station, through any of its copies.
func (s *Service) UnhideTrack(ctx context.Context, userID, stationID, trackID string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM station_hidden_tracks h
		USING songs hs, songs ts
		WHERE h.user_id = $1::uuid AND h.station_id = $2::uuid AND hs.track_id = h.track_id
		  AND ts.track_id = $3::uuid AND hs.song_id = ts.song_id
	`, userID, stationID, trackID)
	return err
}

// HiddenTracks lists what the user hid on the station, newest first.
func (s *Service) HiddenTracks(ctx context.Context, userID, stationID string) ([]HiddenTrack, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id::text, t.title, a.name
		FROM station_hidden_tracks h
		JOIN tracks t ON t.id = h.track_id
		LEFT JOIN artists a ON a.id = t.artist_id
		WHERE h.user_id = $1::uuid AND h.station_id = $2::uuid
		ORDER BY h.created_at DESC, t.id
	`, userID, stationID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (HiddenTrack, error) {
		var t HiddenTrack
		err := row.Scan(&t.TrackID, &t.Title, &t.Artist)
		return t, err
	})
}
