package main

import (
	"context"
	"strings"
	"time"

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
	Loudness        *float64 `json:"loudness,omitempty"`
}

type TrackWithFeatures struct {
	Track
	BPM                   float64 `json:"bpm"`
	Key                   string  `json:"key"`
	Energy                float64 `json:"energy"`
	Valence               float64 `json:"valence"`
	OutroStartSeconds     float64 `json:"outro_start_seconds"`
	IdealCrossfadeSeconds float64 `json:"ideal_crossfade_seconds"`
	IntroStartSeconds     float64 `json:"intro_start_seconds"`
	OutroEndSeconds       float64 `json:"outro_end_seconds"`
	Position              int     `json:"position"`
	Liked                 bool    `json:"liked"`
}

type Station struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	SeedFeatures *string `json:"seed_features,omitempty"`
}

func (db *DB) GetTrackByID(ctx context.Context, id string) (Track, error) {
	var t Track
	err := db.pool.QueryRow(ctx, `
		SELECT t.id::text, t.title, t.path, af.loudness
		FROM tracks t
		LEFT JOIN audio_features af ON t.id = af.track_id
		WHERE t.id = $1
	`, id).Scan(&t.ID, &t.Title, &t.Path, &t.Loudness)
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

func (db *DB) ListAlbums(ctx context.Context) ([]Album, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id::text, title
		FROM albums
		ORDER BY title
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var albums []Album
	for rows.Next() {
		var a Album
		if err := rows.Scan(&a.ID, &a.Title); err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

func (db *DB) ListArtists(ctx context.Context) ([]Artist, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id::text, name
		FROM artists
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var artists []Artist
	for rows.Next() {
		var a Artist
		if err := rows.Scan(&a.ID, &a.Name); err != nil {
			return nil, err
		}
		artists = append(artists, a)
	}
	return artists, rows.Err()
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

type StationQueueStatus struct {
	Station
	TrackCount  int `json:"track_count"`
	PlayedCount int `json:"played_count"`
}

func (db *DB) ListStationsWithQueueStatus(ctx context.Context) ([]StationQueueStatus, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT s.id::text, s.name, s.seed_features::text,
		       COUNT(st.track_id) FILTER (WHERE st.station_id = s.id),
		       COUNT(st.track_id) FILTER (WHERE st.station_id = s.id AND st.played_at IS NOT NULL)
		FROM stations s
		LEFT JOIN station_tracks st ON s.id = st.station_id
		GROUP BY s.id, s.name, s.seed_features
		ORDER BY s.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StationQueueStatus
	for rows.Next() {
		var q StationQueueStatus
		if err := rows.Scan(&q.ID, &q.Name, &q.SeedFeatures, &q.TrackCount, &q.PlayedCount); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (db *DB) GetStationByID(ctx context.Context, stationID string) (Station, error) {
	var s Station
	err := db.pool.QueryRow(ctx, `
		SELECT id::text, name, seed_features::text
		FROM stations
		WHERE id = $1
	`, stationID).Scan(&s.ID, &s.Name, &s.SeedFeatures)
	return s, err
}

func (db *DB) UpdateStation(ctx context.Context, stationID, name, seedFeatures string) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE stations
		SET name = $2,
		    seed_features = COALESCE(NULLIF($3, '')::jsonb, seed_features)
		WHERE id = $1
	`, stationID, name, seedFeatures)
	return err
}

func (db *DB) DeleteStation(ctx context.Context, stationID string) error {
	_, err := db.pool.Exec(ctx, `
		DELETE FROM station_tracks WHERE station_id = $1;
		DELETE FROM stations WHERE id = $1;
	`, stationID)
	return err
}

func (db *DB) GetStationQueue(ctx context.Context, stationID string) ([]TrackWithFeatures, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT t.id::text, t.title, a.name, al.title, t.path, t.track_number,
		       t.duration_seconds, t.sample_rate, t.channels,
		       af.bpm, af.key, af.energy, af.valence, af.loudness,
		       COALESCE(af.outro_start_seconds, 0), COALESCE(af.ideal_crossfade_seconds, 0),
		       COALESCE(af.intro_start_seconds, 0), COALESCE(af.outro_end_seconds, 0),
		       st.position,
		       EXISTS (SELECT 1 FROM track_feedback WHERE track_id = t.id AND feedback = 'like') AS liked
		FROM station_tracks st
		JOIN tracks t ON st.track_id = t.id
		JOIN audio_features af ON t.id = af.track_id
		LEFT JOIN artists a ON t.artist_id = a.id
		LEFT JOIN albums al ON t.album_id = al.id
		LEFT JOIN track_feedback f ON t.id = f.track_id AND f.feedback = 'ban'
		WHERE st.station_id = $1 AND f.track_id IS NULL
		ORDER BY CASE
			WHEN EXISTS (SELECT 1 FROM track_feedback WHERE track_id = t.id AND feedback = 'like') THEN 0
			WHEN EXISTS (SELECT 1 FROM track_feedback WHERE track_id = t.id AND feedback = 'skip') THEN 2
			ELSE 1
		END, st.position
	`, stationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queue []TrackWithFeatures
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

func (db *DB) MarkTrackPlayed(ctx context.Context, stationID, trackID string) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE station_tracks
		SET played_at = NOW()
		WHERE station_id = $1 AND track_id = $2
	`, stationID, trackID)
	return err
}

type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
}

func (db *DB) CreateUser(ctx context.Context, username, passwordHash string) (string, error) {
	var id string
	err := db.pool.QueryRow(ctx, `
		INSERT INTO users (username, password_hash)
		VALUES ($1, $2)
		ON CONFLICT (username) DO NOTHING
		RETURNING id::text
	`, username, passwordHash).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (db *DB) GetUserByUsername(ctx context.Context, username string) (User, error) {
	var u User
	err := db.pool.QueryRow(ctx, `
		SELECT id::text, username, password_hash
		FROM users
		WHERE username = $1
	`, username).Scan(&u.ID, &u.Username, &u.PasswordHash)
	return u, err
}

func (db *DB) CreateSession(ctx context.Context, userID, tokenHash string, expiresAt time.Time) (string, error) {
	var id string
	err := db.pool.QueryRow(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id::text
	`, userID, tokenHash, expiresAt).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (db *DB) GetUserByTokenHash(ctx context.Context, tokenHash string) (User, error) {
	var u User
	err := db.pool.QueryRow(ctx, `
		SELECT u.id::text, u.username, u.password_hash
		FROM users u
		JOIN sessions s ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > NOW()
	`, tokenHash).Scan(&u.ID, &u.Username, &u.PasswordHash)
	return u, err
}

func (db *DB) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := db.pool.Exec(ctx, `
		DELETE FROM sessions
		WHERE token_hash = $1
	`, tokenHash)
	return err
}

type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Album struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type SearchResults struct {
	Tracks  []Track  `json:"tracks"`
	Albums  []Album  `json:"albums"`
	Artists []Artist `json:"artists"`
}

func (db *DB) Search(ctx context.Context, query string) (SearchResults, error) {
	var results SearchResults
	tsquery := strings.TrimSpace(query)
	if tsquery == "" {
		return results, nil
	}

	artistRows, err := db.pool.Query(ctx, `
		SELECT id::text, name
		FROM artists
		WHERE to_tsvector('simple', name) @@ plainto_tsquery('simple', $1)
		LIMIT 20
	`, tsquery)
	if err != nil {
		return results, err
	}
	defer artistRows.Close()
	for artistRows.Next() {
		var a Artist
		if err := artistRows.Scan(&a.ID, &a.Name); err != nil {
			return results, err
		}
		results.Artists = append(results.Artists, a)
	}
	if err := artistRows.Err(); err != nil {
		return results, err
	}

	albumRows, err := db.pool.Query(ctx, `
		SELECT id::text, title
		FROM albums
		WHERE to_tsvector('simple', title) @@ plainto_tsquery('simple', $1)
		LIMIT 20
	`, tsquery)
	if err != nil {
		return results, err
	}
	defer albumRows.Close()
	for albumRows.Next() {
		var a Album
		if err := albumRows.Scan(&a.ID, &a.Title); err != nil {
			return results, err
		}
		results.Albums = append(results.Albums, a)
	}
	if err := albumRows.Err(); err != nil {
		return results, err
	}

	trackRows, err := db.pool.Query(ctx, `
		SELECT t.id::text, t.title, a.name, al.title
		FROM tracks t
		LEFT JOIN artists a ON t.artist_id = a.id
		LEFT JOIN albums al ON t.album_id = al.id
		WHERE to_tsvector('simple', coalesce(t.title, '') || ' ' || coalesce(a.name, '') || ' ' || coalesce(al.title, '')) @@ plainto_tsquery('simple', $1)
		LIMIT 20
	`, tsquery)
	if err != nil {
		return results, err
	}
	defer trackRows.Close()
	for trackRows.Next() {
		var t Track
		if err := trackRows.Scan(&t.ID, &t.Title, &t.Artist, &t.Album); err != nil {
			return results, err
		}
		results.Tracks = append(results.Tracks, t)
	}
	if err := trackRows.Err(); err != nil {
		return results, err
	}

	return results, nil
}

type HistoryEntry struct {
	TrackID   string    `json:"track_id"`
	Title     string    `json:"title"`
	Artist    *string   `json:"artist"`
	Album     *string   `json:"album"`
	StationID *string   `json:"station_id"`
	PlayedAt  time.Time `json:"played_at"`
}

func (db *DB) RecordPlay(ctx context.Context, trackID, stationID string) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO listening_history (track_id, station_id)
		VALUES ($1, NULLIF($2, '')::uuid)
	`, trackID, stationID)
	return err
}

func (db *DB) RecordFeedback(ctx context.Context, trackID, feedback string) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO track_feedback (track_id, feedback)
		VALUES ($1, $2)
		ON CONFLICT (track_id, feedback) DO UPDATE SET created_at = NOW()
	`, trackID, feedback)
	return err
}

func (db *DB) ListHistory(ctx context.Context, limit int) ([]HistoryEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.pool.Query(ctx, `
		SELECT t.id::text, t.title, a.name, al.title, h.station_id::text, h.played_at
		FROM listening_history h
		JOIN tracks t ON h.track_id = t.id
		LEFT JOIN artists a ON t.artist_id = a.id
		LEFT JOIN albums al ON t.album_id = al.id
		ORDER BY h.played_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []HistoryEntry
	for rows.Next() {
		var e HistoryEntry
		if err := rows.Scan(&e.TrackID, &e.Title, &e.Artist, &e.Album, &e.StationID, &e.PlayedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (db *DB) ListFeedback(ctx context.Context, feedback string, limit int) ([]Track, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.pool.Query(ctx, `
		SELECT t.id::text, t.title, a.name, al.title
		FROM track_feedback f
		JOIN tracks t ON f.track_id = t.id
		LEFT JOIN artists a ON t.artist_id = a.id
		LEFT JOIN albums al ON t.album_id = al.id
		WHERE f.feedback = $1
		ORDER BY f.created_at DESC
		LIMIT $2
	`, feedback, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tracks []Track
	for rows.Next() {
		var t Track
		if err := rows.Scan(&t.ID, &t.Title, &t.Artist, &t.Album); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}
