ALTER TABLE tracks
    ADD COLUMN IF NOT EXISTS file_size BIGINT,
    ADD COLUMN IF NOT EXISTS file_mtime TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_tracks_file_mtime ON tracks(file_mtime);
