package stations

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrUnknownNode is returned for a browse node id that does not exist.
var ErrUnknownNode = errors.New("unknown browse node")

// BrowseItem is one entry of the media tree apps show (Android Auto's
// browse tree): either a folder to open or a station to play.
type BrowseItem struct {
	// ID is the node to request next (folders) or "station:<id>" (stations).
	ID         string `json:"id"`
	Title      string `json:"title"`
	Subtitle   string `json:"subtitle,omitempty"`
	Browsable  bool   `json:"browsable"`
	Playable   bool   `json:"playable"`
	StationID  string `json:"station_id,omitempty"`
	TrackCount int    `json:"track_count,omitempty"`
	// ArtworkURL is relative to the server and needs the user's credentials.
	ArtworkURL string `json:"artwork_url,omitempty"`
}

// BrowseNode is a folder of the tree with its children.
type BrowseNode struct {
	ID    string       `json:"id"`
	Title string       `json:"title"`
	Items []BrowseItem `json:"items"`
}

const recentLimit = 20

// Browse returns a node of the media tree:
//
//	root                  Stations, Recently played, Genres, Playlists
//	stations              every station the user can play
//	recent                stations by last play
//	genres                genres used by stations  -> genre:<id>
//	playlists             playlists used by stations -> playlist:<id>
//	genre:<id>            stations with that genre
//	playlist:<id>         stations playing that playlist
func (s *Service) Browse(ctx context.Context, userID, node string) (BrowseNode, error) {
	switch {
	case node == "" || node == "root":
		return BrowseNode{ID: "root", Title: "OwnWave", Items: []BrowseItem{
			{ID: "stations", Title: "Stations", Browsable: true},
			{ID: "recent", Title: "Recently played", Browsable: true},
			{ID: "genres", Title: "Genres", Browsable: true},
			{ID: "playlists", Title: "Playlists", Browsable: true},
		}}, nil
	case node == "stations":
		items, err := s.stationItems(ctx, userID, func(Station) bool { return true })
		return BrowseNode{ID: node, Title: "Stations", Items: items}, err
	case node == "recent":
		items, err := s.recentItems(ctx, userID)
		return BrowseNode{ID: node, Title: "Recently played", Items: items}, err
	case node == "genres":
		items, err := s.genreFolders(ctx, userID)
		return BrowseNode{ID: node, Title: "Genres", Items: items}, err
	case node == "playlists":
		items, err := s.playlistFolders(ctx, userID)
		return BrowseNode{ID: node, Title: "Playlists", Items: items}, err
	case strings.HasPrefix(node, "genre:"):
		g := strings.TrimPrefix(node, "genre:")
		names, err := s.genreNames(ctx)
		if err != nil {
			return BrowseNode{}, err
		}
		name, ok := names[g]
		if !ok {
			return BrowseNode{}, ErrUnknownNode
		}
		items, err := s.stationItems(ctx, userID, func(st Station) bool { return contains(st.Genres, g) })
		return BrowseNode{ID: node, Title: name, Items: items}, err
	case strings.HasPrefix(node, "playlist:"):
		id := strings.TrimPrefix(node, "playlist:")
		if _, err := uuid.Parse(id); err != nil {
			return BrowseNode{}, ErrUnknownNode
		}
		var name string
		err := s.pool.QueryRow(ctx, `SELECT name FROM playlists WHERE id = $1::uuid`, id).Scan(&name)
		if errors.Is(err, pgx.ErrNoRows) {
			return BrowseNode{}, ErrUnknownNode
		}
		if err != nil {
			return BrowseNode{}, err
		}
		items, err := s.stationItems(ctx, userID, func(st Station) bool { return contains(st.PlaylistIDs, id) })
		return BrowseNode{ID: node, Title: name, Items: items}, err
	}
	return BrowseNode{}, ErrUnknownNode
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (s *Service) genreNames(ctx context.Context) (map[string]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name FROM station_genres`)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	var id, name string
	_, err = pgx.ForEachRow(rows, []any{&id, &name}, func() error { out[id] = name; return nil })
	return out, err
}

// StationItem turns a station into a playable browse item.
func StationItem(st Station, genreNames map[string]string) BrowseItem {
	var genres []string
	for _, g := range st.Genres {
		if n, ok := genreNames[g]; ok {
			genres = append(genres, n)
		} else {
			genres = append(genres, g)
		}
	}
	sub := fmt.Sprintf("%d tracks", st.TrackCount)
	if len(genres) > 0 {
		sub += " · " + strings.Join(genres, ", ")
	}
	return BrowseItem{
		ID: "station:" + st.ID, Title: st.Name, Subtitle: sub, Playable: true,
		StationID: st.ID, TrackCount: st.TrackCount, ArtworkURL: "/api/stations/" + st.ID + "/cover",
	}
}

func (s *Service) stationItems(ctx context.Context, userID string, keep func(Station) bool) ([]BrowseItem, error) {
	list, err := s.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	names, err := s.genreNames(ctx)
	if err != nil {
		return nil, err
	}
	items := []BrowseItem{}
	for _, st := range list {
		if keep(st) {
			items = append(items, StationItem(st, names))
		}
	}
	return items, nil
}

func (s *Service) recentItems(ctx context.Context, userID string) ([]BrowseItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT station_id::text FROM listening_history
		WHERE user_id = $1::uuid AND station_id IS NOT NULL
		GROUP BY station_id ORDER BY MAX(played_at) DESC LIMIT $2
	`, userID, recentLimit)
	if err != nil {
		return nil, err
	}
	order, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	rank := map[string]int{}
	for i, id := range order {
		rank[id] = i
	}
	items, err := s.stationItems(ctx, userID, func(st Station) bool { _, ok := rank[st.ID]; return ok })
	sort.SliceStable(items, func(i, j int) bool { return rank[items[i].StationID] < rank[items[j].StationID] })
	return items, err
}

