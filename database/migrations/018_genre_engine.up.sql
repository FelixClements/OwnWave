-- Genre engine: raw source data per track/artist plus the blended station
-- genres stations are built from. Replaces the per-label rows in track_genres.

-- Raw genre tag(s) read from the file, e.g. "Techno; Hard Techno".
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS genre_tag TEXT;

-- Discogs400 output per track. activations follow the model's label order;
-- model_version changes when the model or the analysis windows change.
CREATE TABLE IF NOT EXISTS track_audio_genres (
    track_id UUID PRIMARY KEY REFERENCES tracks(id) ON DELETE CASCADE,
    model_version TEXT NOT NULL,
    activations REAL[] NOT NULL,
    embedding vector(1280),
    analyzed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- MusicBrainz tags per artist name (lowercased primary artist of a credit).
-- mbid is NULL when no confident match was found; the row still records the
-- lookup so it is not repeated on every scan.
CREATE TABLE IF NOT EXISTS musicbrainz_artists (
    name_key TEXT PRIMARY KEY,
    mbid TEXT,
    tags JSONB NOT NULL DEFAULT '[]',
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Blended result: a track's station genres. One primary per scored track.
CREATE TABLE IF NOT EXISTS track_station_genres (
    track_id UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    genre TEXT NOT NULL,
    score REAL NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (track_id, genre)
);
CREATE INDEX IF NOT EXISTS idx_track_station_genres_genre ON track_station_genres (genre);
CREATE UNIQUE INDEX IF NOT EXISTS idx_track_station_genres_primary
    ON track_station_genres (track_id) WHERE is_primary;

DROP TABLE IF EXISTS track_genres;
