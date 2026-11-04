DELETE FROM app_state WHERE key IN ('auto_stations', 'genres_scored');
DROP TABLE IF EXISTS station_refresh_runs;
DELETE FROM stations WHERE auto_genre IS NOT NULL OR all_playlists;
DROP INDEX IF EXISTS idx_stations_auto_genre;
ALTER TABLE stations
    DROP CONSTRAINT IF EXISTS stations_auto_genre_shared,
    DROP COLUMN IF EXISTS all_playlists,
    DROP COLUMN IF EXISTS auto_genre,
    DROP COLUMN IF EXISTS auto_active,
    DROP COLUMN IF EXISTS auto_excluded,
    DROP COLUMN IF EXISTS name_is_custom;
