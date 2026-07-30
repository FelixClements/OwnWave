CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE audio_features
    ADD COLUMN IF NOT EXISTS spectral_centroid FLOAT,
    ADD COLUMN IF NOT EXISTS mfcc FLOAT[],
    ADD COLUMN IF NOT EXISTS feature_vector VECTOR(33);

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

CREATE INDEX IF NOT EXISTS idx_audio_features_feature_vector ON audio_features USING ivfflat (feature_vector vector_cosine_ops);
