// Package stations owns stations. There are two kinds.
//
// Custom stations are made by hand: admins make shared ones, everyone else
// personal ones. A custom station's pool is one or more synced Spotify
// playlists: their local matches and library picks, optionally limited to
// station genres (primary or secondary).
//
// Genre stations are made by Refresh: one shared station per station genre
// with enough songs in the library. A genre station plays every song whose
// main genre is that genre, evaluated live.
//
// Every station plays songs: of several copies of a song, its best copy
// (tracks.copy_of). Bans, likes, history and hides count for the whole song.
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

// ErrConflict is returned for a change that would clash with a genre
// station, e.g. a custom station that duplicates one.
var ErrConflict = errors.New("station conflict")

// Station kinds.
const (
	KindCustom = "custom"
	KindGenre  = "genre"
)

const maxNameLen = 100

var genreIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

// Definition is what a station plays.
type Definition struct {
	// Library: a genre station's pool, every song of the library whose main
	// genre is in Genres. Playlists play no part.
	Library bool `json:"-"`
	// AllPlaylists: the pool is every playlist, including ones added
	// later; PlaylistIDs is then ignored.
	AllPlaylists bool     `json:"all_playlists"`
	PlaylistIDs  []string `json:"playlist_ids"`
	// Genres limits the pool to these station genres; empty means all.
	Genres []string `json:"genres"`
	// Enhance mixes in the playlists' library picks: songs from the
	// library that sound like them.
	Enhance bool `json:"enhance"`
}

// Station is a station as clients see it.
type Station struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Shared bool   `json:"shared"`
	// Kind is KindCustom or KindGenre.
	Kind         string `json:"kind"`
	AllPlaylists bool   `json:"all_playlists"`
	// PlaylistIDs are the playlists the station plays; for AllPlaylists
	// stations, every playlist right now.
	PlaylistIDs []string  `json:"playlist_ids"`
	Genres      []string  `json:"genres"`
	Enhance     bool      `json:"enhance"`
	TrackCount  int       `json:"track_count"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Definition returns what the station plays.
func (s Station) Definition() Definition {
	return Definition{Library: s.Kind == KindGenre, AllPlaylists: s.AllPlaylists, PlaylistIDs: s.PlaylistIDs, Genres: s.Genres, Enhance: s.Enhance}
}

// Request is the create/update body. On update, nil fields keep their
// current value. A genre station only changes its name.
type Request struct {
	Name         *string  `json:"name"`
	AllPlaylists *bool    `json:"all_playlists"`
	PlaylistIDs  []string `json:"playlist_ids"`
	Genres       []string `json:"genres"`
	Enhance      *bool    `json:"enhance"`
}

// Normalize trims, dedupes and sorts the definition and checks its shape.
// Whether the playlists and genres exist is checked against the database.
func (d Definition) Normalize() (Definition, error) {
	out := Definition{AllPlaylists: d.AllPlaylists, PlaylistIDs: []string{}, Enhance: d.Enhance}
	seen := map[string]bool{}
	for _, id := range d.PlaylistIDs {
		if d.AllPlaylists {
			break
		}
		u, err := uuid.Parse(strings.TrimSpace(id))
		if err != nil {
			return out, fmt.Errorf("%w: playlist_ids must be valid ids", ErrInvalid)
		}
		if s := u.String(); !seen[s] {
			seen[s] = true
			out.PlaylistIDs = append(out.PlaylistIDs, s)
		}
	}
	if len(out.PlaylistIDs) == 0 && !out.AllPlaylists {
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
