DROP TABLE IF EXISTS track_genres;

ALTER TABLE stations
    DROP COLUMN IF EXISTS is_auto,
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS last_refreshed_at;
