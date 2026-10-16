//go:build integration

package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"ownwave/api/internal/testdb"
)

func TestSimilarTracksOrdersByDistanceWithTrackInfo(t *testing.T) {
	pool := testdb.NewPool(t)
	ctx := context.Background()
	db := NewDB(pool)

	add := func(title string, first float64, withVector bool) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO tracks (path, title) VALUES ($1, $1) RETURNING id::text`, "/m/"+title).Scan(&id))
		var vector any
		if withVector {
			vector = "[" + fmt.Sprint(first) + strings.Repeat(",0.1", 32) + "]"
		}
		_, err := pool.Exec(ctx, `INSERT INTO audio_features (track_id, feature_vector) VALUES ($1::uuid, $2::vector)`, id, vector)
		require.NoError(t, err)
		return id
	}
	seed := add("seed", 1, true)
	near := add("near", 0.9, true)
	far := add("far", -1, true)
	add("novector", 0, false)

	similar, err := db.SimilarTracks(ctx, seed, 10)
	require.NoError(t, err)
	require.Len(t, similar, 2)
	require.Equal(t, near, similar[0].TrackID)
	require.Equal(t, "/m/near", similar[0].Title)
	require.Equal(t, far, similar[1].TrackID)
	require.Less(t, similar[0].Distance, similar[1].Distance)

	none, err := db.SimilarTracks(ctx, add("lonely", 0, false), 10)
	require.NoError(t, err)
	require.Empty(t, none)
}
