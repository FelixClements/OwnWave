package stations

import (
	"context"
	"time"
)

// QueuePageSize caps one queue response; the client refetches when it reaches
// the end. Songs played longest ago come first, so every song gets its turn.
const QueuePageSize = 200

// QueueTrack is one song of a station's queue, as clients see it.
type QueueTrack struct {
	ID                    string   `json:"id"`
	Title                 string   `json:"title"`
	Artist                *string  `json:"artist,omitempty"`
	Album                 *string  `json:"album,omitempty"`
	TrackNumber           *int     `json:"track_number,omitempty"`
	DurationSeconds       *float64 `json:"duration_seconds,omitempty"`
	SampleRate            *int     `json:"sample_rate,omitempty"`
	Channels              *int     `json:"channels,omitempty"`
	Loudness              *float64 `json:"loudness,omitempty"`
	BPM                   float64  `json:"bpm"`
	Key                   string   `json:"key"`
	Energy                float64  `json:"energy"`
	Valence               float64  `json:"valence"`
	OutroStartSeconds     float64  `json:"outro_start_seconds"`
	IdealCrossfadeSeconds float64  `json:"ideal_crossfade_seconds"`
	IntroStartSeconds     float64  `json:"intro_start_seconds"`
	OutroEndSeconds       float64  `json:"outro_end_seconds"`
	Position              int      `json:"position"`
	Liked                 bool     `json:"liked"`
}

// Queue returns up to QueuePageSize songs of the station for the user: the
// station's pool (Definition.where) less the songs the user banned or hid
// on it, with likes flagged, in the order orderQueue describes. History,
// bans, likes and hides count for a whole song, through any of its copies.
// ErrNotFound unless the user may play the station.
func (s *Service) Queue(ctx context.Context, userID, stationID string) ([]QueueTrack, error) {
	st, err := s.visible(ctx, userID, stationID)
	if err != nil {
		return nil, err
	}
	items, err := s.queuePool(ctx, userID, st)
	if err != nil {
		return nil, err
	}
	return orderQueue(items, s.recentHours, time.Now()), nil
}

// queuePool reads the songs the station plays for the user, each with when
// the user last played it on this station (listening_history, which
// stations only reads).
func (s *Service) queuePool(ctx context.Context, userID string, st Station) ([]queued, error) {
	args := sqlArgs{userID, st.ID}
	rows, err := s.pool.Query(ctx, `
		SELECT t.id::text, t.title, a.name, al.title, t.track_number,
		       t.duration_seconds, t.sample_rate, t.channels,
		       COALESCE(af.bpm, 0), COALESCE(af.key, ''), COALESCE(af.energy, 0), COALESCE(af.valence, 0), af.loudness,
		       COALESCE(af.outro_start_seconds, 0), COALESCE(af.ideal_crossfade_seconds, 0),
		       COALESCE(af.intro_start_seconds, 0), COALESCE(af.outro_end_seconds, 0),
		       `+liked("$1")+`,
		       lp.last_played
		FROM tracks t
		LEFT JOIN audio_features af ON af.track_id = t.id
		LEFT JOIN artists a ON t.artist_id = a.id
		LEFT JOIN albums al ON t.album_id = al.id
		LEFT JOIN (
			SELECT hs.song_id AS song, MAX(h.played_at) AS last_played
			FROM listening_history h
			JOIN songs hs ON hs.track_id = h.track_id
			WHERE h.user_id = $1::uuid AND h.station_id = $2::uuid
			GROUP BY 1
		) lp ON lp.song = t.id
		WHERE `+notBanned("$1")+` AND `+notHidden("$1", "$2")+` AND `+st.Definition().where(&args), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []queued
	for rows.Next() {
		var it queued
		q := &it.track
		if err := rows.Scan(&q.ID, &q.Title, &q.Artist, &q.Album,
			&q.TrackNumber, &q.DurationSeconds, &q.SampleRate, &q.Channels,
			&q.BPM, &q.Key, &q.Energy, &q.Valence, &q.Loudness,
			&q.OutroStartSeconds, &q.IdealCrossfadeSeconds,
			&q.IntroStartSeconds, &q.OutroEndSeconds,
			&q.Liked, &it.lastPlayed); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

type queued struct {
	track      QueueTrack
	lastPlayed *time.Time
}
