-- Stations are a pool of playlists, optionally limited to station genres.
-- The old seed stations (track, artist, album, cluster, mood, BPM/energy
-- ranges) are gone, with the k-means clusters and feature vectors only
-- they used.

DELETE FROM stations;
ALTER TABLE stations
    DROP COLUMN IF EXISTS seed_features,
    DROP COLUMN IF EXISTS is_auto,
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS last_refreshed_at,
    -- Empty: every genre of the pool.
    ADD COLUMN genres TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
DROP INDEX IF EXISTS idx_stations_auto_name;

CREATE TABLE station_playlists (
    station_id UUID NOT NULL REFERENCES stations(id) ON DELETE CASCADE,
    -- RESTRICT: a playlist cannot be deleted while a station uses it.
    playlist_id UUID NOT NULL REFERENCES playlists(id) ON DELETE RESTRICT,
    PRIMARY KEY (station_id, playlist_id)
);
CREATE INDEX idx_station_playlists_playlist ON station_playlists (playlist_id);

-- Station genres as the analytics service defines them (it keeps this
-- table in sync); lets the API name and validate genres.
CREATE TABLE station_genres (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    position INT NOT NULL,
    is_station BOOLEAN NOT NULL DEFAULT TRUE
);

-- Suggestions an admin dropped. pool_key is the sorted, comma-joined
-- playlist ids; genre '' is the all-genres suggestion.
CREATE TABLE station_suggestion_dismissals (
    pool_key TEXT NOT NULL,
    genre TEXT NOT NULL,
    dismissed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (pool_key, genre)
);

DROP TABLE IF EXISTS track_clusters;
DROP TABLE IF EXISTS cluster_centers;
DROP TABLE IF EXISTS feature_stats;
ALTER TABLE audio_features DROP COLUMN IF EXISTS feature_vector;
