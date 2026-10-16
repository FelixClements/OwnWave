-- Stations are live: a station stores only its seed, and its tracks are
-- computed from that seed on every queue request. Stored membership and the
-- auto-station bookkeeping are no longer read or written.
DROP TABLE IF EXISTS station_tracks;

DROP INDEX IF EXISTS idx_stations_user_auto_name;
ALTER TABLE stations
    DROP COLUMN IF EXISTS is_auto,
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS last_refreshed_at;

-- The ivfflat index was built on an empty table (untrained lists), so any
-- query that used it would get poor recall. Similarity search is exact.
DROP INDEX IF EXISTS idx_audio_features_feature_vector;

-- Recently-played lookups for station queues.
CREATE INDEX IF NOT EXISTS idx_listening_history_user_station
    ON listening_history (user_id, station_id, track_id, played_at DESC);
