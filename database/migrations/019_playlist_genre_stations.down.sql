-- Restores the old columns and tables, empty. Stations are deleted: the
-- old seed format cannot be rebuilt from playlist/genre stations.
DELETE FROM stations;
DROP TABLE IF EXISTS station_suggestion_dismissals;
DROP TABLE IF EXISTS station_genres;
DROP TABLE IF EXISTS station_playlists;
ALTER TABLE stations
    DROP COLUMN IF EXISTS genres,
    DROP COLUMN IF EXISTS updated_at,
    ADD COLUMN IF NOT EXISTS seed_features JSONB,
    ADD COLUMN IF NOT EXISTS is_auto BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS source TEXT,
    ADD COLUMN IF NOT EXISTS last_refreshed_at TIMESTAMPTZ;
CREATE UNIQUE INDEX IF NOT EXISTS idx_stations_auto_name ON stations(name) WHERE is_auto = TRUE;

ALTER TABLE audio_features ADD COLUMN IF NOT EXISTS feature_vector VECTOR(33);
CREATE INDEX IF NOT EXISTS idx_audio_features_feature_vector ON audio_features USING ivfflat (feature_vector vector_cosine_ops);
CREATE TABLE IF NOT EXISTS track_clusters (
    track_id UUID PRIMARY KEY REFERENCES tracks(id) ON DELETE CASCADE,
    cluster_id INT NOT NULL
);
CREATE TABLE IF NOT EXISTS cluster_centers (
    cluster_id INT PRIMARY KEY,
    center_vector VECTOR(33) NOT NULL,
    track_count INT NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS feature_stats (
    name TEXT PRIMARY KEY,
    data JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
