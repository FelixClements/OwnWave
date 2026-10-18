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
		"empty is whole library":     {req: Request{}, want: Seed{}},
		"track needs id":             {req: Request{SeedType: "track"}, wantErr: true},
		"track rejects bad uuid":     {req: Request{SeedType: "track", TrackID: "nope"}, wantErr: true},
		"genre needs main genre":     {req: Request{SeedType: "genre"}, wantErr: true},
		"sub genre needs both":       {req: Request{SeedType: "sub_genre", MainGenre: "rock"}, wantErr: true},
		"cluster needs id":           {req: Request{SeedType: "cluster"}, wantErr: true},
		"unknown type":               {req: Request{SeedType: "vibes"}, wantErr: true},
		"energy above 1":             {req: Request{SeedType: "mood", MaxEnergy: f(1.5)}, wantErr: true},
		"min above max":              {req: Request{MinBPM: f(150), MaxBPM: f(100)}, wantErr: true},
		"negative bpm":               {req: Request{MinBPM: f(-1)}, wantErr: true},
		"genre drops unrelated id":   {req: Request{SeedType: "genre", MainGenre: "rock", TrackID: validID}, want: Seed{Type: "genre", MainGenre: "rock"}},
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
	raw := `{"seed_type": "genre", "main_genre": "jazz"}`
	seed, err := ParseStored(&raw)
	require.NoError(t, err)
	require.Equal(t, Seed{Type: "genre", MainGenre: "jazz"}, seed)

	seed, err = ParseStored(nil)
	require.NoError(t, err)
	require.True(t, seed.IsEmpty())
}

func TestSeedJSONRoundTrip(t *testing.T) {
	seed := Seed{Type: "sub_genre", MainGenre: "rock", SubGenre: "punk", MinBPM: f(120)}
	raw, err := seed.JSON()
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"sub_genre","main_genre":"rock","sub_genre":"punk","min_bpm":120}`, string(raw))

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
	where := Seed{Type: "genre", MainGenre: "rock", MinBPM: f(100)}.Where(&args)
	require.Contains(t, where, "g.main_genre = $2")
	require.Contains(t, where, "COALESCE(af.bpm, 0) >= $3")
	require.Equal(t, Args{"user", "rock", 100.0}, args)

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

func TestTitleCase(t *testing.T) {
	require.Equal(t, "Hip Hop", TitleCase("hip hop"))
	require.Equal(t, "Électronique", TitleCase("électronique"))
}
