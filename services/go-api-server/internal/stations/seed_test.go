package stations

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const validID = "6f1b8c6e-6a43-4e4b-9b1e-2b3f8d1a0c55"

func f(v float64) *float64 { return &v }

func TestRequestSeedValidation(t *testing.T) {
	cases := map[string]struct {
		req     Request
		wantErr bool
		want    Seed
	}{
		"empty is whole library":    {req: Request{}, want: Seed{}},
		"track needs id":            {req: Request{SeedType: "track"}, wantErr: true},
		"track rejects bad uuid":    {req: Request{SeedType: "track", TrackID: "nope"}, wantErr: true},
		"playlists need one":        {req: Request{SeedType: "playlists"}, wantErr: true},
		"playlists reject bad uuid": {req: Request{SeedType: "playlists", PlaylistIDs: []string{"nope"}}, wantErr: true},
		"genre is gone":             {req: Request{SeedType: "genre"}, wantErr: true},
		"cluster needs id":          {req: Request{SeedType: "cluster"}, wantErr: true},
		"unknown type":              {req: Request{SeedType: "vibes"}, wantErr: true},
		"energy above 1":            {req: Request{SeedType: "mood", MaxEnergy: f(1.5)}, wantErr: true},
		"min above max":             {req: Request{MinBPM: f(150), MaxBPM: f(100)}, wantErr: true},
		"negative bpm":              {req: Request{MinBPM: f(-1)}, wantErr: true},
		"artist drops unrelated id": {req: Request{SeedType: "artist", ArtistID: validID, TrackID: validID}, want: Seed{Type: "artist", ArtistID: validID}},
		"playlists canonical ids": {
			req:  Request{SeedType: "playlists", PlaylistIDs: []string{"6F1B8C6E-6A43-4E4B-9B1E-2B3F8D1A0C55", validID}},
			want: Seed{Type: "playlists", PlaylistIDs: []string{validID}},
		},
		"track keeps numeric filter": {req: Request{SeedType: "track", TrackID: validID, MinBPM: f(90)}, want: Seed{Type: "track", TrackID: validID, MinBPM: f(90)}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.req.Seed()
			if tc.wantErr {
				require.True(t, errors.Is(err, ErrInvalidSeed), "got %v", err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestParseStoredAcceptsLegacySeedType(t *testing.T) {
	raw := `{"seed_type": "artist", "artist_id": "` + validID + `"}`
	seed, err := ParseStored(&raw)
	require.NoError(t, err)
	require.Equal(t, Seed{Type: "artist", ArtistID: validID}, seed)

	seed, err = ParseStored(nil)
	require.NoError(t, err)
	require.True(t, seed.IsEmpty())
}

func TestSeedJSONRoundTrip(t *testing.T) {
	seed := Seed{Type: "playlists", PlaylistIDs: []string{validID}, MinBPM: f(120)}
	raw, err := seed.JSON()
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"playlists","playlist_ids":["`+validID+`"],"min_bpm":120}`, string(raw))

	s := string(raw)
	parsed, err := ParseStored(&s)
	require.NoError(t, err)
	require.Equal(t, seed, parsed)

	empty, err := Seed{}.JSON()
	require.NoError(t, err)
	require.Nil(t, empty)
}

func TestWhereBuildsParameterizedPredicates(t *testing.T) {
	args := Args{"user"}
	where := Seed{Type: "playlists", PlaylistIDs: []string{validID}, MinBPM: f(100)}.Where(&args)
	require.Contains(t, where, "pi.playlist_id = ANY($2::uuid[])")
	require.Contains(t, where, "COALESCE(af.bpm, 0) >= $3")
	require.Equal(t, Args{"user", []string{validID}, 100.0}, args)

	args = Args{}
	require.Equal(t, "TRUE", Seed{}.Where(&args))
	require.Empty(t, args)
}

func TestTypesMatchSchema(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "schemas", "station-seed.schema.json"))
	require.NoError(t, err)
	var schema struct {
		Properties struct {
			Type struct {
				Enum []string `json:"enum"`
			} `json:"type"`
		} `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(raw, &schema))
	require.ElementsMatch(t, schema.Properties.Type.Enum, Types)
}
