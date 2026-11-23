package playlists

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Each best copy with an embedding keeps a list of its nearest songs in
// song_neighbours (docs/adr/0007). A background run brings the lists in line
// with the library; rebuilding a playlist's picks then only reads them.
const (
	// neighboursVersion is stored with each list: changing the rules below
	// rebuilds every list once.
	neighboursVersion = 1
	// neighbourDepth is how many neighbours a list starts with. A playlist
	// filters its own songs and removed picks out of each list, so a list
	// that leaves fewer than pickNeighbours grows (deepen). On the live
	// library 20 was enough for 93% of playlist tracks.
	neighbourDepth = 20
	// deepenFactor is how much a list grows each time it is too short.
	deepenFactor = 4
	// maxAddedSongs bounds the exact comparison that adds new songs to
	// existing lists, which reads every embedding once per new song (about
	// 60 ms each on the live library). With more new songs, every list is
	// rebuilt through the index instead.
	maxAddedSongs = 200
)

// neighbourRun counts what a neighbour sync changed.
type neighbourRun struct {
	// Recomputed lists: new songs, new embeddings, and lists that lost a
	// neighbour that was deleted, became a copy or changed its embedding.
	Recomputed int
	// Extended lists: existing lists a new song entered.
	Extended int
	// Removed lists: songs that were deleted or became a copy.
	Removed int
	// Full is set when every list was rebuilt (first run, new rules, or
	// many new songs).
	Full bool
}

func (r neighbourRun) changed() bool {
	return r.Recomputed > 0 || r.Extended > 0 || r.Removed > 0
}

