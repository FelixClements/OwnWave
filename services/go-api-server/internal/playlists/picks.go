package playlists

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	// picksLock serialises pick rebuilds across requests and API instances.
	picksLock = 4_242_103
	// picksVersion is part of a playlist's build input: changing the rules
	// below rebuilds every playlist's picks.
	picksVersion = 2
	// pickNeighbours is how many nearest library songs each playlist track offers.
	pickNeighbours = 5
	// minPickSimilarity is the cosine similarity a pick needs to the
	// playlist track that found it.
	minPickSimilarity = 0.7
)

// Pick is a library song that sounds like a playlist.
type Pick struct {
	TrackID         string   `json:"track_id"`
	Title           string   `json:"title"`
	Artist          *string  `json:"artist,omitempty"`
	DurationSeconds *float64 `json:"duration_seconds,omitempty"`
	Hits            int      `json:"hits"`
	Similarity      float32  `json:"similarity"`
}

// candidate is a song that one or more playlist tracks have among their
// nearest, in ranking order.
type candidate struct {
	LocalTrack
	hits       int
	similarity float32
}

// choosePicks keeps the best candidates, up to limit, one per song: a
// candidate with the title and an artist of a seed or an earlier pick is
// left out even when its length differs and so it is no recorded copy.
func choosePicks(seeds []LocalTrack, cands []candidate, limit int) []candidate {
	taken := map[string][][]string{}
	take := func(t LocalTrack) {
		k := titleKey(t.Title)
		taken[k] = append(taken[k], artistNames(t.Artist))
	}
	for _, s := range seeds {
		take(s)
	}
	out := make([]candidate, 0, limit)
candidates:
	for _, c := range cands {
		if len(out) == limit {
			break
		}
		names := artistNames(c.Artist)
		for _, other := range taken[titleKey(c.Title)] {
			if overlaps(names, other) {
				continue candidates
			}
		}
		take(c.LocalTrack)
		out = append(out, c)
	}
	return out
}

// RebuildPicks refreshes the picks of every playlist whose input changed
// and returns how many playlists it rebuilt. A playlist's input is its
// matched songs, its removed picks and the revisions of its songs' neighbour
// lists (docs/adr/0007). With followLibrary, as background runs do, it first
// brings the neighbour lists in line with the library, so a new song
// rebuilds only the playlists whose songs it is near. Without, as requests
// do, it rebuilds only playlists whose own songs or removals changed.
func (s *Service) RebuildPicks(ctx context.Context, followLibrary bool) (int, error) {
	start := time.Now()
	var nb neighbourRun
	if followLibrary {
		var err error
		if nb, err = s.syncNeighbours(ctx); err != nil {
			return 0, fmt.Errorf("neighbour lists: %w", err)
		}
	}
	rows, err := s.pool.Query(ctx, `
		WITH seeds AS (
			SELECT DISTINCT pi.playlist_id, ms.song_id AS id
			FROM playlist_items pi
			JOIN spotify_track_matches m ON m.spotify_track_id = pi.spotify_track_id
			JOIN songs ms ON ms.track_id = m.track_id
			WHERE NOT pi.hidden
		), x AS (
			SELECT s.playlist_id,
			       string_agg(s.id::text, ',' ORDER BY s.id::text) AS seeds,
			       string_agg(s.id::text || ':' || COALESCE(l.rev, 0), ',' ORDER BY s.id::text) AS lists
			FROM seeds s LEFT JOIN song_neighbour_lists l ON l.song_id = s.id
			GROUP BY s.playlist_id
		), r AS (
			SELECT playlist_id, string_agg(track_id::text, ',' ORDER BY track_id::text) AS removals
			FROM playlist_pick_removals GROUP BY playlist_id
		)
		SELECT p.id::text, COALESCE(p.picks_input, ''), md5(COALESCE(x.seeds, '')),
		       md5(COALESCE(x.lists, '')), md5(COALESCE(r.removals, ''))
		FROM playlists p
		LEFT JOIN x ON x.playlist_id = p.id
		LEFT JOIN r ON r.playlist_id = p.id
	`)
	if err != nil {
		return 0, err
	}
	type playlist struct{ id, input string }
	var stale []playlist
	var id, have, seeds, lists, removals string
	if _, err := pgx.ForEachRow(rows, []any{&id, &have, &seeds, &lists, &removals}, func() error {
		own := fmt.Sprintf("%d|%s|%s|", picksVersion, seeds, removals)
		input := own + lists
		// Input from older rules waits for a background run, which fills the
		// neighbour lists first.
		ownChanged := have == "" || (strings.HasPrefix(have, fmt.Sprintf("%d|", picksVersion)) && !strings.HasPrefix(have, own))
		if input != have && (followLibrary || ownChanged) {
			stale = append(stale, playlist{id, input})
		}
		return nil
	}); err != nil {
		return 0, err
	}
	deepened := 0
	for _, p := range stale {
		n, err := s.rebuildPlaylistPicks(ctx, p.id, p.input)
		if err != nil {
			return 0, fmt.Errorf("playlist %s: %w", p.id, err)
		}
		deepened += n
	}
	if len(stale) > 0 || nb.changed() || deepened > 0 {
		slog.Info("library picks rebuilt",
			"duration_ms", time.Since(start).Milliseconds(),
			"follow_library", followLibrary,
			"playlists_rebuilt", len(stale),
			"lists_recomputed", nb.Recomputed,
			"lists_extended", nb.Extended,
			"lists_removed", nb.Removed,
			"lists_full", nb.Full,
			"lists_deepened", deepened)
	}
	return len(stale), nil
}

