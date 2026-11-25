package stations

import "fmt"

// sqlArgs accumulates positional SQL parameters ($1, $2, ...).
type sqlArgs []any

// add appends v and returns its placeholder.
func (a *sqlArgs) add(v any) string {
	*a = append(*a, v)
	return fmt.Sprintf("$%d", len(*a))
}

// where returns an SQL predicate on `tracks t` that holds for the songs the
// definition plays, each by its best copy, so t.id is the song id. A genre
// station plays every song of the library whose best copy's main genre is
// its genre; a style station every song whose best copy's style is its
// style. Any other station plays the songs of the local matches of its
// playlists (of every playlist for AllPlaylists), with Enhance also of their
// library picks, and when it has genres, songs with one of them among their
// station genres or as their style.
func (d Definition) where(args *sqlArgs) string {
	if d.Library {
		return `t.id IN (
		SELECT s.track_id FROM songs s WHERE s.best_copy AND s.track_id IN (` + songsWithGenres(args.add(d.Genres), true) + `))`
	}
	matches, picks := "", ""
	if !d.AllPlaylists {
		ids := args.add(d.PlaylistIDs)
		matches = ` AND pi.playlist_id = ANY(` + ids + `::uuid[])`
		picks = ` WHERE lp.playlist_id = ANY(` + ids + `::uuid[])`
	}
	cond := `t.id IN (
		SELECT ms.song_id FROM playlist_items pi
		JOIN spotify_track_matches m ON m.spotify_track_id = pi.spotify_track_id
		JOIN songs ms ON ms.track_id = m.track_id
		WHERE NOT pi.hidden` + matches
	if d.Enhance {
		cond += `
		UNION
		SELECT ps.song_id FROM playlist_library_picks lp JOIN songs ps ON ps.track_id = lp.track_id` + picks
	}
	cond += `)`
	if len(d.Genres) > 0 {
		cond += ` AND t.id IN (` + songsWithGenres(args.add(d.Genres), false) + `)`
	}
	return cond
}
