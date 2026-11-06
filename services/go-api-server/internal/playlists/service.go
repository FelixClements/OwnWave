// Package playlists syncs Spotify playlists into the database and matches
// their tracks to local files. Playlists only grow: a sync adds the tracks
// Spotify lists now and never removes the ones it rotated out.
package playlists

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ownwave/api/internal/spotify"
)

// ErrNotFound is returned for an unknown playlist id.
var ErrNotFound = errors.New("playlist not found")

// Fetcher loads a playlist's current tracks and its cover from Spotify.
type Fetcher interface {
	FetchPlaylist(ctx context.Context, spotifyID string) (spotify.Playlist, error)
	FetchImage(ctx context.Context, url string) (body []byte, mediaType string, err error)
}

type Service struct {
	pool    *pgxpool.Pool
	spotify Fetcher
}

func NewService(pool *pgxpool.Pool, fetcher Fetcher) *Service {
	return &Service{pool: pool, spotify: fetcher}
}

type Playlist struct {
	ID           string `json:"id"`
	SpotifyID    string `json:"spotify_id"`
	Name         string `json:"name"`
	TrackCount   int    `json:"track_count"`
	MatchedCount int    `json:"matched_count"`
	// PickCount is how many library songs its stations play on top of the matches.
	PickCount     int        `json:"pick_count"`
	LastSyncedAt  *time.Time `json:"last_synced_at,omitempty"`
	LastSyncError *string    `json:"last_sync_error,omitempty"`
	// HasCover is true once the playlist's Spotify cover is downloaded.
	HasCover bool `json:"has_cover"`
}

type Item struct {
	SpotifyTrackID string    `json:"spotify_track_id"`
	Title          string    `json:"title"`
	Artists        []string  `json:"artists"`
	DurationMS     *int      `json:"duration_ms,omitempty"`
	FirstSeenAt    time.Time `json:"first_seen_at"`
	LastSeenAt     time.Time `json:"last_seen_at"`
	TrackID        *string   `json:"track_id,omitempty"`
	LocalTitle     *string   `json:"local_title,omitempty"`
	LocalArtist    *string   `json:"local_artist,omitempty"`
	MatchMethod    *string   `json:"match_method,omitempty"`
}

const playlistColumns = `
	p.id::text, p.spotify_id, p.name,
	(SELECT COUNT(*) FROM playlist_items pi WHERE pi.playlist_id = p.id),
	(SELECT COUNT(*) FROM playlist_items pi
	 JOIN spotify_track_matches m ON m.spotify_track_id = pi.spotify_track_id
	 WHERE pi.playlist_id = p.id AND m.track_id IS NOT NULL),
	(SELECT COUNT(*) FROM playlist_library_picks lp WHERE lp.playlist_id = p.id),
	p.last_synced_at, p.last_sync_error, p.cover_image IS NOT NULL`

func scanPlaylist(row pgx.Row) (Playlist, error) {
	var p Playlist
	err := row.Scan(&p.ID, &p.SpotifyID, &p.Name, &p.TrackCount, &p.MatchedCount, &p.PickCount, &p.LastSyncedAt, &p.LastSyncError, &p.HasCover)
	return p, err
}

