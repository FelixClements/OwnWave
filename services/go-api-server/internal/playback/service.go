package playback

import (
	"context"
	"math/rand/v2"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ownwave/api/internal/stations"
)

// QueuePageSize caps one queue response; the client refetches when it reaches
// the end. Recently played tracks sort last, so every track gets its turn.
const QueuePageSize = 200

// Service records playback events and builds live station queues.
type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Record appends listening history; stationID (optional) must be the user's
// own station or a shared one.
func (s *Service) Record(ctx context.Context, userID, trackID, stationID string) error {
	if stationID != "" {
		var visible int
		if err := s.pool.QueryRow(ctx, `
			SELECT 1 FROM stations WHERE id = $1::uuid AND (user_id = $2::uuid OR user_id IS NULL)
		`, stationID, userID).Scan(&visible); err != nil {
			return err
		}
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO listening_history (user_id, track_id, station_id)
		VALUES ($1::uuid, $2, NULLIF($3, '')::uuid)
	`, userID, trackID, stationID)
	return err
}

// BuildQueue returns up to QueuePageSize tracks from the station's live pool:
// tracks not played on this station within recentHours come first in random
// order, followed by recently played ones, least recent first.
func (s *Service) BuildQueue(ctx context.Context, userID, stationID string, def stations.Definition, recentHours int) ([]TrackWithFeatures, error) {
	args := stations.Args{userID, stationID}
	joins := `
		LEFT JOIN audio_features af ON af.track_id = t.id
		LEFT JOIN artists a ON t.artist_id = a.id
		LEFT JOIN albums al ON t.album_id = al.id
		LEFT JOIN (
			SELECT track_id, MAX(played_at) AS last_played
			FROM listening_history
			WHERE user_id = $1::uuid AND station_id = $2::uuid
			GROUP BY track_id
		) lp ON lp.track_id = t.id`
	rows, err := s.pool.Query(ctx, `
		SELECT t.id::text, t.title, a.name, al.title, t.path, t.track_number,
		       t.duration_seconds, t.sample_rate, t.channels,
		       COALESCE(af.bpm, 0), COALESCE(af.key, ''), COALESCE(af.energy, 0), COALESCE(af.valence, 0), af.loudness,
		       COALESCE(af.outro_start_seconds, 0), COALESCE(af.ideal_crossfade_seconds, 0),
		       COALESCE(af.intro_start_seconds, 0), COALESCE(af.outro_end_seconds, 0),
		       EXISTS (SELECT 1 FROM track_feedback l WHERE l.user_id = $1::uuid AND l.track_id = t.id AND l.feedback = 'like'),
		       lp.last_played
		FROM tracks t`+joins+`
		WHERE `+stations.NotBanned("$1")+` AND `+def.Where(&args), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []queued
	for rows.Next() {
		var it queued
		q := &it.track
		if err := rows.Scan(&q.ID, &q.Title, &q.Artist, &q.Album, &q.Path,
			&q.TrackNumber, &q.DurationSeconds, &q.SampleRate, &q.Channels,
			&q.BPM, &q.Key, &q.Energy, &q.Valence, &q.Loudness,
			&q.OutroStartSeconds, &q.IdealCrossfadeSeconds,
			&q.IntroStartSeconds, &q.OutroEndSeconds,
			&q.Liked, &it.lastPlayed); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return orderQueue(items, recentHours, time.Now()), nil
}

type queued struct {
	track      TrackWithFeatures
	lastPlayed *time.Time
}

// artistGap is how many other tracks must play before an artist repeats,
// when the pool allows it.
const artistGap = 3

// spreadArtists reorders items so the same artist does not play twice
// within gap tracks, keeping the given order as far as it can. At each
// position it takes the first remaining track whose artist is not among
// the last gap artists, except that an artist with just enough slots
// left to keep its spacing goes first. When no track qualifies (e.g. a
// one-artist pool) it takes the first remaining track.
func spreadArtists(items []queued, gap int) []queued {
	out := make([]queued, 0, len(items))
	rest := append([]queued(nil), items...)
	remaining := map[string]int{}
	for _, it := range rest {
		if a := artistKey(it.track); a != "" {
			remaining[a]++
		}
	}
	recent := make([]string, 0, gap)
	for len(rest) > 0 {
		pick, found := 0, false
		for i, it := range rest {
			a := artistKey(it.track)
			if containsArtist(recent, a) {
				continue
			}
			// c tracks spaced gap apart need (c-1)*(gap+1)+1 slots.
			if a != "" && (remaining[a]-1)*(gap+1)+1 >= len(rest) {
				pick, found = i, true
				break
			}
			if !found {
				pick, found = i, true
			}
		}
		it := rest[pick]
		rest = append(rest[:pick], rest[pick+1:]...)
		out = append(out, it)
		if a := artistKey(it.track); a != "" {
			remaining[a]--
			recent = append(recent, a)
			if len(recent) > gap {
				recent = recent[1:]
			}
		}
	}
	return out
}

func artistKey(t TrackWithFeatures) string {
	if t.Artist == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(*t.Artist))
}

func containsArtist(recent []string, a string) bool {
	if a == "" {
		return false
	}
	for _, r := range recent {
		if r == a {
			return true
		}
	}
	return false
}

// orderQueue dedupes by title+artist, then puts fresh tracks (shuffled) ahead
// of recently played ones (least recent first), spaces out artists and
// truncates to a page.
func orderQueue(items []queued, recentHours int, now time.Time) []TrackWithFeatures {
	cutoff := now.Add(-time.Duration(recentHours) * time.Hour)
	seen := make(map[string]bool, len(items))
	var fresh, recent []queued
	for _, it := range items {
		artist := ""
		if it.track.Artist != nil {
			artist = *it.track.Artist
		}
		key := strings.ToLower(strings.TrimSpace(it.track.Title) + "|" + strings.TrimSpace(artist))
		if seen[key] {
			continue
		}
		seen[key] = true
		if it.lastPlayed == nil || it.lastPlayed.Before(cutoff) {
			fresh = append(fresh, it)
		} else {
			recent = append(recent, it)
		}
	}
	rand.Shuffle(len(fresh), func(i, j int) { fresh[i], fresh[j] = fresh[j], fresh[i] })
	sort.SliceStable(recent, func(i, j int) bool { return recent[i].lastPlayed.Before(*recent[j].lastPlayed) })

	ordered := spreadArtists(append(fresh, recent...), artistGap)
	if len(ordered) > QueuePageSize {
		ordered = ordered[:QueuePageSize]
	}
	queue := make([]TrackWithFeatures, len(ordered))
	for i, it := range ordered {
		queue[i] = it.track
		queue[i].Position = i
	}
	return queue
}