// syncNeighbours brings every song's neighbour list in line with the
// library: songs, embeddings and copies. It works from a diff of the
// library against the lists, so running it again changes nothing, and an
// empty table (first run) is filled by the same code.
func (s *Service) syncNeighbours(ctx context.Context) (neighbourRun, error) {
	var run neighbourRun
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return run, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, picksLock); err != nil {
		return run, err
	}

	// The songs that should have a list, and which of them have a new
	// embedding (or none yet). This reads no embeddings.
	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE nb_cur ON COMMIT DROP AS
		SELECT g.track_id AS song_id, g.analyzed_at
		FROM track_audio_genres g
		WHERE g.embedding IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM track_copies c WHERE c.track_id = g.track_id);
		ALTER TABLE nb_cur ADD PRIMARY KEY (song_id);
		ANALYZE nb_cur;
		CREATE TEMP TABLE nb_new ON COMMIT DROP AS
		SELECT c.song_id
		FROM nb_cur c LEFT JOIN song_neighbour_lists l ON l.song_id = c.song_id
		WHERE l.song_id IS NULL OR l.analyzed_at <> c.analyzed_at OR l.version <> `+fmt.Sprint(neighboursVersion)+`;
		ALTER TABLE nb_new ADD PRIMARY KEY (song_id);
		ANALYZE nb_new;
	`); err != nil {
		return run, err
	}

	// Songs that were deleted or became a copy lose their list. Deleted
	// tracks lost theirs already, by cascade.
	tag, err := tx.Exec(ctx, `
		DELETE FROM song_neighbour_lists l
		WHERE NOT EXISTS (SELECT 1 FROM nb_cur c WHERE c.song_id = l.song_id)
	`)
	if err != nil {
		return run, err
	}
	run.Removed = int(tag.RowsAffected())

	var added, outdated int
	if err := tx.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM nb_new),
		       (SELECT COUNT(*) FROM song_neighbour_lists WHERE version <> $1)
	`, neighboursVersion).Scan(&added, &outdated); err != nil {
		return run, err
	}
	run.Full = outdated > 0 || added > maxAddedSongs

	// The lists to compute from the index. Besides new songs, a list that
	// holds a neighbour that is gone or moved is rebuilt: a list missing an
	// entry would look complete, and a moved song may no longer belong.
	dirty := `
		SELECT song_id FROM nb_new
		UNION
		SELECT n.song_id FROM song_neighbours n
		WHERE NOT EXISTS (SELECT 1 FROM nb_cur c WHERE c.song_id = n.neighbour_id)
		UNION
		SELECT n.song_id FROM nb_new w JOIN song_neighbours n ON n.neighbour_id = w.song_id`
	if run.Full {
		dirty = `SELECT song_id FROM nb_cur`
	}
	rows, err := tx.Query(ctx, `
		SELECT d.song_id::text, COALESCE(CASE WHEN l.version = $1 THEN l.depth END, $2)
		FROM (`+dirty+`) d
		LEFT JOIN song_neighbour_lists l ON l.song_id = d.song_id
	`, neighboursVersion, neighbourDepth)
	if err != nil {
		return run, err
	}
	var ids []string
	var depths []int32
	var id string
	var depth int32
	if _, err := pgx.ForEachRow(rows, []any{&id, &depth}, func() error {
		ids, depths = append(ids, id), append(depths, depth)
		return nil
	}); err != nil {
		return run, err
	}
	run.Recomputed = len(ids)
	if len(ids) == 0 {
		return run, tx.Commit(ctx)
	}

	changed, err := writeLists(ctx, tx, ids, depths)
	if err != nil {
		return run, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO song_neighbour_lists (song_id, analyzed_at, version, depth)
		SELECT v.id, c.analyzed_at, $3, v.depth
		FROM unnest($1::uuid[], $2::int[]) AS v(id, depth)
		JOIN nb_cur c ON c.song_id = v.id
		ON CONFLICT (song_id) DO UPDATE
		SET analyzed_at = EXCLUDED.analyzed_at, version = EXCLUDED.version, depth = EXCLUDED.depth
	`, ids, depths, neighboursVersion); err != nil {
		return run, err
	}
	if err := bumpRevs(ctx, tx, `SELECT unnest($1::uuid[])`, changed); err != nil {
		return run, err
	}

	if !run.Full && added > 0 {
		if run.Extended, err = addToLists(ctx, tx, ids); err != nil {
			return run, err
		}
	}
	return run, tx.Commit(ctx)
}

// writeLists computes the lists of songs ids, each up to its depth, through
// the HNSW index, replaces their rows and returns the songs whose list
// changed. The lists' own rows in song_neighbour_lists are the caller's.
func writeLists(ctx context.Context, tx pgx.Tx, ids []string, depths []int32) ([]string, error) {
	// The planner prefers an exact scan at a few thousand songs, which reads
	// every embedding (5 KB each, out of line) per song: about 0.15 s each
	// instead of 1 ms. strict_order keeps the index searching, nearest
	// first, past copies the filter drops.
	for _, set := range []string{
		`DROP TABLE IF EXISTS nb_rows`,
		`SET LOCAL enable_seqscan = off`,
		`SET LOCAL hnsw.ef_search = 100`,
		`SET LOCAL hnsw.iterative_scan = strict_order`,
	} {
		if _, err := tx.Exec(ctx, set); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE nb_rows ON COMMIT DROP AS
		SELECT d.id AS song_id, nb.neighbour_id, nb.similarity
		FROM unnest($1::uuid[], $2::int[]) AS d(id, depth)
		JOIN track_audio_genres sg ON sg.track_id = d.id AND sg.embedding IS NOT NULL
		CROSS JOIN LATERAL (
			SELECT c.track_id AS neighbour_id, 1 - (c.embedding <=> sg.embedding) AS similarity
			FROM track_audio_genres c
			WHERE c.track_id <> d.id
			  AND NOT EXISTS (SELECT 1 FROM track_copies x WHERE x.track_id = c.track_id)
			ORDER BY c.embedding <=> sg.embedding
			LIMIT d.depth
		) nb
		WHERE nb.similarity >= $3
	`, ids, depths, minPickSimilarity); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SET LOCAL enable_seqscan = on`); err != nil {
		return nil, err
	}

	// A list changed when its rows differ either way, or it is new.
	rows, err := tx.Query(ctx, `
		WITH old AS (
			SELECT n.song_id, n.neighbour_id, n.similarity FROM song_neighbours n
			WHERE n.song_id = ANY($1::uuid[])
		)
		SELECT song_id::text FROM (
			(SELECT * FROM nb_rows EXCEPT SELECT * FROM old)
			UNION ALL
			(SELECT * FROM old EXCEPT SELECT * FROM nb_rows)
		) d
		UNION
		SELECT v.id::text FROM unnest($1::uuid[]) AS v(id)
		WHERE NOT EXISTS (SELECT 1 FROM song_neighbour_lists l WHERE l.song_id = v.id)
	`, ids)
	if err != nil {
		return nil, err
	}
	changed, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM song_neighbours WHERE song_id = ANY($1::uuid[])`, ids); err != nil {
		return nil, err
	}
	// The list's row has to exist for the foreign key; a caller that adds
	// lists upserts the real values right after.
	if _, err := tx.Exec(ctx, `
		INSERT INTO song_neighbour_lists (song_id, analyzed_at, version, depth)
		SELECT g.track_id, g.analyzed_at, 0, 0
		FROM track_audio_genres g WHERE g.track_id = ANY($1::uuid[])
		ON CONFLICT (song_id) DO NOTHING
	`, ids); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO song_neighbours SELECT song_id, neighbour_id, similarity FROM nb_rows`); err != nil {
		return nil, err
	}
	return changed, nil
}

// addToLists puts new songs into the existing lists they now belong in: a
// list that is full takes a song closer than its last neighbour and drops
// that one, a shorter list takes any song at or above the floor. Lists in
// skip were just computed and hold the new songs already. Similarity is
// symmetric, so this compares each new song with every list's song exactly.
func addToLists(ctx context.Context, tx pgx.Tx, skip []string) (int, error) {
	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE nb_add ON COMMIT DROP AS
		WITH lists AS (
			SELECT l.song_id, CASE WHEN COUNT(n.neighbour_id) >= l.depth THEN MIN(n.similarity) END AS last
			FROM song_neighbour_lists l
			LEFT JOIN song_neighbours n ON n.song_id = l.song_id
			WHERE l.song_id <> ALL($1::uuid[])
			GROUP BY l.song_id, l.depth
		)
		SELECT l.song_id, w.song_id AS neighbour_id, x.similarity
		FROM nb_new w
		JOIN track_audio_genres wg ON wg.track_id = w.song_id
		CROSS JOIN lists l
		JOIN track_audio_genres lg ON lg.track_id = l.song_id
		CROSS JOIN LATERAL (SELECT 1 - (lg.embedding <=> wg.embedding) AS similarity) x
		WHERE x.similarity >= COALESCE(l.last, $2)
	`, skip, minPickSimilarity); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO song_neighbours (song_id, neighbour_id, similarity)
		SELECT song_id, neighbour_id, similarity FROM nb_add
		ON CONFLICT (song_id, neighbour_id) DO UPDATE SET similarity = EXCLUDED.similarity
	`); err != nil {
		return 0, err
	}
	// Full lists keep their depth: the farthest neighbours drop out, in the
	// order picks read lists.
	if _, err := tx.Exec(ctx, `
		DELETE FROM song_neighbours n
		USING (
			SELECT o.song_id, o.neighbour_id, row_number() OVER (
				PARTITION BY o.song_id ORDER BY o.similarity DESC, o.neighbour_id) AS r
			FROM song_neighbours o
			WHERE o.song_id IN (SELECT song_id FROM nb_add)
		) o
		JOIN song_neighbour_lists l ON l.song_id = o.song_id
		WHERE n.song_id = o.song_id AND n.neighbour_id = o.neighbour_id AND o.r > l.depth
	`); err != nil {
		return 0, err
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT COUNT(DISTINCT song_id) FROM nb_add`).Scan(&n); err != nil {
		return 0, err
	}
	return n, bumpRevs(ctx, tx, `SELECT DISTINCT song_id FROM nb_add`)
}

