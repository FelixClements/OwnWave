package stations

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// --- review queue -------------------------------------------------------------

// GenreReviewSong is one song (best copy) on the review queue.
type GenreReviewSong struct {
	TrackID string  `json:"track_id"`
	Title   string  `json:"title"`
	Artist  *string `json:"artist,omitempty"`
	// Genre is the decided genre; nil for an unsure song.
	Genre *string `json:"genre"`
	Style *string `json:"style,omitempty"`
	// Source is what decided: strong_sources, release, classifier or
	// unsure; "blend" before the genre v2 switch.
	Source string `json:"source"`
	// Confidence is the release share, the classifier's vote share or (before
	// the switch) the blended score; nil when unsure.
	Confidence   *float32 `json:"confidence,omitempty"`
	UnsureReason *string  `json:"unsure_reason,omitempty"`
	ReviewReason *string  `json:"review_reason,omitempty"`
	// Suggestion is the genre the engine leans to when it did not pick it:
	// the classifier's (unsure or disagreeing with the tag), else the
	// release lean's.
	Suggestion *string `json:"suggestion,omitempty"`
	// HiddenBy is how many listeners hid the song on a genre station.
	HiddenBy int `json:"hidden_by"`
	// Priority is the queue rank: 1 hidden by listeners, 2 unsure,
	// 3 classifier versus tag, 4 least sure on a station.
	Priority int `json:"priority"`
}

// GenreReviewGroup is one release's songs on the queue; a song without a
// release is a group of its own.
type GenreReviewGroup struct {
	ReleaseID     *string           `json:"release_id,omitempty"`
	ReleaseTitle  *string           `json:"release_title,omitempty"`
	ReleaseArtist *string           `json:"release_artist,omitempty"`
	Compilation   bool              `json:"compilation"`
	Priority      int               `json:"priority"`
	Songs         []GenreReviewSong `json:"songs"`
}

