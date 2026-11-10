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

// sameSong is an SQL predicate: tracks a and b are copies of one song.
func sameSong(a, b string) string {
	return `COALESCE(` + a + `.copy_of, ` + a + `.id) = COALESCE(` + b + `.copy_of, ` + b + `.id)`
}

// NotBanned is an SQL predicate on `tracks t` that drops the songs the
// user (at placeholder userArg) banned, through any of their copies.
func NotBanned(userArg string) string {
	return `NOT EXISTS (
		SELECT 1 FROM track_feedback b JOIN tracks bt ON bt.id = b.track_id
		WHERE b.user_id = ` + userArg + `::uuid AND b.feedback = 'ban' AND ` + sameSong("bt", "t") + `)`
}

// Liked is an SQL predicate on `tracks t`: the user (at placeholder
// userArg) liked this song, through any of its copies.
func Liked(userArg string) string {
	return `EXISTS (
		SELECT 1 FROM track_feedback l JOIN tracks lt ON lt.id = l.track_id
		WHERE l.user_id = ` + userArg + `::uuid AND l.feedback = 'like' AND ` + sameSong("lt", "t") + `)`
}

// NotHidden is an SQL predicate on `tracks t` that drops the songs the
// user hid on the station (at placeholders userArg and stationArg).
func NotHidden(userArg, stationArg string) string {
	return `NOT EXISTS (
		SELECT 1 FROM station_hidden_tracks h JOIN tracks ht ON ht.id = h.track_id
		WHERE h.user_id = ` + userArg + `::uuid AND h.station_id = ` + stationArg + `::uuid AND ` + sameSong("ht", "t") + `)`
}

// stationColumns reads a station; an all-playlists station lists every
// playlist there is now.
const stationColumns = `
	s.id::text, s.name, s.user_id IS NULL, s.auto_genre IS NOT NULL, s.all_playlists, s.genres, s.enhance, s.updated_at,
	CASE WHEN s.all_playlists
	     THEN COALESCE((SELECT array_agg(p.id::text ORDER BY p.id) FROM playlists p), '{}')
	     ELSE COALESCE((SELECT array_agg(sp.playlist_id::text ORDER BY sp.playlist_id)
	                    FROM station_playlists sp WHERE sp.station_id = s.id), '{}')
	END`

// visibleTo is an SQL predicate on `stations s`: the station is the
// user's (at placeholder userArg) or shared, and not a hidden genre station.
func visibleTo(userArg string) string {
	return `(s.user_id = ` + userArg + `::uuid OR s.user_id IS NULL) AND (s.auto_genre IS NULL OR s.auto_active)`
}

func scanStation(row pgx.Row) (Station, error) {
	var st Station
	var genre bool
	err := row.Scan(&st.ID, &st.Name, &st.Shared, &genre, &st.AllPlaylists, &st.Genres, &st.Enhance, &st.UpdatedAt, &st.PlaylistIDs)
	st.Kind = KindCustom
	if genre {
		st.Kind = KindGenre
	}
	return st, err
}

// List returns the user's own stations and the shared ones (genre
// stations only while active), shared first, each with its current track
// count for the user.
func (s *Service) List(ctx context.Context, userID string) ([]Station, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+stationColumns+` FROM stations s
		WHERE `+visibleTo("$1")+`
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
		if list[i].TrackCount, err = s.PoolSize(ctx, userID, list[i].ID, list[i].Definition()); err != nil {
			return nil, err
		}
	}
	return list, nil
}

// Get returns the station if the user may play it. A hidden genre station
// is not found.
func (s *Service) Get(ctx context.Context, userID, id string) (Station, error) {
	st, err := scanStation(s.pool.QueryRow(ctx, `
		SELECT `+stationColumns+` FROM stations s
		WHERE s.id = $1::uuid AND `+visibleTo("$2"), id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return st, ErrNotFound
	}
	if err != nil {
		return st, err
	}
	st.TrackCount, err = s.PoolSize(ctx, userID, st.ID, st.Definition())
	return st, err
}

// PoolSize counts the songs the station plays for the user (bans and hides
// excluded).
func (s *Service) PoolSize(ctx context.Context, userID, stationID string, d Definition) (int, error) {
	args := Args{userID, stationID}
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM tracks t
		WHERE `+NotBanned("$1")+` AND `+NotHidden("$1", "$2")+` AND `+d.Where(&args), args...).Scan(&n)
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

