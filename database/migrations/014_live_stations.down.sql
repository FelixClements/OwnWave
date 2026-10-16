DROP INDEX IF EXISTS idx_listening_history_user_station;

CREATE INDEX IF NOT EXISTS idx_audio_features_feature_vector
    ON audio_features USING ivfflat (feature_vector vector_cosine_ops);

ALTER TABLE stations
    ADD COLUMN IF NOT EXISTS is_auto BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS source TEXT,
    ADD COLUMN IF NOT EXISTS last_refreshed_at TIMESTAMPTZ;
CREATE UNIQUE INDEX IF NOT EXISTS idx_stations_user_auto_name
    ON stations (user_id, name) WHERE is_auto = TRUE;

-- Membership is not restored; stations need a rebuild after rolling back.
CREATE TABLE IF NOT EXISTS station_tracks (
    station_id UUID REFERENCES stations(id) ON DELETE CASCADE,
    track_id UUID REFERENCES tracks(id) ON DELETE CASCADE,
    position INT NOT NULL,
    played_at TIMESTAMPTZ,
    PRIMARY KEY (station_id, position)
);
CREATE INDEX IF NOT EXISTS idx_station_tracks_station ON station_tracks(station_id);
