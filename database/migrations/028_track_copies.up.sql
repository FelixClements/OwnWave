-- Which library files are copies of one song moves out of tracks, which
-- Python owns, into track_copies, which Go owns (docs/adr/0005, issue #112).
-- Go's copy grouping (internal/playlists/copies.go) is its only writer.
--
-- A row: track_id is a copy, copy_of is its song's best copy. A track with
-- no row is its song's best copy, as tracks.copy_of IS NULL was. ON DELETE
-- CASCADE on copy_of keeps the old ON DELETE SET NULL result: when the best
-- copy goes, the copy stands alone until the next grouping run.
CREATE TABLE track_copies (
    track_id UUID PRIMARY KEY REFERENCES tracks(id) ON DELETE CASCADE,
    copy_of UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    CONSTRAINT track_copies_not_self CHECK (track_id <> copy_of)
);
CREATE INDEX idx_track_copies_copy_of ON track_copies (copy_of);

INSERT INTO track_copies (track_id, copy_of)
SELECT id, copy_of FROM tracks WHERE copy_of IS NOT NULL;

-- Same columns and rows as 027; only the source of copy_of changes.
-- best_copy is spelled out so the planner can use the statistics on
-- track_copies.copy_of: the expression form estimates badly.
CREATE OR REPLACE VIEW songs AS
SELECT t.id AS track_id,
       COALESCE(c.copy_of, t.id) AS song_id,
       c.copy_of IS NULL AS best_copy
FROM tracks t
LEFT JOIN track_copies c ON c.track_id = t.id;

DROP INDEX IF EXISTS idx_tracks_copy_of;
ALTER TABLE tracks
    DROP CONSTRAINT IF EXISTS tracks_copy_of_not_self,
    DROP COLUMN copy_of;