// rebuildPlaylistPicks replaces a playlist's picks with what its songs'
// neighbour lists offer and returns how many lists it had to grow.
func (s *Service) rebuildPlaylistPicks(ctx context.Context, playlistID, input string) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, picksLock); err != nil {
		return 0, err
	}

	const seedSQL = `
		SELECT DISTINCT ms.song_id AS id
		FROM playlist_items pi
		JOIN spotify_track_matches m ON m.spotify_track_id = pi.spotify_track_id
		JOIN songs ms ON ms.track_id = m.track_id
		WHERE pi.playlist_id = $1::uuid AND NOT pi.hidden`
	rows, err := tx.Query(ctx, `
		SELECT t.id::text, t.title, COALESCE(a.name, '')
		FROM tracks t LEFT JOIN artists a ON a.id = t.artist_id
		WHERE t.id IN (`+seedSQL+`)
	`, playlistID)
	if err != nil {
		return 0, err
	}
	seeds, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (LocalTrack, error) {
		var t LocalTrack
		err := row.Scan(&t.ID, &t.Title, &t.Artist)
		return t, err
	})
	if err != nil {
		return 0, err
	}

	seedIDs := make([]string, len(seeds))
	for i, s := range seeds {
		seedIDs[i] = s.ID
	}
	deepened, err := deepenLists(ctx, tx, playlistID, seedIDs)
	if err != nil {
		return 0, err
	}

	// Each playlist song offers its pickNeighbours nearest songs, leaving
	// out the playlist's own songs and its removed picks. A song can be a
	// pick of any playlist it is not already in, also when another playlist
	// matches it. A copy is never a pick: its best copy is. Lists hold only
	// best copies at or above the floor; the joins skip what changed since
	// the last neighbour sync.
	rows, err = tx.Query(ctx, `
		WITH seeds AS (SELECT unnest($2::uuid[]) AS id),
		ranked AS (
			SELECT n.neighbour_id, n.similarity, row_number() OVER (
				PARTITION BY n.song_id ORDER BY n.similarity DESC, n.neighbour_id) AS r
			FROM song_neighbours n
			WHERE n.song_id IN (SELECT id FROM seeds)
			  AND n.neighbour_id NOT IN (SELECT id FROM seeds)
			  AND n.neighbour_id NOT IN (SELECT r.track_id FROM playlist_pick_removals r WHERE r.playlist_id = $1::uuid)
			  AND EXISTS (SELECT 1 FROM tracks t WHERE t.id = n.neighbour_id)
			  AND NOT EXISTS (SELECT 1 FROM track_copies x WHERE x.track_id = n.neighbour_id)
		)
		SELECT t.id::text, t.title, COALESCE(a.name, ''), n.hits, n.best
		FROM (
			SELECT neighbour_id, COUNT(*)::int AS hits, MAX(similarity)::real AS best
			FROM ranked WHERE r <= $3
			GROUP BY neighbour_id
		) n
		JOIN tracks t ON t.id = n.neighbour_id
		LEFT JOIN artists a ON a.id = t.artist_id
		ORDER BY n.hits DESC, n.best DESC, t.id
	`, playlistID, seedIDs, pickNeighbours)
	if err != nil {
		return 0, err
	}
	cands, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (candidate, error) {
		var c candidate
		err := row.Scan(&c.ID, &c.Title, &c.Artist, &c.hits, &c.similarity)
		return c, err
	})
	if err != nil {
		return 0, err
	}

	// The limit sets the mix: a playlist gains at most as many songs as it has.
	picks := choosePicks(seeds, cands, len(seeds))
	ids, hits, sims := make([]string, len(picks)), make([]int32, len(picks)), make([]float32, len(picks))
	for i, p := range picks {
		ids[i], hits[i], sims[i] = p.ID, int32(p.hits), p.similarity
	}
	if _, err := tx.Exec(ctx, `DELETE FROM playlist_library_picks WHERE playlist_id = $1::uuid`, playlistID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO playlist_library_picks (playlist_id, track_id, hits, similarity)
		SELECT $1::uuid, v.id, v.hits, v.sim FROM unnest($2::uuid[], $3::int[], $4::real[]) AS v(id, hits, sim)
	`, playlistID, ids, hits, sims); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `UPDATE playlists SET picks_input = $2 WHERE id = $1::uuid`, playlistID, input); err != nil {
		return 0, err
	}
	return deepened, tx.Commit(ctx)
}

// Picks lists a playlist's library picks, strongest first.
func (s *Service) Picks(ctx context.Context, playlistID string) ([]Pick, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id::text, t.title, a.name, t.duration_seconds, p.hits, p.similarity
		FROM playlist_library_picks p
		JOIN tracks t ON t.id = p.track_id
		LEFT JOIN artists a ON a.id = t.artist_id
		WHERE p.playlist_id = $1::uuid
		ORDER BY p.hits DESC, p.similarity DESC, t.id
	`, playlistID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Pick, error) {
		var p Pick
		err := row.Scan(&p.TrackID, &p.Title, &p.Artist, &p.DurationSeconds, &p.Hits, &p.Similarity)
		return p, err
	})
}

// RemovePick takes a song out of a playlist's picks for good; the playlist's
// other picks and the song's other playlists are not affected. The freed
// place fills at the next rebuild: the removal changes the playlist's input.
func (s *Service) RemovePick(ctx context.Context, playlistID, trackID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		DELETE FROM playlist_library_picks WHERE playlist_id = $1::uuid AND track_id = $2::uuid
	`, playlistID, trackID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO playlist_pick_removals (playlist_id, track_id) VALUES ($1::uuid, $2::uuid)
		ON CONFLICT DO NOTHING
	`, playlistID, trackID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