func (s *Service) genreFolders(ctx context.Context, userID string) ([]BrowseItem, error) {
	list, err := s.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	stationsPer := map[string]int{}
	for _, st := range list {
		for _, g := range st.Genres {
			stationsPer[g]++
		}
	}
	rows, err := s.pool.Query(ctx, `SELECT id, name FROM station_genres WHERE is_station ORDER BY position`)
	if err != nil {
		return nil, err
	}
	items := []BrowseItem{}
	var id, name string
	_, err = pgx.ForEachRow(rows, []any{&id, &name}, func() error {
		if n := stationsPer[id]; n > 0 {
			items = append(items, BrowseItem{ID: "genre:" + id, Title: name, Subtitle: plural(n, "station"), Browsable: true})
		}
		return nil
	})
	return items, err
}

func (s *Service) playlistFolders(ctx context.Context, userID string) ([]BrowseItem, error) {
	list, err := s.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	stationsPer := map[string]int{}
	for _, st := range list {
		for _, p := range st.PlaylistIDs {
			stationsPer[p]++
		}
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text, name FROM playlists ORDER BY lower(name)`)
	if err != nil {
		return nil, err
	}
	items := []BrowseItem{}
	var id, name string
	_, err = pgx.ForEachRow(rows, []any{&id, &name}, func() error {
		if n := stationsPer[id]; n > 0 {
			items = append(items, BrowseItem{ID: "playlist:" + id, Title: name, Subtitle: plural(n, "station"), Browsable: true})
		}
		return nil
	})
	return items, err
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// SearchHit is a station that answers a spoken or typed query.
type SearchHit struct {
	BrowseItem
	// Reason says what matched: station, genre, playlist or artist.
	Reason string `json:"reason"`
}

// voiceNoise strips the words around a voice request ("play techno radio on OwnWave").
var voiceNoise = regexp.MustCompile(`(?i)^\s*(play|start|put on|listen to)\s+|\s+(on|in|from)\s+ownwave\s*$|\s+(station|radio|music|mix)\s*$`)

// CleanQuery reduces a voice request to what to play.
func CleanQuery(q string) string {
	q = strings.TrimSpace(q)
	for {
		next := strings.TrimSpace(voiceNoise.ReplaceAllString(q, ""))
		if next == q {
			return q
		}
		q = next
	}
}

const maxSearchHits = 10

// Search finds stations to play for a query, best first: stations by
// name, then stations with a matching genre or playlist, then stations
// holding the most tracks of a matching artist.
func (s *Service) Search(ctx context.Context, userID, query string) ([]SearchHit, error) {
	q := strings.ToLower(CleanQuery(query))
	if q == "" {
		return []SearchHit{}, nil
	}
	list, err := s.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	names, err := s.genreNames(ctx)
	if err != nil {
		return nil, err
	}

	type scored struct {
		hit   SearchHit
		score float64
	}
	best := map[string]scored{}
	add := func(st Station, reason string, score float64) {
		if cur, ok := best[st.ID]; !ok || score > cur.score {
			best[st.ID] = scored{SearchHit{StationItem(st, names), reason}, score}
		}
	}

	matchingGenres := map[string]bool{}
	for id, name := range names {
		if n := strings.ToLower(name); strings.Contains(n, q) || strings.Contains(q, n) || strings.ReplaceAll(id, "_", " ") == q {
			matchingGenres[id] = true
		}
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text FROM playlists WHERE lower(name) LIKE '%' || $1 || '%'`, q)
	if err != nil {
		return nil, err
	}
	playlistIDs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}

	for _, st := range list {
		name := strings.ToLower(st.Name)
		switch {
		case name == q:
			add(st, "station", 100)
		case strings.Contains(name, q):
			add(st, "station", 80)
		}
		for _, g := range st.Genres {
			if matchingGenres[g] {
				// Prefer stations that are only that genre.
				add(st, "genre", 60-float64(len(st.Genres)))
			}
		}
		for _, p := range playlistIDs {
			if contains(st.PlaylistIDs, p) {
				add(st, "playlist", 50)
			}
		}
	}

	if len(best) < maxSearchHits {
		if err := s.addArtistHits(ctx, userID, q, list, func(st Station, share float64) { add(st, "artist", 10+30*share) }); err != nil {
			return nil, err
		}
	}

	out := make([]scored, 0, len(best))
	for _, v := range best {
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].hit.Title < out[j].hit.Title
	})
	hits := []SearchHit{}
	for i := 0; i < len(out) && i < maxSearchHits; i++ {
		hits = append(hits, out[i].hit)
	}
	return hits, nil
}

