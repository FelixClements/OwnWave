-- A synced playlist's Spotify cover, downloaded once so clients never ask
-- Spotify for it. cover_url is where it came from: a sync downloads again
-- only when Spotify lists a different image.
ALTER TABLE playlists
    ADD COLUMN cover_url TEXT,
    ADD COLUMN cover_image BYTEA,
    ADD COLUMN cover_type TEXT;
