package stations

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// HiddenTrack is a song a user hid on a station.
type HiddenTrack struct {
	TrackID string  `json:"track_id"`
	Title   string  `json:"title"`
	Artist  *string `json:"artist,omitempty"`
}

// HideTrack keeps a song off one station for one user. Other stations and
// other users are not affected; a ban is the way to never hear it anywhere.
func (s *Service) HideTrack(ctx context.Context, userID, stationID, trackID string) error {
	var song string
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(copy_of, id)::text FROM tracks WHERE id = $1::uuid`, trackID).Scan(&song)
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
		USING tracks ht, tracks t
		WHERE h.user_id = $1::uuid AND h.station_id = $2::uuid AND ht.id = h.track_id
		  AND t.id = $3::uuid AND `+sameSong("ht", "t"), userID, stationID, trackID)
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
