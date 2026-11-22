// Package library reads the music library: tracks, albums and artists.
//
// Python's analytics service owns these tables (ADR 0005); this package
// only reads them.
package library

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Service reads the library tables.
type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type Track struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Artist          *string  `json:"artist,omitempty"`
	Album           *string  `json:"album,omitempty"`
	Path            string   `json:"-"`
	TrackNumber     *int     `json:"track_number,omitempty"`
	DurationSeconds *float64 `json:"duration_seconds,omitempty"`
	SampleRate      *int     `json:"sample_rate,omitempty"`
	Channels        *int     `json:"channels,omitempty"`
	Loudness        *float64 `json:"loudness,omitempty"`
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

func (s *Service) GetTrackByID(ctx context.Context, id string) (Track, error) {
	var t Track
	err := s.pool.QueryRow(ctx, `
		SELECT t.id::text, t.title, t.path, af.loudness
		FROM tracks t
		LEFT JOIN audio_features af ON t.id = af.track_id
		WHERE t.id = $1
	`, id).Scan(&t.ID, &t.Title, &t.Path, &t.Loudness)
	return t, err
}

func (s *Service) ListTracks(ctx context.Context, limit, offset int, q string) ([]Track, error) {
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
	rows, err := s.pool.Query(ctx, query, args...)
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

func (s *Service) ListAlbums(ctx context.Context) ([]Album, error) {
	rows, err := s.pool.Query(ctx, `
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

func (s *Service) ListArtists(ctx context.Context) ([]Artist, error) {
	rows, err := s.pool.Query(ctx, `
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

func (s *Service) Search(ctx context.Context, query string) (SearchResults, error) {
	results := SearchResults{
		Tracks:  make([]Track, 0),
		Albums:  make([]Album, 0),
		Artists: make([]Artist, 0),
	}
	tsquery := strings.TrimSpace(query)
	if tsquery == "" {
		return results, nil
	}

	artistRows, err := s.pool.Query(ctx, `
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

	albumRows, err := s.pool.Query(ctx, `
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

	trackRows, err := s.pool.Query(ctx, `
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

func (s *Service) CountTracks(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM tracks`).Scan(&count)
	return count, err
}
