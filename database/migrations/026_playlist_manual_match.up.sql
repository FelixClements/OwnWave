-- Admins fix what the playlist matcher gets wrong (issue #68).
--
-- A manual match row is never changed by the matcher: method 'manual' with a
-- track (picked by hand), or method 'none' with no track (marked "no local
-- file", which is also how a wrong match is cleared). A manual match whose
-- file is later deleted (track_id set NULL by the foreign key) goes back to
-- the matcher. Deleting the row hands the track back to the matcher.
ALTER TABLE spotify_track_matches ADD COLUMN IF NOT EXISTS manual BOOLEAN NOT NULL DEFAULT FALSE;

-- A hidden song leaves its playlist's stations, picks and counts. Syncs only
-- refresh details and last_seen_at, so it stays hidden.
ALTER TABLE playlist_items ADD COLUMN IF NOT EXISTS hidden BOOLEAN NOT NULL DEFAULT FALSE;
