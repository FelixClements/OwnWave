ALTER TABLE tracks
    DROP COLUMN IF EXISTS tags_version,
    DROP COLUMN IF EXISTS mb_artist_ids,
    DROP COLUMN IF EXISTS mb_recording_id,
    DROP COLUMN IF EXISTS isrc,
    DROP COLUMN IF EXISTS release_id;
DROP TABLE IF EXISTS releases;
