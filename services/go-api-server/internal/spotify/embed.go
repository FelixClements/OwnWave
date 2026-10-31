// Package spotify reads public Spotify playlists from their embed page.
//
// Spotify's Web API refuses Spotify-owned (editorial) playlists to new apps,
// but the public embed page carries the playlist's current tracks (up to
// about 100) as JSON. That page is not a documented API: when its shape
// changes, Parse returns an error instead of an empty playlist.
package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// ErrInvalidURL is returned for input that names no Spotify playlist.
var ErrInvalidURL = errors.New("not a Spotify playlist URL")

// FetchError is a failure to load or read a playlist from Spotify.
type FetchError struct{ Err error }

func (e *FetchError) Error() string { return e.Err.Error() }
func (e *FetchError) Unwrap() error { return e.Err }

// Playlist is the playlist as the embed page shows it right now.
type Playlist struct {
	ID     string
	Name   string
	Tracks []Track
}

type Track struct {
	ID         string
	Title      string
	Artists    []string
	DurationMS int
}

var playlistIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{22}$`)

// ParsePlaylistID accepts a playlist URL (open.spotify.com/playlist/…,
// …/embed/playlist/…, with or without an intl-xx segment or query), a
// spotify:playlist:… URI, or a bare id.
func ParsePlaylistID(input string) (string, error) {
	s := strings.TrimSpace(input)
	if rest, ok := strings.CutPrefix(s, "spotify:playlist:"); ok {
		s = rest
	} else if i := strings.Index(s, "/playlist/"); i >= 0 {
		s = s[i+len("/playlist/"):]
		if j := strings.IndexAny(s, "/?#"); j >= 0 {
			s = s[:j]
		}
	}
	if !playlistIDPattern.MatchString(s) {
		return "", ErrInvalidURL
	}
	return s, nil
}

// Client fetches embed pages.
type Client struct {
	HTTP    *http.Client
	BaseURL string // defaults to https://open.spotify.com
}

// FetchPlaylist downloads and parses the embed page of a public playlist.
// Failures are *FetchError.
func (c Client) FetchPlaylist(ctx context.Context, id string) (Playlist, error) {
	if !playlistIDPattern.MatchString(id) {
		return Playlist{}, ErrInvalidURL
	}
	p, err := c.fetch(ctx, id)
	if err != nil {
		return Playlist{}, &FetchError{Err: err}
	}
	return p, nil
}

func (c Client) fetch(ctx context.Context, id string) (Playlist, error) {
	base := c.BaseURL
	if base == "" {
		base = "https://open.spotify.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/embed/playlist/"+id, nil)
	if err != nil {
		return Playlist{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; OwnWave playlist sync)")
	req.Header.Set("Accept-Language", "en")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Playlist{}, fmt.Errorf("fetch playlist: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Playlist{}, fmt.Errorf("fetch playlist: spotify answered %s", resp.Status)
	}
	page, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return Playlist{}, fmt.Errorf("fetch playlist: %w", err)
	}
	return Parse(page)
}

var nextDataPattern = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

// Parse extracts the playlist from an embed page.
func Parse(page []byte) (Playlist, error) {
	m := nextDataPattern.FindSubmatch(page)
	if m == nil {
		return Playlist{}, errors.New("embed page has no playlist data (page format changed?)")
	}
	var data struct {
		Props struct {
			PageProps struct {
				State struct {
					Data struct {
						Entity struct {
							Type      string `json:"type"`
							ID        string `json:"id"`
							Name      string `json:"name"`
							TrackList []struct {
								URI        string `json:"uri"`
								Title      string `json:"title"`
								Subtitle   string `json:"subtitle"`
								Duration   int    `json:"duration"`
								EntityType string `json:"entityType"`
							} `json:"trackList"`
						} `json:"entity"`
					} `json:"data"`
				} `json:"state"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(m[1], &data); err != nil {
		return Playlist{}, fmt.Errorf("embed page data: %w", err)
	}
	e := data.Props.PageProps.State.Data.Entity
	if e.Type != "playlist" || e.ID == "" {
		return Playlist{}, errors.New("embed page holds no playlist (page format changed?)")
	}

	p := Playlist{ID: e.ID, Name: strings.TrimSpace(e.Name)}
	for _, t := range e.TrackList {
		id, ok := strings.CutPrefix(t.URI, "spotify:track:")
		if !ok || strings.TrimSpace(t.Title) == "" {
			continue // podcast episodes, local files
		}
		p.Tracks = append(p.Tracks, Track{
			ID:         id,
			Title:      strings.TrimSpace(t.Title),
			Artists:    splitArtists(t.Subtitle),
			DurationMS: t.Duration,
		})
	}
	if len(p.Tracks) == 0 {
		return Playlist{}, errors.New("embed page lists no tracks (page format changed?)")
	}
	return p, nil
}

// splitArtists splits the embed subtitle "A, B" into names.
func splitArtists(subtitle string) []string {
	var out []string
	for _, a := range strings.Split(subtitle, ",") {
		if a = strings.TrimSpace(strings.ReplaceAll(a, " ", " ")); a != "" {
			out = append(out, a)
		}
	}
	return out
}
