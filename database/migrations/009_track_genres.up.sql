CREATE TABLE IF NOT EXISTS track_genres (
    track_id UUID REFERENCES tracks(id) ON DELETE CASCADE,
    main_genre TEXT NOT NULL,
    sub_genre TEXT NOT NULL,
    confidence FLOAT NOT NULL,
    source TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (track_id, sub_genre, source)
);

CREATE INDEX IF NOT EXISTS idx_track_genres_track ON track_genres(track_id);
CREATE INDEX IF NOT EXISTS idx_track_genres_main ON track_genres(main_genre, sub_genre);
CREATE INDEX IF NOT EXISTS idx_track_genres_confidence ON track_genres(confidence);

ALTER TABLE stations
    ADD COLUMN IF NOT EXISTS is_auto BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS source TEXT,
    ADD COLUMN IF NOT EXISTS last_refreshed_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS idx_stations_auto_name
    ON stations(name) WHERE is_auto = TRUE;
