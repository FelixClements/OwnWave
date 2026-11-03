package stations

import (
	"context"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

// MinSuggestionTracks is the smallest pool a suggestion may have.
const MinSuggestionTracks = 25

// redundantShare: a genre suggestion covering at least this share of its
// pool would play almost the same tracks as the all-genres station.
const redundantShare = 0.9

// Suggestion is a station OwnWave proposes. Accepting it creates a station
// with these playlists and (unless Genre is empty) this one genre.
type Suggestion struct {
	PlaylistIDs   []string `json:"playlist_ids"`
	PlaylistNames []string `json:"playlist_names"`
	Genre         string   `json:"genre"` // "" = all genres of the pool
	GenreName     string   `json:"genre_name,omitempty"`
	TrackCount    int      `json:"track_count"`
	Name          string   `json:"name"`
}

type playlistRef struct{ id, name string }

// poolData is everything suggestions are computed from, loaded at once:
// the library is small enough (thousands of matched tracks) to count in memory.
type poolData struct {
	playlists   []playlistRef
	matched     map[string]map[string]bool // playlist id -> local track ids
	trackGenres map[string]map[string]bool // track id -> station genres
	genres      []Genre                    // station genres, display order
	stations    map[string]bool            // pool key + "|" + genres key of existing stations
	dismissed   map[string]bool            // pool key + "|" + genre
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
		{`SELECT DISTINCT pi.playlist_id::text, m.track_id::text FROM playlist_items pi
		  JOIN spotify_track_matches m ON m.spotify_track_id = pi.spotify_track_id
		  WHERE m.track_id IS NOT NULL`, func(p, t string) {
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
		  FROM stations s`, func(pool, genres string) {
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

// poolTracks is the union of the playlists' matched tracks.
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

// Suggestions proposes stations: for each playlist alone and for all
// playlists together, the whole pool and each station genre with at least
// MinSuggestionTracks tracks. Pools that already have such a station, and
// dismissed suggestions, are skipped. Largest first.
func (s *Service) Suggestions(ctx context.Context) ([]Suggestion, error) {
	d, err := s.loadPools(ctx)
	if err != nil {
		return nil, err
	}
	type pool struct {
		ids, names []string
		all        bool // every playlist
	}
	var pools []pool
	for _, p := range d.playlists {
		pools = append(pools, pool{ids: []string{p.id}, names: []string{p.name}})
	}
	if len(d.playlists) > 1 {
		all := pool{all: true}
		for _, p := range d.playlists {
			all.ids = append(all.ids, p.id)
			all.names = append(all.names, p.name)
		}
		pools = append(pools, all)
	}

	out := []Suggestion{}
	for _, p := range pools {
		key := PoolKey(p.ids)
		tracks := d.poolTracks(p.ids)
		if len(tracks) < MinSuggestionTracks {
			continue
		}
		sorted := append([]string(nil), p.ids...)
		sort.Strings(sorted)
		poolName := p.names[0]
		if p.all {
			poolName = "All playlists"
		}
		if !d.stations[key+"|"] && !d.dismissed[key+"|"] {
			out = append(out, Suggestion{
				PlaylistIDs: sorted, PlaylistNames: p.names, TrackCount: len(tracks), Name: poolName,
			})
		}
		for _, g := range d.genres {
			n := d.countGenre(tracks, g.ID)
			if n < MinSuggestionTracks || float64(n) >= redundantShare*float64(len(tracks)) ||
				d.stations[key+"|"+g.ID] || d.dismissed[key+"|"+g.ID] {
				continue
			}
			name := g.Name
			if !p.all {
				name = g.Name + " · " + poolName
			}
			out = append(out, Suggestion{
				PlaylistIDs: sorted, PlaylistNames: p.names, Genre: g.ID, GenreName: g.Name,
				TrackCount: n, Name: name,
			})
		}
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