// addArtistHits reports, for each station, the share of its tracks by
// artists whose name matches q.
func (s *Service) addArtistHits(ctx context.Context, userID, q string, list []Station, add func(Station, float64)) error {
	for _, st := range list {
		if st.TrackCount == 0 {
			continue
		}
		args := Args{userID, q}
		var n int
		err := s.pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM tracks t JOIN artists a ON a.id = t.artist_id
			WHERE lower(a.name) LIKE '%' || $2 || '%' AND `+NotBanned("$1")+` AND `+st.Definition().Where(&args), args...).Scan(&n)
		if err != nil {
			return err
		}
		if n > 0 {
			add(st, float64(n)/float64(st.TrackCount))
		}
	}
	return nil
}

// CoverCandidates returns up to limit track ids of a station, one per
// album, in a stable order, to take the station's cover art from.
func (s *Service) CoverCandidates(ctx context.Context, userID string, st Station, limit int) ([]string, error) {
	args := Args{userID}
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM (
			SELECT DISTINCT ON (COALESCE(t.album_id::text, t.id::text)) t.id::text AS id, t.album_id
			FROM tracks t WHERE `+NotBanned("$1")+` AND `+st.Definition().Where(&args)+`
			ORDER BY COALESCE(t.album_id::text, t.id::text), t.id
		) x ORDER BY md5(id) LIMIT `+fmt.Sprint(limit), args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