// bumpRevs gives the lists of the songs the query returns a new revision,
// which rebuilds the playlists they seed.
func bumpRevs(ctx context.Context, tx pgx.Tx, songs string, args ...any) error {
	_, err := tx.Exec(ctx, `
		UPDATE song_neighbour_lists SET rev = nextval('song_neighbour_rev')
		WHERE song_id IN (`+songs+`)
	`, args...)
	return err
}

// deepenLists grows the lists of a playlist's seeds that are full but leave
// fewer than pickNeighbours picks once the playlist's own songs and removed
// picks are left out, until each leaves enough or holds every song above the
// floor. Growing only adds neighbours past a list's end, so other playlists'
// picks stay the same and the revision stays too.
func deepenLists(ctx context.Context, tx pgx.Tx, playlistID string, seeds []string) (int, error) {
	grown := 0
	for {
		rows, err := tx.Query(ctx, `
			SELECT l.song_id::text, l.depth * $4
			FROM song_neighbour_lists l
			WHERE l.song_id = ANY($2::uuid[])
			  AND (SELECT COUNT(*) FROM song_neighbours n WHERE n.song_id = l.song_id) >= l.depth
			  AND (SELECT COUNT(*) FROM song_neighbours n
			       WHERE n.song_id = l.song_id
			         AND n.neighbour_id <> ALL($2::uuid[])
			         AND EXISTS (SELECT 1 FROM tracks t WHERE t.id = n.neighbour_id)
			         AND NOT EXISTS (SELECT 1 FROM track_copies x WHERE x.track_id = n.neighbour_id)
			         AND NOT EXISTS (SELECT 1 FROM playlist_pick_removals r
			                         WHERE r.playlist_id = $1::uuid AND r.track_id = n.neighbour_id)
			  ) < $3
		`, playlistID, seeds, pickNeighbours, deepenFactor)
		if err != nil {
			return grown, err
		}
		var ids []string
		var depths []int32
		var id string
		var depth int32
		if _, err := pgx.ForEachRow(rows, []any{&id, &depth}, func() error {
			ids, depths = append(ids, id), append(depths, depth)
			return nil
		}); err != nil {
			return grown, err
		}
		if len(ids) == 0 {
			return grown, nil
		}
		if _, err := writeLists(ctx, tx, ids, depths); err != nil {
			return grown, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE song_neighbour_lists l SET depth = v.depth
			FROM unnest($1::uuid[], $2::int[]) AS v(id, depth) WHERE l.song_id = v.id
		`, ids, depths); err != nil {
			return grown, err
		}
		grown += len(ids)
	}
}
