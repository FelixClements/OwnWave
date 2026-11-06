package playlists

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	// picksLock serialises pick rebuilds across requests and API instances.
	picksLock = 4_242_103
	// picksVersion is part of a playlist's build input: changing the rules
	// below rebuilds every playlist's picks.
	picksVersion = 1
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

// RebuildPicks refreshes the picks of every playlist whose matched songs
// changed and returns how many playlists it rebuilt. With followLibrary it
// also rebuilds when the library's songs or embeddings changed, which means
// every playlist: about a minute on a library of thousands, so requests
// leave that to the background runs.
func (s *Service) RebuildPicks(ctx context.Context, followLibrary bool) (int, error) {
	var library string
	if err := s.pool.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM track_audio_genres WHERE embedding IS NOT NULL)::text
		       || '|' || COALESCE((SELECT MAX(analyzed_at)::text FROM track_audio_genres), '')
		       || '|' || (SELECT COUNT(*) FROM tracks WHERE copy_of IS NOT NULL)::text
	`).Scan(&library); err != nil {
		return 0, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT p.id::text, COALESCE(p.picks_input, ''), md5(COALESCE((
			SELECT string_agg(DISTINCT COALESCE(mt.copy_of, mt.id)::text, ',' ORDER BY COALESCE(mt.copy_of, mt.id)::text)
			FROM playlist_items pi
			JOIN spotify_track_matches m ON m.spotify_track_id = pi.spotify_track_id
			JOIN tracks mt ON mt.id = m.track_id
			WHERE pi.playlist_id = p.id), ''))
		FROM playlists p
	`)
	if err != nil {
		return 0, err
	}
	type playlist struct{ id, input string }
	var stale []playlist
	var id, have, seeds string
	if _, err := pgx.ForEachRow(rows, []any{&id, &have, &seeds}, func() error {
		input := fmt.Sprintf("%d|%s|%s", picksVersion, library, seeds)
		if input != have && (followLibrary || !strings.HasSuffix(have, "|"+seeds)) {
			stale = append(stale, playlist{id, input})
		}
		return nil
	}); err != nil {
		return 0, err
	}
	for _, p := range stale {
		if err := s.rebuildPlaylistPicks(ctx, p.id, p.input); err != nil {
			return 0, fmt.Errorf("playlist %s: %w", p.id, err)
		}
	}
	return len(stale), nil
}

func (s *Service) rebuildPlaylistPicks(ctx context.Context, playlistID, input string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, picksLock); err != nil {
		return err
	}

	const seedSQL = `
		SELECT DISTINCT COALESCE(mt.copy_of, mt.id) AS id
		FROM playlist_items pi
		JOIN spotify_track_matches m ON m.spotify_track_id = pi.spotify_track_id
		JOIN tracks mt ON mt.id = m.track_id
		WHERE pi.playlist_id = $1::uuid`
	rows, err := tx.Query(ctx, `
		SELECT t.id::text, t.title, COALESCE(a.name, '')
		FROM tracks t LEFT JOIN artists a ON a.id = t.artist_id
		WHERE t.id IN (`+seedSQL+`)
	`, playlistID)
	if err != nil {
		return err
	}
	seeds, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (LocalTrack, error) {
		var t LocalTrack
		err := row.Scan(&t.ID, &t.Title, &t.Artist)
		return t, err
	})
	if err != nil {
		return err
	}

	// A song can be a pick of any playlist it is not already in, also when
	// another playlist matches it.
	rows, err = tx.Query(ctx, `
		WITH seeds AS (`+seedSQL+`)
		SELECT t.id::text, t.title, COALESCE(a.name, ''), n.hits, n.best
		FROM (
			SELECT nb.track_id, COUNT(*)::int AS hits, MAX(nb.similarity)::real AS best
			FROM seeds s
			JOIN track_audio_genres sg ON sg.track_id = s.id AND sg.embedding IS NOT NULL
			CROSS JOIN LATERAL (
				SELECT c.track_id, 1 - (c.embedding <=> sg.embedding) AS similarity
				FROM track_audio_genres c
				JOIN tracks ct ON ct.id = c.track_id AND ct.copy_of IS NULL
				WHERE c.embedding IS NOT NULL
				  AND c.track_id NOT IN (SELECT id FROM seeds)
				  AND c.track_id NOT IN (SELECT r.track_id FROM playlist_pick_removals r WHERE r.playlist_id = $1::uuid)
				ORDER BY c.embedding <=> sg.embedding
				LIMIT $2
			) nb
			WHERE nb.similarity >= $3
			GROUP BY nb.track_id
		) n
		JOIN tracks t ON t.id = n.track_id
		LEFT JOIN artists a ON a.id = t.artist_id
		ORDER BY n.hits DESC, n.best DESC, t.id
	`, playlistID, pickNeighbours, minPickSimilarity)
	if err != nil {
		return err
	}
	cands, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (candidate, error) {
		var c candidate
		err := row.Scan(&c.ID, &c.Title, &c.Artist, &c.hits, &c.similarity)
		return c, err
	})
	if err != nil {
		return err
	}

	// The limit sets the mix: a playlist gains at most as many songs as it has.
	picks := choosePicks(seeds, cands, len(seeds))
	ids, hits, sims := make([]string, len(picks)), make([]int32, len(picks)), make([]float32, len(picks))
	for i, p := range picks {
		ids[i], hits[i], sims[i] = p.ID, int32(p.hits), p.similarity
	}
	if _, err := tx.Exec(ctx, `DELETE FROM playlist_library_picks WHERE playlist_id = $1::uuid`, playlistID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO playlist_library_picks (playlist_id, track_id, hits, similarity)
		SELECT $1::uuid, v.id, v.hits, v.sim FROM unnest($2::uuid[], $3::int[], $4::real[]) AS v(id, hits, sim)
	`, playlistID, ids, hits, sims); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE playlists SET picks_input = $2 WHERE id = $1::uuid`, playlistID, input); err != nil {
		return err
	}
	return tx.Commit(ctx)
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
// place fills at the next rebuild.
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
