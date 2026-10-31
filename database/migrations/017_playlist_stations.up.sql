-- Stations are built from synced Spotify playlists instead of genre
-- predictions. Genre stations are removed; their history keeps its rows
-- (listening_history.station_id is set to NULL).
DELETE FROM stations
WHERE COALESCE(seed_features->>'type', seed_features->>'seed_type')
      IN ('genre', 'sub_genre', 'uncategorized');

-- A station without an owner is shared: every user can play it, only admins
-- change it.
ALTER TABLE stations ALTER COLUMN user_id DROP NOT NULL;

CREATE TABLE IF NOT EXISTS playlists (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    spotify_id TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_synced_at TIMESTAMPTZ,
    last_sync_error TEXT
);

-- Every track a playlist has ever listed. Syncs only add rows, so a playlist
-- grows as Spotify rotates it.
CREATE TABLE IF NOT EXISTS playlist_items (
    playlist_id UUID NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    spotify_track_id TEXT NOT NULL,
    title TEXT NOT NULL,
    artists TEXT[] NOT NULL DEFAULT '{}',
    duration_ms INT,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (playlist_id, spotify_track_id)
);
CREATE INDEX IF NOT EXISTS idx_playlist_items_spotify_track ON playlist_items(spotify_track_id);

-- The local file each Spotify track resolves to, shared by all playlists.
-- track_id is NULL while no local file matches.
CREATE TABLE IF NOT EXISTS spotify_track_matches (
    spotify_track_id TEXT PRIMARY KEY,
    track_id UUID REFERENCES tracks(id) ON DELETE SET NULL,
    method TEXT NOT NULL,
    matched_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_spotify_track_matches_track ON spotify_track_matches(track_id);
