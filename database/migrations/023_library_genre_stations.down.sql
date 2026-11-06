ALTER TABLE track_station_genres DROP COLUMN IF EXISTS corrected;
DROP TABLE IF EXISTS genre_corrections;
DROP TABLE IF EXISTS station_hidden_tracks;
ALTER TABLE stations DROP CONSTRAINT IF EXISTS stations_auto_genre_shared;
UPDATE stations SET all_playlists = TRUE WHERE auto_genre IS NOT NULL;
ALTER TABLE stations ADD CONSTRAINT stations_auto_genre_shared
    CHECK (auto_genre IS NULL OR (user_id IS NULL AND all_playlists));
DROP INDEX IF EXISTS idx_tracks_copy_of;
ALTER TABLE tracks
    DROP CONSTRAINT IF EXISTS tracks_copy_of_not_self,
    DROP COLUMN IF EXISTS copy_of;
