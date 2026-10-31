DROP TABLE IF EXISTS spotify_track_matches;
DROP TABLE IF EXISTS playlist_items;
DROP TABLE IF EXISTS playlists;

-- Shared stations have no owner to fall back to.
DELETE FROM stations WHERE user_id IS NULL;
ALTER TABLE stations ALTER COLUMN user_id SET NOT NULL;
