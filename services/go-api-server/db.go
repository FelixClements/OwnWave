package main

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ownwave/api/internal/playback"
)

type DB struct {
	pool *pgxpool.Pool
}

func NewDB(pool *pgxpool.Pool) *DB {
	return &DB{pool: pool}
}

type Track = playback.Track

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

func (db *DB) ListTracks(ctx context.Context, limit, offset int, q string) ([]Track, error) {
	args := []any{limit, offset}
	where := ""
	if q != "" {
		where = `WHERE t.title ILIKE $3 OR a.name ILIKE $3 OR al.title ILIKE $3`
		args = append(args, "%"+q+"%")
	}
	query := `
		SELECT t.id::text, t.title, a.name, al.title, t.path, t.track_number,
		       t.duration_seconds, t.sample_rate, t.channels
		FROM tracks t
		LEFT JOIN artists a ON t.artist_id = a.id
		LEFT JOIN albums al ON t.album_id = al.id
		` + where + `
		ORDER BY t.title
		LIMIT $1 OFFSET $2
	`
	rows, err := db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tracks := make([]Track, 0)
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
	albums := make([]Album, 0)
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
	artists := make([]Artist, 0)
	for rows.Next() {
		var a Artist
		if err := rows.Scan(&a.ID, &a.Name); err != nil {
			return nil, err
		}
		artists = append(artists, a)
	}
	return artists, rows.Err()
}

func (db *DB) ListStations(ctx context.Context, userID string) ([]Station, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id::text, name, seed_features::text
		FROM stations
		WHERE user_id = $1::uuid
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stations := make([]Station, 0)
	for rows.Next() {
		var s Station
		if err := rows.Scan(&s.ID, &s.Name, &s.SeedFeatures); err != nil {
			return nil, err
		}
		stations = append(stations, s)
	}
	return stations, rows.Err()
}

func (db *DB) GetStationByID(ctx context.Context, userID, stationID string) (Station, error) {
	var s Station
	err := db.pool.QueryRow(ctx, `
		SELECT id::text, name, seed_features::text
		FROM stations
		WHERE id = $1::uuid AND user_id = $2::uuid
	`, stationID, userID).Scan(&s.ID, &s.Name, &s.SeedFeatures)
	return s, err
}

// CreateStation stores a station; seed is the stations.Seed JSON (nil = whole library).
func (db *DB) CreateStation(ctx context.Context, userID, name string, seed []byte) (string, error) {
	var id string
	err := db.pool.QueryRow(ctx, `
		INSERT INTO stations (user_id, name, seed_features)
		VALUES ($1::uuid, $2, $3::jsonb)
		RETURNING id::text
	`, userID, name, seed).Scan(&id)
	return id, err
}

// UpdateStation renames a station and replaces its seed.
func (db *DB) UpdateStation(ctx context.Context, userID, stationID, name string, seed []byte) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE stations SET name = $3, seed_features = $4::jsonb
		WHERE id = $1::uuid AND user_id = $2::uuid
	`, stationID, userID, name, seed)
	return err
}

// EnsureStation returns the user's station with this name, creating it with
// seed when missing.
func (db *DB) EnsureStation(ctx context.Context, userID, name string, seed []byte) (id string, created bool, err error) {
	err = db.pool.QueryRow(ctx, `
		SELECT id::text FROM stations WHERE user_id = $1::uuid AND name = $2
		ORDER BY created_at LIMIT 1
	`, userID, name).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", false, err
	}
	id, err = db.CreateStation(ctx, userID, name, seed)
	return id, err == nil, err
}

// PlayedCount is the number of distinct tracks the user has played on a station.
func (db *DB) PlayedCount(ctx context.Context, userID, stationID string) (int, error) {
	var n int
	err := db.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT track_id) FROM listening_history
		WHERE user_id = $1::uuid AND station_id = $2::uuid
	`, userID, stationID).Scan(&n)
	return n, err
}

type GenreCount struct {
	MainGenre  string
	SubGenre   string
	TrackCount int
}