// Create stores a new custom station. Admins create shared stations, other
// users personal ones.
func (s *Service) Create(ctx context.Context, user auth.User, req Request) (Station, error) {
	if req.Name == nil {
		return Station{}, fmt.Errorf("%w: name is required", ErrInvalid)
	}
	name, err := validName(*req.Name)
	if err != nil {
		return Station{}, err
	}
	def, err := Definition{
		AllPlaylists: req.AllPlaylists != nil && *req.AllPlaylists,
		PlaylistIDs:  req.PlaylistIDs,
		Genres:       req.Genres,
		Enhance:      req.Enhance != nil && *req.Enhance,
	}.Normalize()
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
		INSERT INTO stations (user_id, name, genres, all_playlists, enhance) VALUES (NULLIF($1, '')::uuid, $2, $3, $4, $5)
		RETURNING id::text
	`, owner, name, def.Genres, def.AllPlaylists, def.Enhance).Scan(&id); err != nil {
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

// Update changes a custom station's name and/or definition. A genre
// station only changes its name, which the refresh then keeps.
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
	if st.Kind == KindGenre {
		if req.AllPlaylists != nil || req.PlaylistIDs != nil || req.Genres != nil || req.Enhance != nil {
			return Station{}, fmt.Errorf("%w: a genre station only changes its name", ErrInvalid)
		}
		if req.Name != nil {
			if err := s.renameGenreStation(ctx, id, name); err != nil {
				return Station{}, err
			}
		}
		return s.Get(ctx, user.ID, id)
	}

	def := st.Definition()
	if req.AllPlaylists != nil {
		def.AllPlaylists = *req.AllPlaylists
	}
	if req.PlaylistIDs != nil {
		def.PlaylistIDs = req.PlaylistIDs
	}
	if req.Genres != nil {
		def.Genres = req.Genres
	}
	if req.Enhance != nil {
		def.Enhance = *req.Enhance
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
		UPDATE stations SET name = $2, genres = $3, all_playlists = $4, enhance = $5, updated_at = NOW() WHERE id = $1::uuid
	`, id, name, def.Genres, def.AllPlaylists, def.Enhance); err != nil {
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

// Delete removes a custom station the user may change. Genre stations are
// not deleted (the next refresh would bring them back): exclude the genre.
func (s *Service) Delete(ctx context.Context, user auth.User, id string) error {
	st, err := s.changeable(ctx, user, id)
	if err != nil {
		return err
	}
	if st.Kind == KindGenre {
		return fmt.Errorf("%w: genre stations are not deleted; exclude the genre in Admin instead", ErrConflict)
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM stations WHERE id = $1::uuid`, id)
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

// UsedBy returns the names of the stations that list a playlist by id.
// All-playlists stations hold on to no single playlist.
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
	Tracks int          `json:"tracks"` // the playlists' matched tracks, without library picks
	Genres []GenreCount `json:"genres"` // most tracks first; a track counts once per genre
}

// GenreCounts breaks a playlist pool down by station genre, so a client
// can show what each genre filter would play. allPlaylists counts every
// playlist and ignores playlistIDs.
func (s *Service) GenreCounts(ctx context.Context, playlistIDs []string, allPlaylists bool) (PoolGenres, error) {
	def, err := Definition{PlaylistIDs: playlistIDs, AllPlaylists: allPlaylists}.Normalize()
	if err != nil {
		return PoolGenres{}, err
	}
	data, err := s.loadPools(ctx)
	if err != nil {
		return PoolGenres{}, err
	}
	ids := def.PlaylistIDs
	if def.AllPlaylists {
		ids = data.allPlaylistIDs()
	}
	tracks := data.poolTracks(ids)
	out := PoolGenres{Tracks: len(tracks), Genres: []GenreCount{}}
	for _, g := range data.genres {
		if n := data.countGenre(tracks, g.ID); n > 0 {
			out.Genres = append(out.Genres, GenreCount{Genre: g, Tracks: n})
		}
	}
	sortGenreCounts(out.Genres)
	return out, nil
}

// Dismiss hides a single-playlist suggestion (genre "" is the all-genres
// one) for good.
func (s *Service) Dismiss(ctx context.Context, playlistIDs []string, genre string) error {
	def, err := Definition{PlaylistIDs: playlistIDs}.Normalize()
	if err != nil {
		return err
	}
	if len(def.PlaylistIDs) != 1 {
		return fmt.Errorf("%w: suggestions are for one playlist", ErrInvalid)
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
