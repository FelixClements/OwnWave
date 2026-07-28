package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	pool *pgxpool.Pool
}

func NewDB(pool *pgxpool.Pool) *DB {
	return &DB{pool: pool}
}

type Track struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Artist          *string  `json:"artist,omitempty"`
	Album           *string  `json:"album,omitempty"`
	Path            string   `json:"path"`
	TrackNumber     *int     `json:"track_number,omitempty"`
	DurationSeconds *float64 `json:"duration_seconds,omitempty"`
	SampleRate      *int     `json:"sample_rate,omitempty"`
	Channels        *int     `json:"channels,omitempty"`
}

type TrackWithFeatures struct {
	Track
	BPM                    float64 `json:"bpm"`
	Key                    string  `json:"key"`
	Energy                 float64 `json:"energy"`
	Valence                float64 `json:"valence"`
	OutroStartSeconds      float64 `json:"outro_start_seconds"`
	IdealCrossfadeSeconds  float64 `json:"ideal_crossfade_seconds"`
	Position               int     `json:"position"`
}

type Station struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	SeedFeatures *string `json:"seed_features,omitempty"`
}

func (db *DB) GetTrackByID(ctx context.Context, id string) (Track, error) {
	var t Track
	err := db.pool.QueryRow(ctx, `
		SELECT t.id::text, t.title, t.path
		FROM tracks t
		WHERE t.id = $1
	`, id).Scan(&t.ID, &t.Title, &t.Path)
	return t, err
}

func (db *DB) ListTracks(ctx context.Context) ([]Track, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT t.id::text, t.title, a.name, al.title, t.path, t.track_number,
		       t.duration_seconds, t.sample_rate, t.channels
		FROM tracks t
		LEFT JOIN artists a ON t.artist_id = a.id
		LEFT JOIN albums al ON t.album_id = al.id
		ORDER BY t.title
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		var t Track
		if err := rows.Scan(&t.ID, &t.Title, &t.Artist, &t.Album, &t.Path,
			&t.TrackNumber, &t.DurationSeconds, &t.SampleRate, &t.Channels); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

func (db *DB) ListStations(ctx context.Context) ([]Station, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id::text, name, seed_features::text
		FROM stations
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stations []Station
	for rows.Next() {
		var s Station
		if err := rows.Scan(&s.ID, &s.Name, &s.SeedFeatures); err != nil {
			return nil, err
		}
		stations = append(stations, s)
	}
	return stations, rows.Err()
}

func (db *DB) GetStationQueue(ctx context.Context, stationID string) ([]TrackWithFeatures, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT t.id::text, t.title, a.name, al.title, t.path, t.track_number,
		       t.duration_seconds, t.sample_rate, t.channels,
		       af.bpm, af.key, af.energy, af.valence,
		       af.outro_start_seconds, af.ideal_crossfade_seconds,
		       st.position
		FROM station_tracks st
		JOIN tracks t ON st.track_id = t.id
		JOIN audio_features af ON t.id = af.track_id
		LEFT JOIN artists a ON t.artist_id = a.id
		LEFT JOIN albums al ON t.album_id = al.id
		WHERE st.station_id = $1
		ORDER BY st.position
	`, stationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queue []TrackWithFeatures
	for rows.Next() {
		var q TrackWithFeatures
		if err := rows.Scan(&q.ID, &q.Title, &q.Artist, &q.Album, &q.Path,
			&q.TrackNumber, &q.DurationSeconds, &q.SampleRate, &q.Channels,
			&q.BPM, &q.Key, &q.Energy, &q.Valence,
			&q.OutroStartSeconds, &q.IdealCrossfadeSeconds, &q.Position); err != nil {
			return nil, err
		}
		queue = append(queue, q)
	}
	return queue, rows.Err()
}

func (db *DB) MarkTrackPlayed(ctx context.Context, stationID, trackID string) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE station_tracks
		SET played_at = NOW()
		WHERE station_id = $1 AND track_id = $2
	`, stationID, trackID)
	return err
}
