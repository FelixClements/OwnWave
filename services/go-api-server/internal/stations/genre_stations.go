package stations

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Genre stations: one shared station per station genre, playing every song
// of the library with that main genre. Refresh creates, shows and hides them; it
// never deletes one, so a station keeps its id, name and history while
// its genre is below the threshold.
const (
	// GenreStationShowAt is how many songs a genre needs before its
	// station is shown.
	GenreStationShowAt = MinSuggestionTracks
	// GenreStationHideBelow: a shown station is hidden only below this, so
	// a genre near the threshold does not flicker in and out.
	GenreStationHideBelow = 20
)

// refreshLock is the advisory lock that serialises refreshes across
// requests, the background runner and API instances.
const refreshLock = 4_242_101

const autoStationsKey = "auto_stations"

// Change kinds reported by Refresh.
const (
	ChangeCreated = "created" // the genre's station was made (and shown)
	ChangeShown   = "shown"   // a hidden station came back
	ChangeHidden  = "hidden"  // below the threshold, excluded, or switched off
	ChangeRenamed = "renamed" // the station took the genre's new display name
)

// Change is one thing a refresh did to a genre station.
type Change struct {
	Genre  string `json:"genre"`
	Name   string `json:"name"`
	Change string `json:"change"`
}

// Genre station states, as GenreStations reports them.
const (
	StateActive         = "active"
	StateBelowThreshold = "below_threshold"
	StateExcluded       = "excluded"
	StateOff            = "off" // genre stations are switched off
)

// GenreStation is a station genre and its genre station, if it has one.
type GenreStation struct {
	Genre     string `json:"genre"`
	GenreName string `json:"genre_name"`
	// StationID and Name are empty until the genre first reaches the threshold.
	StationID    string `json:"station_id,omitempty"`
	Name         string `json:"name"`
	NameIsCustom bool   `json:"name_is_custom"`
	Tracks       int    `json:"tracks"`
	State        string `json:"state"`
}

// GenreStationSettings is the admin view of genre stations.
type GenreStationSettings struct {
	Enabled   bool           `json:"enabled"`
	ShowAt    int            `json:"show_at"`
	HideBelow int            `json:"hide_below"`
	Genres    []GenreStation `json:"genres"`
}

type autoStation struct {
	id, genre, name     string
	nameIsCustom        bool
	active, excluded    bool
	stillAStationGenre  bool
	tracks              int
	stationGenreName    string
	stationGenreOrdinal int
}

type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func autoEnabled(ctx context.Context, q queryer) (bool, error) {
	var enabled *bool
	err := q.QueryRow(ctx, `SELECT (value->>'enabled')::boolean FROM app_state WHERE key = $1`, autoStationsKey).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && enabled == nil) {
		return true, nil // on by default
	}
	return err == nil && *enabled, err
}

// genreTrackCounts counts, per station genre, the songs of the library with
// that main genre, as genre stations play them.
func genreTrackCounts(ctx context.Context, q queryer) (map[string]int, error) {
	rows, err := q.Query(ctx, `
		SELECT g.genre, COUNT(*) FROM track_station_genres g
		JOIN tracks t ON t.id = g.track_id
		WHERE g.is_primary AND t.copy_of IS NULL
		GROUP BY g.genre`)
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	var g string
	var n int
	_, err = pgx.ForEachRow(rows, []any{&g, &n}, func() error { out[g] = n; return nil })
	return out, err
}

// loadGenreStations returns every station genre with its station (if any)
// in display order, plus stations whose genre is no longer a station genre.
func loadGenreStations(ctx context.Context, q queryer) ([]autoStation, error) {
	counts, err := genreTrackCounts(ctx, q)
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, `
		SELECT COALESCE(g.id, s.auto_genre), COALESCE(s.id::text, ''), COALESCE(s.name, ''),
		       COALESCE(s.name_is_custom, FALSE), COALESCE(s.auto_active, FALSE), COALESCE(s.auto_excluded, FALSE),
		       g.id IS NOT NULL, COALESCE(g.name, ''), COALESCE(g.position, 2147483647)
		FROM (SELECT id, name, position FROM station_genres WHERE is_station) g
		FULL JOIN (SELECT * FROM stations WHERE auto_genre IS NOT NULL) s ON s.auto_genre = g.id
		ORDER BY 9, 1`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (autoStation, error) {
		var a autoStation
		err := row.Scan(&a.genre, &a.id, &a.name, &a.nameIsCustom, &a.active, &a.excluded,
			&a.stillAStationGenre, &a.stationGenreName, &a.stationGenreOrdinal)
		a.tracks = counts[a.genre]
		return a, err
	})
	return list, err
}

// shouldShow decides whether a genre station is shown.
func (a autoStation) shouldShow(enabled bool) bool {
	if !enabled || a.excluded || !a.stillAStationGenre {
		return false
	}
	if a.active {
		return a.tracks >= GenreStationHideBelow
	}
	return a.tracks >= GenreStationShowAt
}