func (s *Service) List(ctx context.Context) ([]Playlist, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+playlistColumns+` FROM playlists p ORDER BY lower(p.name)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Playlist, 0)
	for rows.Next() {
		p, err := scanPlaylist(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Service) Get(ctx context.Context, id string) (Playlist, error) {
	p, err := scanPlaylist(s.pool.QueryRow(ctx, `SELECT `+playlistColumns+` FROM playlists p WHERE p.id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

// ItemFilter selects playlist items by match status.
type ItemFilter string

const (
	AllItems     ItemFilter = ""
	MatchedItems ItemFilter = "matched"
	// MissingItems are the tracks with no local file: the playlist's missing list.
	MissingItems ItemFilter = "missing"
)

// Items lists every track the playlist has ever had, newest first,
// optionally only the matched or only the missing ones.
func (s *Service) Items(ctx context.Context, id string, filter ItemFilter) ([]Item, error) {
	status := ""
	switch filter {
	case MatchedItems:
		status = "AND m.track_id IS NOT NULL"
	case MissingItems:
		status = "AND m.track_id IS NULL"
	}
	rows, err := s.pool.Query(ctx, `
		SELECT pi.spotify_track_id, pi.title, pi.artists, pi.duration_ms, pi.first_seen_at, pi.last_seen_at,
		       m.track_id::text, t.title, a.name, m.method
		FROM playlist_items pi
		LEFT JOIN spotify_track_matches m ON m.spotify_track_id = pi.spotify_track_id
		LEFT JOIN tracks t ON t.id = m.track_id
		LEFT JOIN artists a ON a.id = t.artist_id
		WHERE pi.playlist_id = $1::uuid `+status+`
		ORDER BY pi.first_seen_at DESC, pi.title
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Item, 0)
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.SpotifyTrackID, &it.Title, &it.Artists, &it.DurationMS, &it.FirstSeenAt, &it.LastSeenAt,
			&it.TrackID, &it.LocalTitle, &it.LocalArtist, &it.MatchMethod); err != nil {
			return nil, err
		}
		if it.TrackID == nil {
			it.MatchMethod = nil
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// Add fetches a Spotify playlist and stores it (or re-syncs it when it is
// already known), then matches its tracks.
func (s *Service) Add(ctx context.Context, url string) (Playlist, error) {
	spotifyID, err := spotify.ParsePlaylistID(url)
	if err != nil {
		return Playlist{}, err
	}
	fetched, err := s.spotify.FetchPlaylist(ctx, spotifyID)
	if err != nil {
		return Playlist{}, err
	}
	var id string
	err = s.pool.QueryRow(ctx, `
		INSERT INTO playlists (spotify_id, name) VALUES ($1, $2)
		ON CONFLICT (spotify_id) DO UPDATE SET spotify_id = EXCLUDED.spotify_id
		RETURNING id::text
	`, spotifyID, fetched.Name).Scan(&id)
	if err != nil {
		return Playlist{}, err
	}
	if err := s.store(ctx, id, fetched); err != nil {
		return Playlist{}, err
	}
	if _, err := s.MatchPending(ctx); err != nil {
		return Playlist{}, err
	}
	return s.Get(ctx, id)
}

// Sync adds the playlist's current Spotify tracks and matches new ones. A
// failed fetch is recorded on the playlist and changes nothing else.
func (s *Service) Sync(ctx context.Context, id string) (Playlist, error) {
	if err := s.syncOne(ctx, id); err != nil {
		return Playlist{}, err
	}
	if _, err := s.MatchPending(ctx); err != nil {
		return Playlist{}, err
	}
	return s.Get(ctx, id)
}

func (s *Service) syncOne(ctx context.Context, id string) error {
	var spotifyID string
	err := s.pool.QueryRow(ctx, `SELECT spotify_id FROM playlists WHERE id = $1::uuid`, id).Scan(&spotifyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	fetched, err := s.spotify.FetchPlaylist(ctx, spotifyID)
	if err != nil {
		if _, dbErr := s.pool.Exec(ctx, `UPDATE playlists SET last_sync_error = $2 WHERE id = $1::uuid`, id, err.Error()); dbErr != nil {
			return dbErr
		}
		return err
	}
	return s.store(ctx, id, fetched)
}

// store adds fetched tracks to the playlist; known tracks only get their
// details and last_seen_at refreshed.
func (s *Service) store(ctx context.Context, id string, p spotify.Playlist) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	batch := &pgx.Batch{}
	for _, t := range p.Tracks {
		batch.Queue(`
			INSERT INTO playlist_items (playlist_id, spotify_track_id, title, artists, duration_ms)
			VALUES ($1::uuid, $2, $3, $4, NULLIF($5, 0))
			ON CONFLICT (playlist_id, spotify_track_id) DO UPDATE SET
				title = EXCLUDED.title, artists = EXCLUDED.artists,
				duration_ms = EXCLUDED.duration_ms, last_seen_at = NOW()
		`, id, t.ID, t.Title, t.Artists, t.DurationMS)
	}
	batch.Queue(`
		UPDATE playlists SET name = $2, last_synced_at = NOW(), last_sync_error = NULL
		WHERE id = $1::uuid
	`, id, p.Name)
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return s.refreshCover(ctx, id, p.CoverURL)
}

// refreshCover downloads the playlist's cover when Spotify lists one the
// playlist does not have yet. A failed download keeps the old cover (and
// is retried next sync): the tracks matter, the cover does not.
func (s *Service) refreshCover(ctx context.Context, id, coverURL string) error {
	if coverURL == "" {
		return nil
	}
	var current *string
	var has bool
	if err := s.pool.QueryRow(ctx, `SELECT cover_url, cover_image IS NOT NULL FROM playlists WHERE id = $1::uuid`, id).
		Scan(&current, &has); err != nil {
		return err
	}
	if has && current != nil && *current == coverURL {
		return nil
	}
	img, mediaType, err := s.spotify.FetchImage(ctx, coverURL)
	if err != nil {
		slog.Warn("playlist cover download failed", "playlist_id", id, "error", err)
		return nil
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE playlists SET cover_url = $2, cover_image = $3, cover_type = $4 WHERE id = $1::uuid
	`, id, coverURL, img, mediaType)
	return err
}

// Cover returns the playlist's downloaded cover and its media type, or a
// nil image when it has none.
func (s *Service) Cover(ctx context.Context, id string) (img []byte, mediaType string, err error) {
	var kind *string
	err = s.pool.QueryRow(ctx, `SELECT cover_image, cover_type FROM playlists WHERE id = $1::uuid`, id).Scan(&img, &kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil || len(img) == 0 {
		return nil, "", err
	}
	mediaType = "image/jpeg"
	if kind != nil && *kind != "" {
		mediaType = *kind
	}
	return img, mediaType, nil
}

// MatchPending tries to match every playlist track without a local file,
// e.g. new playlist tracks or files added since the last attempt.
func (s *Service) MatchPending(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (pi.spotify_track_id) pi.spotify_track_id, pi.title, pi.artists, COALESCE(pi.duration_ms, 0)
		FROM playlist_items pi
		LEFT JOIN spotify_track_matches m ON m.spotify_track_id = pi.spotify_track_id
		WHERE m.track_id IS NULL
		ORDER BY pi.spotify_track_id, pi.last_seen_at DESC
	`)
	if err != nil {
		return 0, err
	}
	pending, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (spotify.Track, error) {
		var t spotify.Track
		err := row.Scan(&t.ID, &t.Title, &t.Artists, &t.DurationMS)
		return t, err
	})
	if err != nil || len(pending) == 0 {
		return 0, err
	}

	local, err := loadLocal(ctx, s.pool)
	if err != nil {
		return 0, err
	}

	m := NewMatcher(local)
	matched := 0
	batch := &pgx.Batch{}
	for _, t := range pending {
		trackID, method, ok := m.Match(t)
		if ok {
			matched++
		}
		batch.Queue(`
			INSERT INTO spotify_track_matches (spotify_track_id, track_id, method)
			VALUES ($1, NULLIF($2, '')::uuid, $3)
			ON CONFLICT (spotify_track_id) DO UPDATE SET
				track_id = EXCLUDED.track_id, method = EXCLUDED.method, matched_at = NOW()
		`, t.ID, trackID, method)
	}
	if err := s.pool.SendBatch(ctx, batch).Close(); err != nil {
		return 0, err
	}
	return matched, nil
}

// SyncStale syncs every playlist not synced within maxAge, then matches
// all unmatched tracks, so files added by a scan are found even when no
// playlist is due. Spotify rotates editorial playlists every week or two,
// so tracks that come and go between two syncs are never seen: run this
// often enough.
func (s *Service) SyncStale(ctx context.Context, maxAge time.Duration) error {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text FROM playlists
		WHERE last_synced_at IS NULL OR last_synced_at < NOW() - make_interval(secs => $1)
		ORDER BY last_synced_at NULLS FIRST
	`, maxAge.Seconds())
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.syncOne(ctx, id); err != nil {
			slog.Warn("playlist sync failed", "playlist_id", id, "error", err)
		}
	}
	_, err = s.MatchPending(ctx)
	return err
}

// Delete removes a playlist unless a station still uses it; it then
// returns the names of those stations.
func (s *Service) Delete(ctx context.Context, id string) (usedBy []string, err error) {
	rows, err := s.pool.Query(ctx, `
		SELECT s.name FROM stations s JOIN station_playlists sp ON sp.station_id = s.id
		WHERE sp.playlist_id = $1::uuid ORDER BY lower(s.name)
	`, id)
	if err != nil {
		return nil, err
	}
	if usedBy, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil || len(usedBy) > 0 {
		return usedBy, err
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM playlists WHERE id = $1::uuid`, id)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return nil, nil
}
