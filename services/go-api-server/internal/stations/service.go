package stations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ownwave/api/internal/auth"
)

// Service stores stations and works out their pools and suggestions.
type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// NotBanned is an SQL predicate on `tracks t` that drops the tracks the
// user (at placeholder userArg) banned.
func NotBanned(userArg string) string {
	return `NOT EXISTS (
		SELECT 1 FROM track_feedback b
		WHERE b.user_id = ` + userArg + `::uuid AND b.track_id = t.id AND b.feedback = 'ban')`
}

const stationColumns = `
	s.id::text, s.name, s.user_id IS NULL, s.genres, s.updated_at,
	COALESCE((SELECT array_agg(sp.playlist_id::text ORDER BY sp.playlist_id)
	          FROM station_playlists sp WHERE sp.station_id = s.id), '{}')`

func scanStation(row pgx.Row) (Station, error) {
	var st Station
	err := row.Scan(&st.ID, &st.Name, &st.Shared, &st.Genres, &st.UpdatedAt, &st.PlaylistIDs)
	return st, err
}

// List returns the user's own stations and the shared ones, shared first,
// each with its current track count for the user.
func (s *Service) List(ctx context.Context, userID string) ([]Station, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+stationColumns+` FROM stations s
		WHERE s.user_id = $1::uuid OR s.user_id IS NULL
		ORDER BY s.user_id IS NOT NULL, lower(s.name)
	`, userID)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Station, error) { return scanStation(row) })
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].TrackCount, err = s.PoolSize(ctx, userID, list[i].Definition()); err != nil {
			return nil, err
		}
	}
	return list, nil
}

// Get returns the station if the user may play it.
func (s *Service) Get(ctx context.Context, userID, id string) (Station, error) {
	st, err := scanStation(s.pool.QueryRow(ctx, `
		SELECT `+stationColumns+` FROM stations s
		WHERE s.id = $1::uuid AND (s.user_id = $2::uuid OR s.user_id IS NULL)
	`, id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return st, ErrNotFound
	}
	if err != nil {
		return st, err
	}
	st.TrackCount, err = s.PoolSize(ctx, userID, st.Definition())
	return st, err
}

// PoolSize counts the tracks the definition plays for the user (bans excluded).
func (s *Service) PoolSize(ctx context.Context, userID string, d Definition) (int, error) {
	args := Args{userID}
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM tracks t WHERE `+NotBanned("$1")+` AND `+d.Where(&args), args...).Scan(&n)
	return n, err
}

// checkExists returns ErrInvalid unless every playlist and genre exists.
func (s *Service) checkExists(ctx context.Context, q pgx.Tx, d Definition) error {
	var playlists, genres int
	if err := q.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM playlists WHERE id = ANY($1::uuid[])),
		       (SELECT COUNT(*) FROM station_genres WHERE is_station AND id = ANY($2::text[]))
	`, d.PlaylistIDs, d.Genres).Scan(&playlists, &genres); err != nil {
		return err
	}
	if playlists != len(d.PlaylistIDs) {
		return fmt.Errorf("%w: unknown playlist", ErrInvalid)
	}
	if genres != len(d.Genres) {
		return fmt.Errorf("%w: unknown genre", ErrInvalid)
	}
	return nil
}

func setPlaylists(ctx context.Context, tx pgx.Tx, stationID string, ids []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM station_playlists WHERE station_id = $1::uuid`, stationID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO station_playlists (station_id, playlist_id)
		SELECT $1::uuid, unnest($2::uuid[])
	`, stationID, ids)
	return err
}

