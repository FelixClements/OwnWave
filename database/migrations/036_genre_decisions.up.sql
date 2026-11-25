-- Genre engine v2 (ADR 0008, issue #137): each song's decided genre and
-- style, or that it is unsure, and why. The Python analytics service
-- (genre_engine.py) is the only writer. Until the genre v2 switch only the
-- bulk run on a copy of the database writes here; the live app keeps using
-- track_station_genres from the v1 engine.

CREATE TABLE track_genre_decisions (
    track_id UUID PRIMARY KEY REFERENCES tracks(id) ON DELETE CASCADE,
    -- Genre and style ids of station_genres (after the switch). genre is
    -- NULL for an unsure song, which joins no genre station.
    genre TEXT,
    style TEXT,
    -- What decided: a correction, agreeing strong sources, the release
    -- lean, the classifier, or nothing (unsure).
    source TEXT NOT NULL CHECK (source IN ('correction', 'strong_sources', 'release', 'classifier', 'unsure')),
    -- 1 for a correction or strong sources, the release share for the
    -- release lean, the vote share for the classifier; NULL when unsure.
    confidence REAL,
    unsure_reason TEXT CHECK (unsure_reason IN (
        'strong_sources_disagree', 'empty_fence', 'outside_fence',
        'release_vs_classifier', 'no_audio', 'classifier_unsure'
    )),
    -- The classifier is 80% or more sure of another genre than the song's
    -- specific genre tag: the tag is kept and the song is queued for review.
    review_reason TEXT CHECK (review_reason IN ('classifier_disagrees')),
    -- The song's fence (genres it may get); NULL when nothing narrowed it.
    fence TEXT[],
    -- Per-source "why" breakdown: each source's answer, the fence, the
    -- release lean, the classifier's vote share and runner-ups, the style.
    why JSONB NOT NULL DEFAULT '{}',
    catalog_version TEXT NOT NULL,
    decided_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT track_genre_decisions_unsure CHECK ((source = 'unsure') = (genre IS NULL)),
    CONSTRAINT track_genre_decisions_unsure_reason CHECK ((source = 'unsure') = (unsure_reason IS NOT NULL)),
    CONSTRAINT track_genre_decisions_style CHECK (style IS NULL OR genre IS NOT NULL)
);
CREATE INDEX idx_track_genre_decisions_genre ON track_genre_decisions (genre) WHERE genre IS NOT NULL;
CREATE INDEX idx_track_genre_decisions_style ON track_genre_decisions (style) WHERE style IS NOT NULL;

-- A correction now applies to a song (track_id, the song's id), a release
-- (release_id, new) or an artist (artist). The most specific one wins.
ALTER TABLE genre_corrections ADD COLUMN release_id UUID REFERENCES releases(id) ON DELETE CASCADE;
ALTER TABLE genre_corrections DROP CONSTRAINT genre_corrections_one_target;
ALTER TABLE genre_corrections ADD CONSTRAINT genre_corrections_one_target
    CHECK (num_nonnulls(track_id, artist, release_id) = 1);
CREATE UNIQUE INDEX idx_genre_corrections_release ON genre_corrections (release_id) WHERE release_id IS NOT NULL;

-- The admin review queue (ADR 0008 decision 4), one row per song (best
-- copy) not decided by a correction. Grouped by release and ranked:
--   1 songs listeners hid on a genre station,
--   2 unsure songs,
--   3 classifier-versus-tag disagreements,
--   4 the least sure songs on stations (release lean or classifier).
-- Read it ORDER BY position. A release's songs stay together, placed by the
-- release's best-ranked song.
CREATE VIEW genre_review_queue AS
WITH hides AS (
    SELECT s.song_id, COUNT(DISTINCT h.user_id)::int AS users
    FROM station_hidden_tracks h
    JOIN songs s ON s.track_id = h.track_id
    JOIN stations st ON st.id = h.station_id AND st.auto_genre IS NOT NULL
    GROUP BY 1
), items AS (
    SELECT d.track_id, t.release_id, COALESCE(t.release_id, d.track_id) AS release_group,
           d.genre, d.style, d.source, d.confidence, d.unsure_reason, d.review_reason,
           COALESCE(h.users, 0) AS hidden_by,
           CASE
               WHEN h.users > 0 THEN 1
               WHEN d.source = 'unsure' THEN 2
               WHEN d.review_reason IS NOT NULL THEN 3
               ELSE 4
           END AS priority
    FROM track_genre_decisions d
    JOIN songs s ON s.track_id = d.track_id AND s.best_copy
    JOIN tracks t ON t.id = d.track_id
    LEFT JOIN hides h ON h.song_id = s.song_id
    WHERE d.source <> 'correction'
      AND (h.users > 0
           OR d.source = 'unsure'
           OR d.review_reason IS NOT NULL
           OR (d.source IN ('release', 'classifier')
               AND EXISTS (SELECT 1 FROM station_genres g WHERE g.id = d.genre AND g.is_station)))
), grouped AS (
    SELECT i.*,
           MIN(priority) OVER w AS release_priority,
           MAX(hidden_by) OVER w AS release_hidden_by,
           MIN(COALESCE(confidence, 0)) OVER w AS release_confidence
    FROM items i
    WINDOW w AS (PARTITION BY release_group)
)
SELECT track_id, release_id, genre, style, source, confidence, unsure_reason, review_reason,
       hidden_by, priority, release_priority,
       ROW_NUMBER() OVER (
           ORDER BY release_priority, release_hidden_by DESC, release_confidence, release_group,
                    priority, hidden_by DESC, COALESCE(confidence, 0), track_id
       ) AS position
FROM grouped;
