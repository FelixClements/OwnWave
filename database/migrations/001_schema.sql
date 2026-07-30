CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vector";

CREATE TABLE IF NOT EXISTS artists (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS albums (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    artist_id UUID REFERENCES artists(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    year INT,
    UNIQUE (artist_id, title)
);

CREATE TABLE IF NOT EXISTS tracks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    path TEXT NOT NULL UNIQUE,
    artist_id UUID REFERENCES artists(id) ON DELETE SET NULL,
    album_id UUID REFERENCES albums(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    track_number INT,
    duration_seconds FLOAT,
    sample_rate INT,
    channels INT,
    file_size BIGINT,
    file_mtime TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audio_features (
    track_id UUID PRIMARY KEY REFERENCES tracks(id) ON DELETE CASCADE,
    bpm FLOAT,
    key TEXT,
    loudness FLOAT,
    energy FLOAT,
    valence FLOAT,
    outro_start_seconds FLOAT,
    ideal_crossfade_seconds FLOAT,
    chroma JSONB,
    spectral_centroid FLOAT,
    mfcc FLOAT[],
    feature_vector VECTOR(33),
    analyzed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS stations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    seed_features JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS station_tracks (
    station_id UUID REFERENCES stations(id) ON DELETE CASCADE,
    track_id UUID REFERENCES tracks(id) ON DELETE CASCADE,
    position INT NOT NULL,
    played_at TIMESTAMPTZ,
    PRIMARY KEY (station_id, position)
);

CREATE TABLE IF NOT EXISTS scan_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    path TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    error TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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

CREATE INDEX IF NOT EXISTS idx_tracks_artist ON tracks(artist_id);
CREATE INDEX IF NOT EXISTS idx_tracks_album ON tracks(album_id);
CREATE INDEX IF NOT EXISTS idx_tracks_file_mtime ON tracks(file_mtime);
CREATE INDEX IF NOT EXISTS idx_audio_features_bpm ON audio_features(bpm);
CREATE INDEX IF NOT EXISTS idx_audio_features_feature_vector ON audio_features USING ivfflat (feature_vector vector_cosine_ops);
CREATE INDEX IF NOT EXISTS idx_station_tracks_station ON station_tracks(station_id);