// GenreReviewPage is one page of the review queue, paged by groups.
type GenreReviewPage struct {
	// Switched: the genre v2 engine decides genres. Before the switch the
	// queue lists the v1 blend's least sure songs, and corrections take no
	// release scope and no style.
	Switched bool               `json:"switched"`
	Groups   []GenreReviewGroup `json:"groups"`
	// Total counts groups and Songs songs on the whole queue.
	Total  int `json:"total"`
	Songs  int `json:"songs"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

// reviewQueueV2 is the queue of genre engine v2 (the genre_review_queue view,
// ADR 0008 decision 4), with the song's suggestion from its "why".
const reviewQueueV2 = `
	SELECT q.track_id, q.release_id, q.genre, q.style, q.source, q.confidence,
	       q.unsure_reason, q.review_reason, q.hidden_by, q.priority, q.position,
	       COALESCE(d.why #>> '{classifier,genre}', d.why #>> '{release,genre}') AS suggestion
	FROM genre_review_queue q
	JOIN track_genre_decisions d ON d.track_id = q.track_id
`

// reviewQueueV1 stands in before the switch: the v1 blend's songs that are
// not corrected, most often hidden on a genre station first, then the
// lowest scores. It has the view's columns.
const reviewQueueV1 = `
	WITH hides AS (
		SELECT hs.song_id AS song, COUNT(DISTINCT h.user_id)::int AS users
		FROM station_hidden_tracks h
		JOIN songs hs ON hs.track_id = h.track_id
		JOIN stations st ON st.id = h.station_id AND st.auto_genre IS NOT NULL
		GROUP BY 1
	), items AS (
		SELECT t.id AS track_id, t.release_id, g.genre, NULL::text AS style, 'blend'::text AS source,
		       g.score AS confidence, NULL::text AS unsure_reason, NULL::text AS review_reason,
		       COALESCE(h.users, 0) AS hidden_by,
		       CASE WHEN h.users > 0 THEN 1 ELSE 4 END AS priority
		FROM tracks t
		JOIN songs s ON s.track_id = t.id AND s.best_copy
		JOIN track_station_genres g ON g.track_id = t.id AND g.is_primary AND NOT g.corrected
		LEFT JOIN hides h ON h.song = t.id
		WHERE NOT EXISTS (SELECT 1 FROM genre_corrections c WHERE c.track_id = t.id)
	)
	SELECT i.*, ROW_NUMBER() OVER (ORDER BY priority, hidden_by DESC, confidence, track_id) AS position,
	       NULL::text AS suggestion
	FROM items i
`

// GenreReview returns one page of the review queue: limit groups from
// offset. A release's songs stay together, placed by its best-ranked song.
// Songs decided by a correction are never on it.
func (s *Service) GenreReview(ctx context.Context, offset, limit int) (GenreReviewPage, error) {
	switched, err := s.genreV2Switched(ctx)
	if err != nil {
		return GenreReviewPage{}, err
	}
	queue := reviewQueueV1
	if switched {
		queue = reviewQueueV2
	}
	page := GenreReviewPage{Switched: switched, Groups: []GenreReviewGroup{}, Offset: offset, Limit: limit}
	rows, err := s.pool.Query(ctx, `
		WITH q AS (
			SELECT q.*, COALESCE(q.release_id, q.track_id) AS grp FROM (`+queue+`) q
		), groups AS (
			SELECT grp, MIN(position) AS pos, MIN(priority) AS priority FROM q GROUP BY grp
		), page AS (
			SELECT * FROM groups ORDER BY pos OFFSET $1 LIMIT $2
		)
		SELECT (SELECT COUNT(*) FROM groups)::int, (SELECT COUNT(*) FROM q)::int,
		       q.grp::text, p.priority, q.release_id::text, r.title, r.album_artist, COALESCE(r.is_compilation, FALSE),
		       q.track_id::text, t.title, a.name, q.genre, q.style, q.source, q.confidence::real,
		       q.unsure_reason, q.review_reason, q.suggestion, q.hidden_by, q.priority
		FROM page p
		JOIN q ON q.grp = p.grp
		JOIN tracks t ON t.id = q.track_id
		LEFT JOIN artists a ON a.id = t.artist_id
		LEFT JOIN releases r ON r.id = q.release_id
		ORDER BY p.pos, q.position
	`, offset, limit)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	last := ""
	for rows.Next() {
		var (
			grp  string
			g    GenreReviewGroup
			song GenreReviewSong
		)
		if err := rows.Scan(&page.Total, &page.Songs, &grp, &g.Priority, &g.ReleaseID, &g.ReleaseTitle, &g.ReleaseArtist, &g.Compilation,
			&song.TrackID, &song.Title, &song.Artist, &song.Genre, &song.Style, &song.Source, &song.Confidence,
			&song.UnsureReason, &song.ReviewReason, &song.Suggestion, &song.HiddenBy, &song.Priority); err != nil {
			return page, err
		}
		// Only suggest what the engine did not already pick.
		if song.Suggestion != nil && song.Genre != nil && *song.Suggestion == *song.Genre {
			song.Suggestion = nil
		}
		if grp != last {
			g.Songs = []GenreReviewSong{}
			page.Groups = append(page.Groups, g)
			last = grp
		}
		cur := &page.Groups[len(page.Groups)-1]
		cur.Songs = append(cur.Songs, song)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	if len(page.Groups) == 0 && offset > 0 {
		// Past the end: still report the totals.
		err = s.pool.QueryRow(ctx, `
			SELECT COUNT(DISTINCT COALESCE(release_id, track_id))::int, COUNT(*)::int FROM (`+queue+`) q
		`).Scan(&page.Total, &page.Songs)
	}
	return page, err
}

// --- one song's decision ("why") --------------------------------------------------

// GenreDecision is how genre engine v2 decided one song, with the "why"
// breakdown it stored: each source's answer and fence, the fence, the
// release lean, the classifier's vote and where the style came from.
type GenreDecision struct {
	TrackID      string          `json:"track_id"`
	Title        string          `json:"title"`
	Artist       *string         `json:"artist,omitempty"`
	GenreTag     *string         `json:"genre_tag,omitempty"`
	ReleaseID    *string         `json:"release_id,omitempty"`
	ReleaseTitle *string         `json:"release_title,omitempty"`
	Genre        *string         `json:"genre"`
	Style        *string         `json:"style,omitempty"`
	Source       string          `json:"source"`
	Confidence   *float32        `json:"confidence,omitempty"`
	UnsureReason *string         `json:"unsure_reason,omitempty"`
	ReviewReason *string         `json:"review_reason,omitempty"`
	Fence        *[]string       `json:"fence,omitempty"`
	Why          json.RawMessage `json:"why"`
	DecidedAt    time.Time       `json:"decided_at"`
}

// GenreDecision returns a song's decision. ErrNotFound when the engine has
// not decided it (always before the genre v2 switch).
func (s *Service) GenreDecision(ctx context.Context, trackID string) (GenreDecision, error) {
	var d GenreDecision
	err := s.pool.QueryRow(ctx, `
		SELECT t.id::text, t.title, a.name, t.genre_tag, t.release_id::text, r.title,
		       g.genre, g.style, g.source, g.confidence, g.unsure_reason, g.review_reason,
		       g.fence, g.why, g.decided_at
		FROM track_genre_decisions g
		JOIN tracks t ON t.id = g.track_id
		LEFT JOIN artists a ON a.id = t.artist_id
		LEFT JOIN releases r ON r.id = t.release_id
		WHERE g.track_id = $1::uuid
	`, trackID).Scan(&d.TrackID, &d.Title, &d.Artist, &d.GenreTag, &d.ReleaseID, &d.ReleaseTitle,
		&d.Genre, &d.Style, &d.Source, &d.Confidence, &d.UnsureReason, &d.ReviewReason,
		&d.Fence, &d.Why, &d.DecidedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	return d, err
}

// --- overview --------------------------------------------------------------------

// GenreCatalogEntry is a genre or style of the catalog, with how many songs
// (best copies) have it.
type GenreCatalogEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Station: a genre (or style) that may get a station.
	Station bool `json:"station"`
	Songs   int  `json:"songs"`
	// Styles of a genre; always empty before the genre v2 switch.
	Styles []GenreCatalogEntry `json:"styles,omitempty"`
}

// GenreOverview counts the library's songs by genre and style, the unsure
// and flagged songs, and carries the classifier's test score.
type GenreOverview struct {
	Switched bool `json:"switched"`
	// Songs counts best copies; Decided those with a genre.
	Songs   int `json:"songs"`
	Decided int `json:"decided"`
	// Unsure, Flagged, WithStyle and ToReview are nil before the switch.
	Unsure    *int `json:"unsure,omitempty"`
	Flagged   *int `json:"flagged,omitempty"`
	WithStyle *int `json:"with_style,omitempty"`
	ToReview  *int `json:"to_review,omitempty"`
	// BySource counts decided-by (correction, strong_sources, release,
	// classifier, unsure); nil before the switch.
	BySource map[string]int      `json:"by_source,omitempty"`
	Genres   []GenreCatalogEntry `json:"genres"`
	// TestScore is the classifier's leave-one-out score on the trusted labels
	// (genre_engine._test_score), stored at each rescore; nil until the first.
	TestScore   json.RawMessage `json:"test_score,omitempty"`
	TestScoreAt *time.Time      `json:"test_score_at,omitempty"`
}

// GenreOverview returns the admin Genres page's Overview.
func (s *Service) GenreOverview(ctx context.Context) (GenreOverview, error) {
	switched, err := s.genreV2Switched(ctx)
	if err != nil {
		return GenreOverview{}, err
	}
	o := GenreOverview{Switched: switched, Genres: []GenreCatalogEntry{}}
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM songs WHERE best_copy`).Scan(&o.Songs); err != nil {
		return o, err
	}

	// Songs per catalog id (genre or style).
	counts := map[string]int{}
	countSQL := `
		SELECT g.genre, COUNT(*)::int FROM track_station_genres g
		JOIN songs s ON s.track_id = g.track_id AND s.best_copy
		WHERE g.is_primary GROUP BY 1`
	if switched {
		countSQL = `
			SELECT id, COUNT(*)::int FROM (
				SELECT d.genre AS id FROM track_genre_decisions d
				JOIN songs s ON s.track_id = d.track_id AND s.best_copy WHERE d.genre IS NOT NULL
				UNION ALL
				SELECT d.style FROM track_genre_decisions d
				JOIN songs s ON s.track_id = d.track_id AND s.best_copy WHERE d.style IS NOT NULL
			) x GROUP BY 1`
	}
	rows, err := s.pool.Query(ctx, countSQL)
	if err != nil {
		return o, err
	}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			rows.Close()
			return o, err
		}
		counts[id] = n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return o, err
	}

	catalogSQL := `SELECT id, name, is_station, NULL::text FROM station_genres ORDER BY position, id`
	if switched {
		catalogSQL = `
			SELECT c.id, c.name, c.is_station, c.parent_id FROM station_genres c
			LEFT JOIN station_genres p ON p.id = c.parent_id
			ORDER BY COALESCE(p.position, c.position), c.parent_id NULLS FIRST, c.position, c.id`
	}
	rows, err = s.pool.Query(ctx, catalogSQL)
	if err != nil {
		return o, err
	}
	index := map[string]int{}
	for rows.Next() {
		var e GenreCatalogEntry
		var parent *string
		if err := rows.Scan(&e.ID, &e.Name, &e.Station, &parent); err != nil {
			rows.Close()
			return o, err
		}
		e.Songs = counts[e.ID]
		if parent == nil {
			index[e.ID] = len(o.Genres)
			o.Genres = append(o.Genres, e)
		} else if i, ok := index[*parent]; ok {
			o.Genres[i].Styles = append(o.Genres[i].Styles, e)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return o, err
	}
	for _, g := range o.Genres {
		o.Decided += g.Songs
	}

	if switched {
		var unsure, flagged, withStyle, toReview int
		o.BySource = map[string]int{}
		rows, err := s.pool.Query(ctx, `
			SELECT d.source, COUNT(*)::int,
			       COUNT(*) FILTER (WHERE d.review_reason IS NOT NULL)::int,
			       COUNT(*) FILTER (WHERE d.style IS NOT NULL)::int
			FROM track_genre_decisions d
			JOIN songs s ON s.track_id = d.track_id AND s.best_copy
			GROUP BY 1`)
		if err != nil {
			return o, err
		}
		for rows.Next() {
			var source string
			var n, f, st int
			if err := rows.Scan(&source, &n, &f, &st); err != nil {
				rows.Close()
				return o, err
			}
			o.BySource[source] = n
			flagged += f
			withStyle += st
			if source == "unsure" {
				unsure = n
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return o, err
		}
		if err := s.pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM genre_review_queue`).Scan(&toReview); err != nil {
			return o, err
		}
		o.Unsure, o.Flagged, o.WithStyle, o.ToReview = &unsure, &flagged, &withStyle, &toReview
	}

	var at time.Time
	err = s.pool.QueryRow(ctx, `SELECT value, updated_at FROM app_state WHERE key = 'genre_test_score'`).Scan(&o.TestScore, &at)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		o.TestScore = nil
	case err != nil:
		return o, err
	default:
		o.TestScoreAt = &at
	}
	return o, nil
}
