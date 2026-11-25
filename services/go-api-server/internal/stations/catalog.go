package stations

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// The station genre catalog has two levels (ADR 0008): genres such as House
// and styles such as Deep House, each style under exactly one genre. Both
// get stations.
const (
	CatalogGenre = "genre"
	CatalogStyle = "style"
)

// catalogSQL reads station_genres with each entry's kind and parent genre.
// kind and parent_id arrive with the genre v2 switch migration
// (database/genre-v2); until it is in, every entry is a genre. Reading them
// through to_jsonb lets the same SQL run before and after the switch, so the
// release only has to apply the migration.
const catalogSQL = `(SELECT c.id, c.name, c.position, c.is_station,
	COALESCE(to_jsonb(c)->>'kind', 'genre') AS kind, to_jsonb(c)->>'parent_id' AS parent_id
	FROM station_genres c)`

// styleIDsSQL is the ids of the catalog's styles (none before the switch).
const styleIDsSQL = `SELECT id FROM ` + catalogSQL + ` sc WHERE sc.kind = 'style'`

// catalogEntry is one genre or style of the catalog.
type catalogEntry struct {
	id, name  string
	kind      string // CatalogGenre or CatalogStyle
	parent    string // a style's genre; "" for a genre
	isStation bool
}

// catalogOrder sorts `catalogSQL c` joined with its parent `p` for display:
// genres in their order, each followed by its styles in theirs.
const catalogOrder = `COALESCE(p.position, c.position), COALESCE(p.id, c.id), c.parent_id IS NOT NULL, c.position, c.id`

// loadCatalog returns every genre and style in display order.
func loadCatalog(ctx context.Context, q queryer) ([]catalogEntry, error) {
	rows, err := q.Query(ctx, `
		SELECT c.id, c.name, c.kind, COALESCE(c.parent_id, ''), c.is_station
		FROM `+catalogSQL+` c LEFT JOIN `+catalogSQL+` p ON p.id = c.parent_id
		ORDER BY `+catalogOrder)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (catalogEntry, error) {
		var e catalogEntry
		err := row.Scan(&e.id, &e.name, &e.kind, &e.parent, &e.isStation)
		return e, err
	})
}

// songsWithGenres is SQL for the track ids (every copy) whose genre or style
// is one of the ids at placeholder arg. A genre counts through the song's
// station genres (only its main genre when mainOnly); a style through the
// style the genre engine decided. Unsure songs have neither, so they are on
// no genre or style station.
func songsWithGenres(arg string, mainOnly bool) string {
	main := ""
	if mainOnly {
		main = "g.is_primary AND "
	}
	return `SELECT g.track_id FROM track_station_genres g WHERE ` + main + `g.genre = ANY(` + arg + `::text[])
		UNION ALL
		SELECT d.track_id FROM track_genre_decisions d
		WHERE d.genre IS NOT NULL AND d.style = ANY(` + arg + `::text[]) AND d.style IN (` + styleIDsSQL + `)`
}