// GenreCounts returns per-main-genre (SubGenre empty) and per-sub-genre track
// counts for predictions at or above minConfidence.
func (db *DB) GenreCounts(ctx context.Context, minConfidence float64) ([]GenreCount, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT main_genre, COALESCE(sub_genre, ''), COUNT(DISTINCT track_id)
		FROM track_genres
		WHERE confidence >= $1
		GROUP BY GROUPING SETS ((main_genre), (main_genre, sub_genre))
		ORDER BY main_genre, sub_genre NULLS FIRST
	`, minConfidence)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]GenreCount, 0)
	for rows.Next() {
		var g GenreCount
		if err := rows.Scan(&g.MainGenre, &g.SubGenre, &g.TrackCount); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

type SimilarTrack struct {
	TrackID  string  `json:"track_id"`
	Title    string  `json:"title"`
	Artist   *string `json:"artist,omitempty"`
	Album    *string `json:"album,omitempty"`
	Distance float64 `json:"distance"`
}

// SimilarTracks returns the nearest tracks by cosine distance of feature vectors.
func (db *DB) SimilarTracks(ctx context.Context, trackID string, limit int) ([]SimilarTrack, error) {
	rows, err := db.pool.Query(ctx, `
		WITH target AS (
			SELECT feature_vector FROM audio_features
			WHERE track_id = $1::uuid AND feature_vector IS NOT NULL
		)
		SELECT af.track_id::text, t.title, a.name, al.title,
		       (af.feature_vector <=> target.feature_vector)::float8 AS distance
		FROM audio_features af
		CROSS JOIN target
		JOIN tracks t ON t.id = af.track_id
		LEFT JOIN artists a ON t.artist_id = a.id
		LEFT JOIN albums al ON t.album_id = al.id
		WHERE af.track_id <> $1::uuid AND af.feature_vector IS NOT NULL
		ORDER BY distance
		LIMIT $2
	`, trackID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SimilarTrack, 0)
	for rows.Next() {
		var st SimilarTrack
		if err := rows.Scan(&st.TrackID, &st.Title, &st.Artist, &st.Album, &st.Distance); err != nil {
			return nil, err
		}
		st.Distance = math.Round(st.Distance*1e4) / 1e4
		out = append(out, st)
	}
	return out, rows.Err()
}

func (db *DB) DeleteStation(ctx context.Context, userID, stationID string) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `DELETE FROM stations WHERE id = $1 AND user_id = $2::uuid`, stationID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return tx.Commit(ctx)
}

func (db *DB) UpdateUserProfile(ctx context.Context, userID, email, fullName string) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE users
		SET email = NULLIF($2, ''),
		    full_name = NULLIF($3, '')
		WHERE id = $1
	`, userID, email, fullName)
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
	results := SearchResults{
		Tracks:  make([]Track, 0),
		Albums:  make([]Album, 0),
		Artists: make([]Artist, 0),
	}
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

func (db *DB) RecordFeedback(ctx context.Context, userID, trackID, feedback string) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO track_feedback (user_id, track_id, feedback)
		VALUES ($1::uuid, $2, $3)
		ON CONFLICT (user_id, track_id, feedback) DO UPDATE SET created_at = NOW()
	`, userID, trackID, feedback)
	return err
}

func (db *DB) DeleteFeedback(ctx context.Context, userID, trackID, feedback string) error {
	_, err := db.pool.Exec(ctx, `
		DELETE FROM track_feedback
		WHERE user_id = $1::uuid AND track_id = $2 AND feedback = $3
	`, userID, trackID, feedback)
	return err
}

func (db *DB) ListHistory(ctx context.Context, userID string, limit int) ([]HistoryEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.pool.Query(ctx, `
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

func (db *DB) ListFeedback(ctx context.Context, userID, feedback string, limit int) ([]Track, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.pool.Query(ctx, `
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
	tracks := make([]Track, 0)
	for rows.Next() {
		var t Track
		if err := rows.Scan(&t.ID, &t.Title, &t.Artist, &t.Album); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

func (db *DB) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func (db *DB) CountTracks(ctx context.Context) (int, error) {
	var count int
	err := db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM tracks`).Scan(&count)
	return count, err
}

func (db *DB) GetAppState(ctx context.Context, key string) (map[string]interface{}, error) {
	var value map[string]interface{}
	err := db.pool.QueryRow(ctx, `
		SELECT value
		FROM app_state
		WHERE key = $1
	`, key).Scan(&value)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (db *DB) SetAppState(ctx context.Context, key string, value map[string]interface{}) error {
	_, err := db.pool.Exec(ctx, `
		INSERT INTO app_state (key, value, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			updated_at = NOW()
	`, key, value)
	return err
}
