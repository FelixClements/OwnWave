package stations

import (
	"fmt"
	"strings"
)

// SimilarPoolSize bounds a track-seeded station to the nearest neighbours of
// its seed track (plus the seed itself).
const SimilarPoolSize = 200

// Args accumulates positional SQL parameters ($1, $2, ...).
type Args []any

// Add appends v and returns its placeholder.
func (a *Args) Add(v any) string {
	*a = append(*a, v)
	return fmt.Sprintf("$%d", len(*a))
}

// Where returns an SQL predicate restricting `tracks t` joined with
// `audio_features af` to the seed's pool. Missing numeric features count as
// 0, as in the original Python filter.
func (s Seed) Where(args *Args, minGenreConfidence float64) string {
	var conds []string
	// confidentGenre is the body of an EXISTS over the track's genre
	// predictions at or above the configured confidence.
	confidentGenre := func() string {
		return "SELECT 1 FROM track_genres g WHERE g.track_id = t.id AND g.confidence >= " + args.Add(minGenreConfidence)
	}

	switch s.Type {
	case "track":
		seed := args.Add(s.TrackID)
		conds = append(conds, fmt.Sprintf(`(t.id = %[1]s::uuid OR t.id IN (
			SELECT n.track_id FROM audio_features n
			WHERE n.feature_vector IS NOT NULL AND n.track_id <> %[1]s::uuid
			  AND EXISTS (SELECT 1 FROM audio_features WHERE track_id = %[1]s::uuid AND feature_vector IS NOT NULL)
			ORDER BY n.feature_vector <=> (SELECT feature_vector FROM audio_features WHERE track_id = %[1]s::uuid)
			LIMIT %[2]d))`, seed, SimilarPoolSize))
	case "artist":
		conds = append(conds, "t.artist_id = "+args.Add(s.ArtistID)+"::uuid")
	case "album":
		conds = append(conds, "t.album_id = "+args.Add(s.AlbumID)+"::uuid")
	case "cluster":
		conds = append(conds, "EXISTS (SELECT 1 FROM track_clusters c WHERE c.track_id = t.id AND c.cluster_id = "+args.Add(*s.ClusterID)+")")
	case "genre":
		genre := confidentGenre()
		conds = append(conds, "EXISTS ("+genre+" AND g.main_genre = "+args.Add(s.MainGenre)+")")
	case "sub_genre":
		genre := confidentGenre()
		main := args.Add(s.MainGenre)
		conds = append(conds, "EXISTS ("+genre+" AND g.main_genre = "+main+" AND g.sub_genre = "+args.Add(s.SubGenre)+")")
	case "uncategorized":
		conds = append(conds, "NOT EXISTS ("+confidentGenre()+")")
	}

	numeric := []struct {
		column string
		value  *float64
		op     string
	}{
		{"af.bpm", s.MinBPM, ">="}, {"af.bpm", s.MaxBPM, "<="},
		{"af.energy", s.MinEnergy, ">="}, {"af.energy", s.MaxEnergy, "<="},
		{"af.valence", s.MinValence, ">="}, {"af.valence", s.MaxValence, "<="},
	}
	for _, n := range numeric {
		if n.value != nil {
			conds = append(conds, fmt.Sprintf("COALESCE(%s, 0) %s %s", n.column, n.op, args.Add(*n.value)))
		}
	}

	if len(conds) == 0 {
		return "TRUE"
	}
	return strings.Join(conds, " AND ")
}
