ALTER TABLE tracks
    ADD COLUMN copy_of UUID REFERENCES tracks(id) ON DELETE SET NULL,
    ADD CONSTRAINT tracks_copy_of_not_self CHECK (copy_of <> id);

UPDATE tracks t SET copy_of = c.copy_of
FROM track_copies c
WHERE c.track_id = t.id;

CREATE INDEX idx_tracks_copy_of ON tracks (copy_of) WHERE copy_of IS NOT NULL;

CREATE OR REPLACE VIEW songs AS
SELECT id AS track_id,
       COALESCE(copy_of, id) AS song_id,
       copy_of IS NULL AS best_copy
FROM tracks;

DROP TABLE IF EXISTS track_copies;
