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

// GenreCorrection is an admin's genre for a song, for all songs of a release
// or for all of an artist's songs (CONTEXT.md: Correction). The genre engine
// in the analytics worker applies it: it beats every other source, the most
// specific one wins, and it becomes a trusted label for the classifier.
type GenreCorrection struct {
	ID string `json:"id"`
	// Scope is "song", "release" or "artist".
	Scope string `json:"scope"`
	// TrackID, TrackTitle and TrackArtist are set on a song's correction.
	TrackID     *string `json:"track_id,omitempty"`
	TrackTitle  *string `json:"track_title,omitempty"`
	TrackArtist *string `json:"track_artist,omitempty"`
	// ReleaseID, ReleaseTitle and ReleaseArtist are set on a release's correction.
	ReleaseID     *string `json:"release_id,omitempty"`
	ReleaseTitle  *string `json:"release_title,omitempty"`
	ReleaseArtist *string `json:"release_artist,omitempty"`
	// Artist is set on an artist's correction.
	Artist *string `json:"artist,omitempty"`
	// Genre is the corrected genre; Style is set when the correction names a
	// style of that genre (stored as the style's id).
	Genre     string    `json:"genre"`
	Style     *string   `json:"style,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// GenreCorrectionRequest corrects one song (TrackID), every song of a
// release (ReleaseID) or of an artist (Artist), to Genre and optionally to
// one of its styles.
type GenreCorrectionRequest struct {
	TrackID   string `json:"track_id"`
	ReleaseID string `json:"release_id"`
	Artist    string `json:"artist"`
	Genre     string `json:"genre"`
	Style     string `json:"style"`
}

// genreV2Switched reports whether the genre v2 switch migration is in:
// station_genres then has kinds (genres and styles). It is the check the
// analytics worker uses too (genre_engine.switched); until then the v1
// blend scores genres and only song and artist corrections exist for it.
func (s *Service) genreV2Switched(ctx context.Context) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM information_schema.columns
		               WHERE table_schema = current_schema()
		                 AND table_name = 'station_genres' AND column_name = 'kind')
	`).Scan(&ok)
	return ok, err
}

// correctionValue checks a correction's genre and style and returns the id
// to store: the style's when one is named, else the genre's.
func (s *Service) correctionValue(ctx context.Context, switched bool, genre, style string) (string, error) {
	var known bool
	query := `SELECT EXISTS (SELECT 1 FROM station_genres WHERE is_station AND id = $1)`
	if switched {
		query = `SELECT EXISTS (SELECT 1 FROM station_genres WHERE kind = 'genre' AND id = $1)`
	}
	if err := s.pool.QueryRow(ctx, query, genre).Scan(&known); err != nil {
		return "", err
	}
	if !known {
		return "", fmt.Errorf("%w: unknown genre %q", ErrInvalid, genre)
	}
	if style == "" {
		return genre, nil
	}
	if !switched {
		return "", fmt.Errorf("%w: styles come with genre engine v2", ErrInvalid)
	}
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM station_genres WHERE kind = 'style' AND id = $1 AND parent_id = $2)
	`, style, genre).Scan(&known); err != nil {
		return "", err
	}
	if !known {
		return "", fmt.Errorf("%w: %q is not a style of %q", ErrInvalid, style, genre)
	}
	return style, nil
}

// SaveGenreCorrection stores or replaces a correction and returns its id.
// The caller has the genres rescored.
func (s *Service) SaveGenreCorrection(ctx context.Context, req GenreCorrectionRequest) (string, error) {
	artist := strings.TrimSpace(req.Artist)
	targets := 0
	for _, v := range []string{req.TrackID, req.ReleaseID, artist} {
		if v != "" {
			targets++
		}
	}
	if targets != 1 {
		return "", fmt.Errorf("%w: correct exactly one of a song, a release or an artist", ErrInvalid)
	}
	switched, err := s.genreV2Switched(ctx)
	if err != nil {
		return "", err
	}
	value, err := s.correctionValue(ctx, switched, req.Genre, req.Style)
	if err != nil {
		return "", err
	}
	var id string
	switch {
	case artist != "":
		err = s.pool.QueryRow(ctx, `
			INSERT INTO genre_corrections (artist, genre) VALUES ($1, $2)
			ON CONFLICT (lower(artist)) WHERE artist IS NOT NULL
			DO UPDATE SET artist = EXCLUDED.artist, genre = EXCLUDED.genre, created_at = NOW()
			RETURNING id::text
		`, artist, value).Scan(&id)
	case req.ReleaseID != "":
		// The v1 blend has no release scope; it would silently skip one.
		if !switched {
			return "", fmt.Errorf("%w: release corrections come with genre engine v2", ErrInvalid)
		}
		if _, err := uuid.Parse(req.ReleaseID); err != nil {
			return "", fmt.Errorf("%w: release_id must be a valid id", ErrInvalid)
		}
		err = s.pool.QueryRow(ctx, `
			INSERT INTO genre_corrections (release_id, genre)
			SELECT id, $2 FROM releases WHERE id = $1::uuid
			ON CONFLICT (release_id) WHERE release_id IS NOT NULL
			DO UPDATE SET genre = EXCLUDED.genre, created_at = NOW()
			RETURNING id::text
		`, req.ReleaseID, value).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("%w: unknown release", ErrInvalid)
		}
	default:
		if _, err := uuid.Parse(req.TrackID); err != nil {
			return "", fmt.Errorf("%w: track_id must be a valid id", ErrInvalid)
		}
		// A song's correction is stored on the song (its best copy), so it
		// holds for every copy.
		var song string
		err = s.pool.QueryRow(ctx, `SELECT song_id::text FROM songs WHERE track_id = $1::uuid`, req.TrackID).Scan(&song)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("%w: unknown track", ErrInvalid)
		}
		if err != nil {
			return "", err
		}
		err = s.pool.QueryRow(ctx, `
			INSERT INTO genre_corrections (track_id, genre) VALUES ($1::uuid, $2)
			ON CONFLICT (track_id) WHERE track_id IS NOT NULL
			DO UPDATE SET genre = EXCLUDED.genre, created_at = NOW()
			RETURNING id::text
		`, song, value).Scan(&id)
	}
	return id, err
}

// DeleteGenreCorrection removes a correction. The caller has the genres rescored.
func (s *Service) DeleteGenreCorrection(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM genre_corrections WHERE id = $1::uuid`, id)
	return err
}