// Create stores a new station. Admins create shared stations, other users
// personal ones.
func (s *Service) Create(ctx context.Context, user auth.User, req Request) (Station, error) {
	if req.Name == nil {
		return Station{}, fmt.Errorf("%w: name is required", ErrInvalid)
	}
	name, err := validName(*req.Name)
	if err != nil {
		return Station{}, err
	}
	def, err := Definition{PlaylistIDs: req.PlaylistIDs, Genres: req.Genres}.Normalize()
	if err != nil {
		return Station{}, err
	}
	owner := user.ID
	if user.IsAdmin {
		owner = ""
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Station{}, err
	}
	defer tx.Rollback(ctx)
	if err := s.checkExists(ctx, tx, def); err != nil {
		return Station{}, err
	}
	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO stations (user_id, name, genres) VALUES (NULLIF($1, '')::uuid, $2, $3)
		RETURNING id::text
	`, owner, name, def.Genres).Scan(&id); err != nil {
		return Station{}, err
	}
	if err := setPlaylists(ctx, tx, id, def.PlaylistIDs); err != nil {
		return Station{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Station{}, err
	}
	return s.Get(ctx, user.ID, id)
}

// Update changes a station's name and/or definition.
func (s *Service) Update(ctx context.Context, user auth.User, id string, req Request) (Station, error) {
	st, err := s.changeable(ctx, user, id)
	if err != nil {
		return Station{}, err
	}
	name := st.Name
	if req.Name != nil {
		if name, err = validName(*req.Name); err != nil {
			return Station{}, err
		}
	}
	def := st.Definition()
	if req.PlaylistIDs != nil {
		def.PlaylistIDs = req.PlaylistIDs
	}
	if req.Genres != nil {
		def.Genres = req.Genres
	}
	if def, err = def.Normalize(); err != nil {
		return Station{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Station{}, err
	}
	defer tx.Rollback(ctx)
	if err := s.checkExists(ctx, tx, def); err != nil {
		return Station{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE stations SET name = $2, genres = $3, updated_at = NOW() WHERE id = $1::uuid
	`, id, name, def.Genres); err != nil {
		return Station{}, err
	}
	if err := setPlaylists(ctx, tx, id, def.PlaylistIDs); err != nil {
		return Station{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Station{}, err
	}
	return s.Get(ctx, user.ID, id)
}

// Delete removes a station the user may change.
func (s *Service) Delete(ctx context.Context, user auth.User, id string) error {
	if _, err := s.changeable(ctx, user, id); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM stations WHERE id = $1::uuid`, id)
	return err
}

func (s *Service) changeable(ctx context.Context, user auth.User, id string) (Station, error) {
	st, err := s.Get(ctx, user.ID, id)
	if err != nil {
		return st, err
	}
	if st.Shared && !user.IsAdmin {
		return st, ErrForbidden
	}
	return st, nil
}

// UsedBy returns the names of the stations that play a playlist.
func (s *Service) UsedBy(ctx context.Context, playlistID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT s.name FROM stations s JOIN station_playlists sp ON sp.station_id = s.id
		WHERE sp.playlist_id = $1::uuid ORDER BY lower(s.name)
	`, playlistID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// Genre is a station genre as the analytics service defines it.
type Genre struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GenreCount is how many tracks of a pool carry a station genre.
type GenreCount struct {
	Genre
	Tracks int `json:"tracks"`
}

// PoolGenres is the genre breakdown of a playlist pool.
type PoolGenres struct {
	Tracks int          `json:"tracks"` // matched tracks in the pool
	Genres []GenreCount `json:"genres"` // most tracks first; a track counts once per genre
}

// GenreCounts breaks a playlist pool down by station genre, so a client
// can show what each genre filter would play.
func (s *Service) GenreCounts(ctx context.Context, playlistIDs []string) (PoolGenres, error) {
	def, err := Definition{PlaylistIDs: playlistIDs}.Normalize()
	if err != nil {
		return PoolGenres{}, err
	}
	data, err := s.loadPools(ctx)
	if err != nil {
		return PoolGenres{}, err
	}
	tracks := data.poolTracks(def.PlaylistIDs)
	out := PoolGenres{Tracks: len(tracks), Genres: []GenreCount{}}
	for _, g := range data.genres {
		if n := data.countGenre(tracks, g.ID); n > 0 {
			out.Genres = append(out.Genres, GenreCount{Genre: g, Tracks: n})
		}
	}
	sortGenreCounts(out.Genres)
	return out, nil
}

// Dismiss hides a suggestion (genre "" is the all-genres one) for good.
func (s *Service) Dismiss(ctx context.Context, playlistIDs []string, genre string) error {
	def, err := Definition{PlaylistIDs: playlistIDs}.Normalize()
	if err != nil {
		return err
	}
	if genre != "" && !genreIDPattern.MatchString(genre) {
		return fmt.Errorf("%w: unknown genre %q", ErrInvalid, genre)
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO station_suggestion_dismissals (pool_key, genre, dismissed_at) VALUES ($1, $2, $3)
		ON CONFLICT (pool_key, genre) DO NOTHING
	`, PoolKey(def.PlaylistIDs), genre, time.Now())
	return err
}
