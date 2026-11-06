package stations

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// GenreCorrection is an admin's genre for a song or for an artist. Genre
// scoring in the analytics worker applies it: a song's correction wins over
// its artist's, which wins over the blended score.
type GenreCorrection struct {
	ID string `json:"id"`
	// TrackID, TrackTitle and TrackArtist are set on a song's correction.
	TrackID     *string `json:"track_id,omitempty"`
	TrackTitle  *string `json:"track_title,omitempty"`
	TrackArtist *string `json:"track_artist,omitempty"`
	// Artist is set on an artist's correction.
	Artist    *string   `json:"artist,omitempty"`
	Genre     string    `json:"genre"`
	CreatedAt time.Time `json:"created_at"`
}

// GenreCorrectionRequest corrects a song (TrackID) or an artist (Artist).
type GenreCorrectionRequest struct {
	TrackID string `json:"track_id"`
	Artist  string `json:"artist"`
	Genre   string `json:"genre"`
}

// SaveGenreCorrection stores or replaces a correction. The caller has the
// genres rescored.
func (s *Service) SaveGenreCorrection(ctx context.Context, req GenreCorrectionRequest) error {
	artist := strings.TrimSpace(req.Artist)
	if (req.TrackID == "") == (artist == "") {
		return fmt.Errorf("%w: correct either a track or an artist", ErrInvalid)
	}
	var known bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM station_genres WHERE is_station AND id = $1)
	`, req.Genre).Scan(&known); err != nil {
		return err
	}
	if !known {
		return fmt.Errorf("%w: unknown genre %q", ErrInvalid, req.Genre)
	}
	if artist != "" {
		_, err := s.pool.Exec(ctx, `
			INSERT INTO genre_corrections (artist, genre) VALUES ($1, $2)
			ON CONFLICT (lower(artist)) WHERE artist IS NOT NULL
			DO UPDATE SET artist = EXCLUDED.artist, genre = EXCLUDED.genre, created_at = NOW()
		`, artist, req.Genre)
		return err
	}
	if _, err := uuid.Parse(req.TrackID); err != nil {
		return fmt.Errorf("%w: track_id must be a valid id", ErrInvalid)
	}
	var song string
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(copy_of, id)::text FROM tracks WHERE id = $1::uuid`, req.TrackID).Scan(&song)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: unknown track", ErrInvalid)
	}
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO genre_corrections (track_id, genre) VALUES ($1::uuid, $2)
		ON CONFLICT (track_id) WHERE track_id IS NOT NULL
		DO UPDATE SET genre = EXCLUDED.genre, created_at = NOW()
	`, song, req.Genre)
	return err
}

// DeleteGenreCorrection removes a correction. The caller has the genres rescored.
func (s *Service) DeleteGenreCorrection(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM genre_corrections WHERE id = $1::uuid`, id)
	return err
}

// GenreCorrections lists every correction, newest first.
func (s *Service) GenreCorrections(ctx context.Context) ([]GenreCorrection, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id::text, c.track_id::text, t.title, a.name, c.artist, c.genre, c.created_at
		FROM genre_corrections c
		LEFT JOIN tracks t ON t.id = c.track_id
		LEFT JOIN artists a ON a.id = t.artist_id
		ORDER BY c.created_at DESC, c.id
	`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (GenreCorrection, error) {
		var c GenreCorrection
		err := row.Scan(&c.ID, &c.TrackID, &c.TrackTitle, &c.TrackArtist, &c.Artist, &c.Genre, &c.CreatedAt)
		return c, err
	})
}

// GenreReviewItem is a song whose main genre an admin may want to check.
type GenreReviewItem struct {
	TrackID string  `json:"track_id"`
	Title   string  `json:"title"`
	Artist  *string `json:"artist,omitempty"`
	Genre   string  `json:"genre"`
	Score   float32 `json:"score"`
	// HiddenBy is how many users hid the song on a genre station.
	HiddenBy int `json:"hidden_by"`
	// CopiesDisagree: another copy of the song has a different main genre.
	CopiesDisagree bool `json:"copies_disagree"`
}

// GenreReview lists the songs most worth checking: most often hidden on a
// genre station, then those whose copies disagree, then the lowest scores.
// Corrected songs are left out, also before scoring has run: confirming a
// genre takes a song off the list.
func (s *Service) GenreReview(ctx context.Context, limit int) ([]GenreReviewItem, error) {
	rows, err := s.pool.Query(ctx, `
		WITH hides AS (
			SELECT COALESCE(ht.copy_of, ht.id) AS song, COUNT(DISTINCT h.user_id)::int AS users
			FROM station_hidden_tracks h
			JOIN tracks ht ON ht.id = h.track_id
			JOIN stations st ON st.id = h.station_id AND st.auto_genre IS NOT NULL
			GROUP BY 1
		), disagree AS (
			SELECT DISTINCT c.copy_of AS song
			FROM tracks c
			JOIN track_station_genres cg ON cg.track_id = c.id AND cg.is_primary
			JOIN track_station_genres bg ON bg.track_id = c.copy_of AND bg.is_primary
			WHERE cg.genre <> bg.genre
		)
		SELECT t.id::text, t.title, a.name, g.genre, g.score,
		       COALESCE(h.users, 0), d.song IS NOT NULL
		FROM tracks t
		JOIN track_station_genres g ON g.track_id = t.id AND g.is_primary AND NOT g.corrected
		LEFT JOIN artists a ON a.id = t.artist_id
		LEFT JOIN hides h ON h.song = t.id
		LEFT JOIN disagree d ON d.song = t.id
		WHERE t.copy_of IS NULL
		  AND NOT EXISTS (SELECT 1 FROM genre_corrections c WHERE c.track_id = t.id)
		ORDER BY COALESCE(h.users, 0) DESC, d.song IS NOT NULL DESC, g.score, t.id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (GenreReviewItem, error) {
		var it GenreReviewItem
		err := row.Scan(&it.TrackID, &it.Title, &it.Artist, &it.Genre, &it.Score, &it.HiddenBy, &it.CopiesDisagree)
		return it, err
	})
}