// GenreCorrections lists the corrections, newest first. A non-empty search
// keeps those whose song, release, artist or genre (id or name) contains it.
func (s *Service) GenreCorrections(ctx context.Context, search string) ([]GenreCorrection, error) {
	switched, err := s.genreV2Switched(ctx)
	if err != nil {
		return nil, err
	}
	// After the switch a correction may hold a style id; report its genre
	// and the style apart.
	genre, style := `c.genre`, `NULL::text`
	if switched {
		genre = `COALESCE(sg.parent_id, c.genre)`
		style = `CASE WHEN sg.kind = 'style' THEN c.genre END`
	}
	rows, err := s.pool.Query(ctx, `
		SELECT c.id::text,
		       CASE WHEN c.track_id IS NOT NULL THEN 'song' WHEN c.release_id IS NOT NULL THEN 'release' ELSE 'artist' END,
		       c.track_id::text, t.title, a.name,
		       c.release_id::text, r.title, r.album_artist,
		       c.artist, `+genre+`, `+style+`, c.created_at
		FROM genre_corrections c
		LEFT JOIN tracks t ON t.id = c.track_id
		LEFT JOIN artists a ON a.id = t.artist_id
		LEFT JOIN releases r ON r.id = c.release_id
		LEFT JOIN station_genres sg ON sg.id = c.genre
		LEFT JOIN station_genres pg ON pg.id = `+genre+`
		WHERE $1 = '' OR concat_ws(' ', t.title, a.name, r.title, r.album_artist, c.artist,
		                           c.genre, sg.name, pg.id, pg.name) ILIKE '%' || $1 || '%'
		ORDER BY c.created_at DESC, c.id
	`, escapeLike(strings.TrimSpace(search)))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (GenreCorrection, error) {
		var c GenreCorrection
		err := row.Scan(&c.ID, &c.Scope, &c.TrackID, &c.TrackTitle, &c.TrackArtist,
			&c.ReleaseID, &c.ReleaseTitle, &c.ReleaseArtist, &c.Artist, &c.Genre, &c.Style, &c.CreatedAt)
		return c, err
	})
}

// escapeLike makes s match literally inside an ILIKE pattern.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
