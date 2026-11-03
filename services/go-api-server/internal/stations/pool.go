package stations

import "fmt"

// Args accumulates positional SQL parameters ($1, $2, ...).
type Args []any

// Add appends v and returns its placeholder.
func (a *Args) Add(v any) string {
	*a = append(*a, v)
	return fmt.Sprintf("$%d", len(*a))
}

// Where returns an SQL predicate on `tracks t` that holds for the tracks
// the definition plays: local matches of its playlists, and when it has
// genres, tracks with one of them among their station genres.
func (d Definition) Where(args *Args) string {
	cond := `t.id IN (
		SELECT m.track_id FROM playlist_items pi
		JOIN spotify_track_matches m ON m.spotify_track_id = pi.spotify_track_id
		WHERE m.track_id IS NOT NULL AND pi.playlist_id = ANY(` + args.Add(d.PlaylistIDs) + `::uuid[]))`
	if len(d.Genres) > 0 {
		cond += ` AND EXISTS (
		SELECT 1 FROM track_station_genres g
		WHERE g.track_id = t.id AND g.genre = ANY(` + args.Add(d.Genres) + `::text[]))`
	}
	return cond
}
