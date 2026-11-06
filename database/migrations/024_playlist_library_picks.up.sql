-- Library songs that sound like a playlist, found with the audio embeddings.
-- A playlist's stations play them alongside its matched tracks, each with the
-- same chance. One song can be a pick for several playlists. See docs/adr/0002.
CREATE TABLE playlist_library_picks (
    playlist_id UUID NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    track_id UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    -- How many of the playlist's tracks have this song among their nearest.
    hits INT NOT NULL,
    -- Cosine similarity to the closest of them.
    similarity REAL NOT NULL,
    PRIMARY KEY (playlist_id, track_id)
);
CREATE INDEX idx_playlist_library_picks_track ON playlist_library_picks (track_id);

-- An admin removed a pick from a playlist: rebuilds keep it out.
CREATE TABLE playlist_pick_removals (
    playlist_id UUID NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    track_id UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (playlist_id, track_id)
);

-- What the playlist's picks were last built from; unchanged input, no rebuild.
ALTER TABLE playlists ADD COLUMN picks_input TEXT;
