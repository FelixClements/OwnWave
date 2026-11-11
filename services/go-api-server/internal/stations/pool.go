package stations

import "fmt"

// Args accumulates positional SQL parameters ($1, $2, ...).
type Args []any

// Add appends v and returns its placeholder.
func (a *Args) Add(v any) string {
	*a = append(*a, v)
	return fmt.Sprintf("$%d", len(*a))
}

// Where returns an SQL predicate on `tracks t` that holds for the songs the
// definition plays, each by its best copy. A genre station plays every song
// of the library whose main genre is its genre. Any other station plays the
// local matches of its playlists (of every playlist for AllPlaylists), with
// Enhance also their library picks, and when it has genres, songs with one
// of them among their station genres.
func (d Definition) Where(args *Args) string {
	if d.Library {
		return `t.copy_of IS NULL AND EXISTS (
		SELECT 1 FROM track_station_genres g
		WHERE g.track_id = t.id AND g.is_primary AND g.genre = ANY(` + args.Add(d.Genres) + `::text[]))`
	}
	matches, picks := "", ""
	if !d.AllPlaylists {
		ids := args.Add(d.PlaylistIDs)
		matches = ` AND pi.playlist_id = ANY(` + ids + `::uuid[])`
		picks = ` WHERE lp.playlist_id = ANY(` + ids + `::uuid[])`
	}
	cond := `t.copy_of IS NULL AND t.id IN (
		SELECT COALESCE(mt.copy_of, mt.id) FROM playlist_items pi
		JOIN spotify_track_matches m ON m.spotify_track_id = pi.spotify_track_id
		JOIN tracks mt ON mt.id = m.track_id
		WHERE NOT pi.hidden` + matches
	if d.Enhance {
		cond += `
		UNION
		SELECT lp.track_id FROM playlist_library_picks lp` + picks
	}
	cond += `)`
	if len(d.Genres) > 0 {
		cond += ` AND EXISTS (
		SELECT 1 FROM track_station_genres g
		WHERE g.track_id = t.id AND g.genre = ANY(` + args.Add(d.Genres) + `::text[]))`
	}
	return cond
}
