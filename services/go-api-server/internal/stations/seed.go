// Package stations owns station seeds: parsing, validation, and the SQL
// filter that turns a seed into the live pool of matching tracks.
package stations

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Types are the supported seed types (keep in sync with
// schemas/station-seed.schema.json).
var Types = []string{"track", "artist", "album", "cluster", "mood", "genre", "sub_genre", "uncategorized"}

// ErrInvalidSeed wraps every seed validation failure.
var ErrInvalidSeed = errors.New("invalid station seed")

// Seed is the filter stored in stations.seed_features. An empty seed matches
// the whole library.
type Seed struct {
	Type       string   `json:"type,omitempty"`
	TrackID    string   `json:"track_id,omitempty"`
	ArtistID   string   `json:"artist_id,omitempty"`
	AlbumID    string   `json:"album_id,omitempty"`
	ClusterID  *int     `json:"cluster_id,omitempty"`
	MainGenre  string   `json:"main_genre,omitempty"`
	SubGenre   string   `json:"sub_genre,omitempty"`
	MinBPM     *float64 `json:"min_bpm,omitempty"`
	MaxBPM     *float64 `json:"max_bpm,omitempty"`
	MinEnergy  *float64 `json:"min_energy,omitempty"`
	MaxEnergy  *float64 `json:"max_energy,omitempty"`
	MinValence *float64 `json:"min_valence,omitempty"`
	MaxValence *float64 `json:"max_valence,omitempty"`
}

// Request is the create/update station body sent by the web app.
type Request struct {
	Name       string   `json:"name"`
	SeedType   string   `json:"seed_type"`
	TrackID    string   `json:"track_id"`
	ArtistID   string   `json:"artist_id"`
	AlbumID    string   `json:"album_id"`
	ClusterID  *int     `json:"cluster_id"`
	MainGenre  string   `json:"main_genre"`
	SubGenre   string   `json:"sub_genre"`
	MinBPM     *float64 `json:"min_bpm"`
	MaxBPM     *float64 `json:"max_bpm"`
	MinEnergy  *float64 `json:"min_energy"`
	MaxEnergy  *float64 `json:"max_energy"`
	MinValence *float64 `json:"min_valence"`
	MaxValence *float64 `json:"max_valence"`
}

// HasSeed reports whether the request carries any seed fields; updates
// without them keep the station's existing seed.
func (r Request) HasSeed() bool {
	return r.SeedType != "" || r.MinBPM != nil || r.MaxBPM != nil ||
		r.MinEnergy != nil || r.MaxEnergy != nil || r.MinValence != nil || r.MaxValence != nil
}

// Seed converts the request into a validated seed. Only the identifier that
// belongs to the chosen seed type is kept.
func (r Request) Seed() (Seed, error) {
	s := Seed{
		Type:       r.SeedType,
		MinBPM:     r.MinBPM,
		MaxBPM:     r.MaxBPM,
		MinEnergy:  r.MinEnergy,
		MaxEnergy:  r.MaxEnergy,
		MinValence: r.MinValence,
		MaxValence: r.MaxValence,
	}
	switch r.SeedType {
	case "track":
		s.TrackID = r.TrackID
	case "artist":
		s.ArtistID = r.ArtistID
	case "album":
		s.AlbumID = r.AlbumID
	case "cluster":
		s.ClusterID = r.ClusterID
	case "genre":
		s.MainGenre = r.MainGenre
	case "sub_genre":
		s.MainGenre, s.SubGenre = r.MainGenre, r.SubGenre
	}
	return s, s.Validate()
}

// ParseStored decodes stations.seed_features, accepting the legacy
// "seed_type" key. A NULL/empty value is the whole-library seed.
func ParseStored(raw *string) (Seed, error) {
	var s Seed
	if raw == nil || strings.TrimSpace(*raw) == "" || *raw == "null" {
		return s, nil
	}
	var legacy struct {
		Seed
		SeedType string `json:"seed_type"`
	}
	if err := json.Unmarshal([]byte(*raw), &legacy); err != nil {
		return s, fmt.Errorf("%w: %v", ErrInvalidSeed, err)
	}
	s = legacy.Seed
	if s.Type == "" {
		s.Type = legacy.SeedType
	}
	return s, nil
}

// IsEmpty reports whether the seed matches the whole library.
func (s Seed) IsEmpty() bool { return s == (Seed{}) }

// JSON returns the value to store in stations.seed_features (nil for empty).
func (s Seed) JSON() ([]byte, error) {
	if s.IsEmpty() {
		return nil, nil
	}
	return json.Marshal(s)
}

func (s Seed) Validate() error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrInvalidSeed, fmt.Sprintf(format, args...))
	}
	requireUUID := func(field, v string) error {
		if _, err := uuid.Parse(v); err != nil {
			return invalid("%s must be a valid id", field)
		}
		return nil
	}

	switch s.Type {
	case "", "mood", "uncategorized":
	case "track":
		if err := requireUUID("track_id", s.TrackID); err != nil {
			return err
		}
	case "artist":
		if err := requireUUID("artist_id", s.ArtistID); err != nil {
			return err
		}
	case "album":
		if err := requireUUID("album_id", s.AlbumID); err != nil {
			return err
		}
	case "cluster":
		if s.ClusterID == nil {
			return invalid("cluster_id is required")
		}
	case "genre":
		if strings.TrimSpace(s.MainGenre) == "" {
			return invalid("main_genre is required")
		}
	case "sub_genre":
		if strings.TrimSpace(s.MainGenre) == "" || strings.TrimSpace(s.SubGenre) == "" {
			return invalid("main_genre and sub_genre are required")
		}
	default:
		return invalid("unsupported seed type %q", s.Type)
	}

	ranges := []struct {
		name     string
		min, max *float64
		upper    float64
	}{
		{"bpm", s.MinBPM, s.MaxBPM, 0},
		{"energy", s.MinEnergy, s.MaxEnergy, 1},
		{"valence", s.MinValence, s.MaxValence, 1},
	}
	for _, r := range ranges {
		for _, v := range []*float64{r.min, r.max} {
			if v != nil && (*v < 0 || (r.upper > 0 && *v > r.upper)) {
				return invalid("%s out of range", r.name)
			}
		}
		if r.min != nil && r.max != nil && *r.min > *r.max {
			return invalid("min_%s is greater than max_%s", r.name, r.name)
		}
	}
	return nil
}

// TitleCase mirrors how genre station names are displayed ("hip hop" ->
// "Hip Hop").
func TitleCase(s string) string {
	parts := strings.Split(s, " ")
	for i, p := range parts {
		if r := []rune(p); len(r) > 0 {
			parts[i] = strings.ToUpper(string(r[0])) + strings.ToLower(string(r[1:]))
		}
	}
	return strings.Join(parts, " ")
}
