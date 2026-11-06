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
// the end. Songs played longest ago come first, so every song gets its turn.
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

// BuildQueue returns up to QueuePageSize songs from the station's live pool,
// in the order orderQueue describes. History, bans, likes and hides count
// for a whole song, through any of its copies.
func (s *Service) BuildQueue(ctx context.Context, userID, stationID string, def stations.Definition, recentHours int) ([]TrackWithFeatures, error) {
	args := stations.Args{userID, stationID}
	joins := `
		LEFT JOIN audio_features af ON af.track_id = t.id
		LEFT JOIN artists a ON t.artist_id = a.id
		LEFT JOIN albums al ON t.album_id = al.id
		LEFT JOIN (
			SELECT COALESCE(ht.copy_of, ht.id) AS song, MAX(h.played_at) AS last_played
			FROM listening_history h
			JOIN tracks ht ON ht.id = h.track_id
			WHERE h.user_id = $1::uuid AND h.station_id = $2::uuid
			GROUP BY 1
		) lp ON lp.song = t.id`
	rows, err := s.pool.Query(ctx, `
		SELECT t.id::text, t.title, a.name, al.title, t.path, t.track_number,
		       t.duration_seconds, t.sample_rate, t.channels,
		       COALESCE(af.bpm, 0), COALESCE(af.key, ''), COALESCE(af.energy, 0), COALESCE(af.valence, 0), af.loudness,
		       COALESCE(af.outro_start_seconds, 0), COALESCE(af.ideal_crossfade_seconds, 0),
		       COALESCE(af.intro_start_seconds, 0), COALESCE(af.outro_end_seconds, 0),
		       `+stations.Liked("$1")+`,
		       lp.last_played
		FROM tracks t`+joins+`
		WHERE `+stations.NotBanned("$1")+` AND `+stations.NotHidden("$1", "$2")+` AND `+def.Where(&args), args...)
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

// playedAgeGroup is the width of the age groups orderQueue shuffles within.
const playedAgeGroup = 7 * 24 * time.Hour

// orderQueue dedupes by title+artist, then orders songs so every one gets a
// turn: never played on this station first, then played longest ago, then
// (after everything else) those played within recentHours, least recent
// first. Never-played songs and each week-wide age group are shuffled, so
// repeat listens still vary. It then spaces out artists and truncates to a page.
func orderQueue(items []queued, recentHours int, now time.Time) []TrackWithFeatures {
	cutoff := now.Add(-time.Duration(recentHours) * time.Hour)
	// Two entries with one title+artist play as one: it counts as played
	// when either was, so the other cannot bring it back early.
	first := make(map[string]int, len(items))
	unique := make([]queued, 0, len(items))
	for _, it := range items {
		artist := ""
		if it.track.Artist != nil {
			artist = *it.track.Artist
		}
		key := strings.ToLower(strings.TrimSpace(it.track.Title) + "|" + strings.TrimSpace(artist))
		if i, seen := first[key]; seen {
			if p := it.lastPlayed; p != nil && (unique[i].lastPlayed == nil || p.After(*unique[i].lastPlayed)) {
				unique[i].lastPlayed = p
			}
			continue
		}
		first[key] = len(unique)
		unique = append(unique, it)
	}

	var never, recent []queued
	played := map[int64][]queued{}
	for _, it := range unique {
		switch {
		case it.lastPlayed == nil:
			never = append(never, it)
		case it.lastPlayed.Before(cutoff):
			age := int64(now.Sub(*it.lastPlayed) / playedAgeGroup)
			played[age] = append(played[age], it)
		default:
			recent = append(recent, it)
		}
	}
	shuffle := func(list []queued) {
		rand.Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })
	}
	shuffle(never)
	ages := make([]int64, 0, len(played))
	for age := range played {
		ages = append(ages, age)
	}
	sort.Slice(ages, func(i, j int) bool { return ages[i] > ages[j] })
	fresh := never
	for _, age := range ages {
		shuffle(played[age])
		fresh = append(fresh, played[age]...)
	}
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
