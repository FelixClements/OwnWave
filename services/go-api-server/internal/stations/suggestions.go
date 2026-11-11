package stations

import (
	"context"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

// MinSuggestionTracks is the smallest pool a suggestion may have.
const MinSuggestionTracks = 25

// Suggestion is a custom station OwnWave proposes for one playlist.
// Accepting it creates a station that plays just that playlist. Genres
// across the library are covered by genre stations, which need no
// accepting.
type Suggestion struct {
	PlaylistIDs   []string `json:"playlist_ids"`
	PlaylistNames []string `json:"playlist_names"`
	TrackCount    int      `json:"track_count"`
	Name          string   `json:"name"`
}

type playlistRef struct{ id, name string }

// poolData is everything suggestions are computed from, loaded at once:
// the library is small enough (thousands of songs) to count in memory.
type poolData struct {
	playlists   []playlistRef
	matched     map[string]map[string]bool // playlist id -> its matched songs, by best copy (no library picks)
	trackGenres map[string]map[string]bool // track id -> station genres
	genres      []Genre                    // station genres, display order
	stations    map[string]bool            // pool key + "|" + genres key of existing custom stations
	dismissed   map[string]bool            // pool key + "|" + genre
}

func (d *poolData) allPlaylistIDs() []string {
	ids := make([]string, 0, len(d.playlists))
	for _, p := range d.playlists {
		ids = append(ids, p.id)
	}
	return ids
}

func (s *Service) loadPools(ctx context.Context) (*poolData, error) {
	d := &poolData{
		matched:     map[string]map[string]bool{},
		trackGenres: map[string]map[string]bool{},
		stations:    map[string]bool{},
		dismissed:   map[string]bool{},
	}
	collect := func(sql string, each func(a, b string)) error {
		rows, err := s.pool.Query(ctx, sql)
		if err != nil {
			return err
		}
		var a, b string
		_, err = pgx.ForEachRow(rows, []any{&a, &b}, func() error { each(a, b); return nil })
		return err
	}
	steps := []struct {
		sql  string
		each func(a, b string)
	}{
		{`SELECT id::text, name FROM playlists ORDER BY lower(name)`, func(id, name string) {
			d.playlists = append(d.playlists, playlistRef{id, name})
		}},
		{`SELECT pi.playlist_id::text, COALESCE(mt.copy_of, mt.id)::text FROM playlist_items pi
		  JOIN spotify_track_matches m ON m.spotify_track_id = pi.spotify_track_id
		  JOIN tracks mt ON mt.id = m.track_id
		  WHERE NOT pi.hidden`, func(p, t string) {
			if d.matched[p] == nil {
				d.matched[p] = map[string]bool{}
			}
			d.matched[p][t] = true
		}},
		{`SELECT track_id::text, genre FROM track_station_genres`, func(t, g string) {
			if d.trackGenres[t] == nil {
				d.trackGenres[t] = map[string]bool{}
			}
			d.trackGenres[t][g] = true
		}},
		{`SELECT id, name FROM station_genres WHERE is_station ORDER BY position`, func(id, name string) {
			d.genres = append(d.genres, Genre{ID: id, Name: name})
		}},
		{`SELECT COALESCE((SELECT string_agg(sp.playlist_id::text, ',' ORDER BY sp.playlist_id::text)
		                   FROM station_playlists sp WHERE sp.station_id = s.id), ''),
		         array_to_string(s.genres, ',')
		  FROM stations s WHERE s.auto_genre IS NULL AND NOT s.all_playlists`, func(pool, genres string) {
			d.stations[pool+"|"+genres] = true
		}},
		{`SELECT pool_key, genre FROM station_suggestion_dismissals`, func(pool, genre string) {
			d.dismissed[pool+"|"+genre] = true
		}},
	}
	for _, step := range steps {
		if err := collect(step.sql, step.each); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// poolTracks is the union of the songs the playlists play.
func (d *poolData) poolTracks(playlistIDs []string) map[string]bool {
	out := map[string]bool{}
	for _, p := range playlistIDs {
		for t := range d.matched[p] {
			out[t] = true
		}
	}
	return out
}

func (d *poolData) countGenre(tracks map[string]bool, genre string) int {
	n := 0
	for t := range tracks {
		if d.trackGenres[t][genre] {
			n++
		}
	}
	return n
}

// Suggestions proposes custom stations, one per playlist with at least
// MinSuggestionTracks matched tracks: the whole playlist, nothing else.
// Skipped: playlists that already have such a station (no genre filter)
// and dismissed suggestions. Largest first.
//
// Pool keys of one playlist never change, so a dismissal stays valid when
// playlists are added.
func (s *Service) Suggestions(ctx context.Context) ([]Suggestion, error) {
	d, err := s.loadPools(ctx)
	if err != nil {
		return nil, err
	}

	out := []Suggestion{}
	for _, p := range d.playlists {
		ids := []string{p.id}
		key := PoolKey(ids)
		tracks := d.poolTracks(ids)
		if len(tracks) < MinSuggestionTracks || d.stations[key+"|"] || d.dismissed[key+"|"] {
			continue
		}
		out = append(out, Suggestion{
			PlaylistIDs: ids, PlaylistNames: []string{p.name}, TrackCount: len(tracks), Name: p.name,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TrackCount > out[j].TrackCount })
	return out, nil
}

func sortGenreCounts(gs []GenreCount) {
	sort.SliceStable(gs, func(i, j int) bool {
		if gs[i].Tracks != gs[j].Tracks {
			return gs[i].Tracks > gs[j].Tracks
		}
		return strings.Compare(gs[i].ID, gs[j].ID) < 0
	})
}
