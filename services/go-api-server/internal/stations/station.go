// Package stations owns stations: a pool of synced Spotify playlists,
// optionally limited to station genres. A station plays the local matches
// of its playlists that carry one of its genres (primary or secondary).
//
// Stations created by admins are shared: everyone plays them, only admins
// change them. Other users' stations are personal.
package stations

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrInvalid wraps every station validation failure.
var ErrInvalid = errors.New("invalid station")

// ErrNotFound is returned for a station that does not exist or is not
// visible to the user.
var ErrNotFound = errors.New("station not found")

// ErrForbidden is returned when the user may not change the station.
var ErrForbidden = errors.New("only admins change shared stations")

const maxNameLen = 100

var genreIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

// Definition is what a station plays.
type Definition struct {
	PlaylistIDs []string `json:"playlist_ids"`
	// Genres limits the pool to these station genres; empty means all.
	Genres []string `json:"genres"`
}

// Station is a station as clients see it.
type Station struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Shared      bool      `json:"shared"`
	PlaylistIDs []string  `json:"playlist_ids"`
	Genres      []string  `json:"genres"`
	TrackCount  int       `json:"track_count"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Definition returns what the station plays.
func (s Station) Definition() Definition {
	return Definition{PlaylistIDs: s.PlaylistIDs, Genres: s.Genres}
}

// Request is the create/update body. On update, nil fields keep their
// current value.
type Request struct {
	Name        *string  `json:"name"`
	PlaylistIDs []string `json:"playlist_ids"`
	Genres      []string `json:"genres"`
}

// Normalize trims, dedupes and sorts the definition and checks its shape.
// Whether the playlists and genres exist is checked against the database.
func (d Definition) Normalize() (Definition, error) {
	var out Definition
	seen := map[string]bool{}
	for _, id := range d.PlaylistIDs {
		u, err := uuid.Parse(strings.TrimSpace(id))
		if err != nil {
			return out, fmt.Errorf("%w: playlist_ids must be valid ids", ErrInvalid)
		}
		if s := u.String(); !seen[s] {
			seen[s] = true
			out.PlaylistIDs = append(out.PlaylistIDs, s)
		}
	}
	if len(out.PlaylistIDs) == 0 {
		return out, fmt.Errorf("%w: pick at least one playlist", ErrInvalid)
	}
	out.Genres = []string{}
	for _, g := range d.Genres {
		g = strings.TrimSpace(g)
		if !genreIDPattern.MatchString(g) {
			return out, fmt.Errorf("%w: unknown genre %q", ErrInvalid, g)
		}
		if !seen["genre:"+g] {
			seen["genre:"+g] = true
			out.Genres = append(out.Genres, g)
		}
	}
	sort.Strings(out.PlaylistIDs)
	sort.Strings(out.Genres)
	return out, nil
}

// PoolKey identifies a playlist pool: its sorted ids joined by commas.
func PoolKey(playlistIDs []string) string {
	ids := append([]string(nil), playlistIDs...)
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

func validName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%w: name is required", ErrInvalid)
	}
	if len([]rune(name)) > maxNameLen {
		return "", fmt.Errorf("%w: name is longer than %d characters", ErrInvalid, maxNameLen)
	}
	return name, nil
}
