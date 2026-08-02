ALTER TABLE audio_features
    ADD COLUMN IF NOT EXISTS intro_start_seconds FLOAT,
    ADD COLUMN IF NOT EXISTS outro_end_seconds FLOAT;
