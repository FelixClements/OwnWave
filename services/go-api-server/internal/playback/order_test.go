package playback

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func q(id, artist string) queued {
	return queued{track: TrackWithFeatures{Track: Track{ID: id, Title: id, Artist: &artist}}}
}

func ids(items []queued) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.track.ID
	}
	return out
}

func TestSpreadArtistsKeepsGapWhenPossible(t *testing.T) {
	in := []queued{q("a1", "A"), q("a2", "A"), q("b1", "B"), q("c1", "C"), q("d1", "D"), q("a3", "a ")}
	require.Equal(t, []string{"a1", "b1", "c1", "d1", "a2", "a3"}, ids(spreadArtists(in, 3)))
}

func TestSpreadArtistsFallsBackWhenPoolIsOneArtist(t *testing.T) {
	in := []queued{q("1", "A"), q("2", "A"), q("3", "A")}
	require.Equal(t, []string{"1", "2", "3"}, ids(spreadArtists(in, 3)))
}

func TestSpreadArtistsIgnoresMissingArtist(t *testing.T) {
	none := queued{track: TrackWithFeatures{Track: Track{ID: "x", Title: "x"}}}
	in := []queued{none, none, q("a", "A")}
	require.Len(t, spreadArtists(in, 3), 3)
}

func TestSpreadArtistsPlacesTheTightArtistInTime(t *testing.T) {
	// A greedy pass would play B..H first and then have only A left.
	in := []queued{q("b", "B"), q("c", "C"), q("d", "D"), q("e", "E"), q("f", "F"), q("g", "G"),
		q("a1", "A"), q("a2", "A"), q("a3", "A"), q("h", "H")}
	out := ids(spreadArtists(in, 3))
	var as []int
	for i, id := range out {
		if id[0] == 'a' {
			as = append(as, i)
		}
	}
	require.Len(t, as, 3)
	require.GreaterOrEqual(t, as[1]-as[0], 4, out)
	require.GreaterOrEqual(t, as[2]-as[1], 4, out)
}
