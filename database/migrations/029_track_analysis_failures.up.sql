-- Analysis that failed for a track, so a scan doesn't retry it every time
-- (ADR 0006, #119). Python owns this table (ADR 0005).
--
-- kind 'features': the audio feature analyzers failed. Retried when the
-- file's size or mtime changes (the row is deleted then) or on a force scan.
-- kind 'genre': the audio genre model returned nothing. model_version is the
-- model that failed; a new model version retries it.
CREATE TABLE track_analysis_failures (
    track_id UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('features', 'genre')),
    model_version TEXT,
    error TEXT NOT NULL,
    failed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (track_id, kind)
);