// Refresh brings the genre stations in line with the library: it creates
// the station of every genre that reached GenreStationShowAt, shows and
// hides stations, and gives stations without a custom name their genre's
// display name. Their track pools need no refresh (they are evaluated
// live). Running it again changes nothing.
func (s *Service) Refresh(ctx context.Context) ([]Change, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, refreshLock); err != nil {
		return nil, err
	}
	enabled, err := autoEnabled(ctx, tx)
	if err != nil {
		return nil, err
	}
	list, err := loadGenreStations(ctx, tx)
	if err != nil {
		return nil, err
	}

	changes := []Change{}
	for _, a := range list {
		show := a.shouldShow(enabled)
		switch {
		case a.id == "" && show:
			// ON CONFLICT: the unique index is the last guard against a duplicate.
			tag, err := tx.Exec(ctx, `
				INSERT INTO stations (user_id, name, genres, auto_genre, auto_active)
				VALUES (NULL, $1, ARRAY[$2], $2, TRUE)
				ON CONFLICT (auto_genre) WHERE auto_genre IS NOT NULL DO NOTHING
			`, a.stationGenreName, a.genre)
			if err != nil {
				return nil, err
			}
			if tag.RowsAffected() > 0 {
				changes = append(changes, Change{Genre: a.genre, Name: a.stationGenreName, Change: ChangeCreated})
			}
			continue
		case a.id == "":
			continue
		}
		name := a.name
		if !a.nameIsCustom && a.stillAStationGenre && a.stationGenreName != "" && a.name != a.stationGenreName {
			name = a.stationGenreName
			changes = append(changes, Change{Genre: a.genre, Name: name, Change: ChangeRenamed})
		}
		if show != a.active {
			kind := ChangeHidden
			if show {
				kind = ChangeShown
			}
			changes = append(changes, Change{Genre: a.genre, Name: name, Change: kind})
		}
		if show != a.active || name != a.name {
			if _, err := tx.Exec(ctx, `
				UPDATE stations SET name = $2, auto_active = $3, updated_at = NOW() WHERE id = $1::uuid
			`, a.id, name, show); err != nil {
				return nil, err
			}
		}
	}
	return changes, tx.Commit(ctx)
}

// GenreStations reports every station genre, its track count and what its
// genre station is doing.
func (s *Service) GenreStations(ctx context.Context) (GenreStationSettings, error) {
	enabled, err := autoEnabled(ctx, s.pool)
	if err != nil {
		return GenreStationSettings{}, err
	}
	list, err := loadGenreStations(ctx, s.pool)
	if err != nil {
		return GenreStationSettings{}, err
	}
	out := GenreStationSettings{Enabled: enabled, ShowAt: GenreStationShowAt, HideBelow: GenreStationHideBelow, Genres: []GenreStation{}}
	for _, a := range list {
		if !a.stillAStationGenre {
			continue
		}
		g := GenreStation{
			Genre: a.genre, GenreName: a.stationGenreName, StationID: a.id, Name: a.name,
			NameIsCustom: a.nameIsCustom, Tracks: a.tracks,
		}
		if g.Name == "" {
			g.Name = a.stationGenreName
		}
		switch {
		case a.excluded:
			g.State = StateExcluded
		case !enabled:
			g.State = StateOff
		case a.active:
			g.State = StateActive
		default:
			g.State = StateBelowThreshold
		}
		out.Genres = append(out.Genres, g)
	}
	return out, nil
}

// SetGenreStationsEnabled switches genre stations on or off. Off hides
// them all (nothing is deleted); the caller refreshes.
func (s *Service) SetGenreStationsEnabled(ctx context.Context, enabled bool) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO app_state (key, value, updated_at) VALUES ($1, jsonb_build_object('enabled', $2::boolean), NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`, autoStationsKey, enabled)
	return err
}

// GenreStationUpdate changes one genre's station. Nil fields stay as they are.
type GenreStationUpdate struct {
	// Name renames the station for good; "" goes back to the genre's name.
	Name *string `json:"name"`
	// Excluded hides the genre's station and keeps the refresh from showing it.
	Excluded *bool `json:"excluded"`
}

// UpdateGenreStation renames a genre station or (un)excludes its genre.
// A genre that never had a station gets one, hidden, so the choice sticks
// for when it reaches the threshold. The caller refreshes.
func (s *Service) UpdateGenreStation(ctx context.Context, genre string, u GenreStationUpdate) error {
	var genreName string
	err := s.pool.QueryRow(ctx, `SELECT name FROM station_genres WHERE id = $1 AND is_station`, genre).Scan(&genreName)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	name, custom := (*string)(nil), (*bool)(nil)
	if u.Name != nil {
		n := strings.TrimSpace(*u.Name)
		isCustom := n != "" && n != genreName
		if n == "" {
			n = genreName
		} else if n, err = validName(n); err != nil {
			return err
		}
		name, custom = &n, &isCustom
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO stations (user_id, name, genres, auto_genre, auto_active, auto_excluded, name_is_custom)
		VALUES (NULL, COALESCE($2, $3), ARRAY[$1], $1, FALSE, COALESCE($4, FALSE), COALESCE($5, FALSE))
		ON CONFLICT (auto_genre) WHERE auto_genre IS NOT NULL DO UPDATE SET
			name = COALESCE($2, stations.name),
			auto_excluded = COALESCE($4, stations.auto_excluded),
			name_is_custom = COALESCE($5, stations.name_is_custom),
			updated_at = NOW()
	`, genre, name, genreName, u.Excluded, custom); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// renameGenreStation gives a genre station a name the refresh keeps
// (unless it is the genre's own name).
func (s *Service) renameGenreStation(ctx context.Context, id, name string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE stations SET name = $2, updated_at = NOW(),
			name_is_custom = $2 <> COALESCE((SELECT g.name FROM station_genres g WHERE g.id = stations.auto_genre), '')
		WHERE id = $1::uuid
	`, id, name)
	return err
}
