-- Genre stations play the whole library, not only playlist matches. Copies
-- of a song are tracked (no file is touched), wrong genres can be corrected,
-- and a user can hide a song on one station. See docs/adr/0002.

-- NULL: this file is its song's best copy (most files have no other copy).
-- Otherwise the best copy of the same song. A station plays best copies only;
-- bans, likes, history and hides count for the whole song.
ALTER TABLE tracks
    ADD COLUMN copy_of UUID REFERENCES tracks(id) ON DELETE SET NULL,
    ADD CONSTRAINT tracks_copy_of_not_self CHECK (copy_of <> id);
CREATE INDEX idx_tracks_copy_of ON tracks (copy_of) WHERE copy_of IS NOT NULL;

-- A genre station's pool is the library, so it no longer has a playlist pool.
-- Station ids, names, history and feedback stay.
ALTER TABLE stations DROP CONSTRAINT stations_auto_genre_shared;
UPDATE stations SET all_playlists = FALSE WHERE auto_genre IS NOT NULL;
ALTER TABLE stations ADD CONSTRAINT stations_auto_genre_shared
    CHECK (auto_genre IS NULL OR (user_id IS NULL AND NOT all_playlists));

-- A user hid a song on one station: a wrong fit there, fine elsewhere.
CREATE TABLE station_hidden_tracks (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    station_id UUID NOT NULL REFERENCES stations(id) ON DELETE CASCADE,
    track_id UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, station_id, track_id)
);
CREATE INDEX idx_station_hidden_tracks_track ON station_hidden_tracks (track_id);

-- An admin's genre for a song (track_id) or for every track of an artist
-- (artist, as typed). Genre scoring applies these last: a track correction
-- wins over an artist correction, which wins over the blended score.
-- They live apart from track_station_genres because scoring rewrites that.
CREATE TABLE genre_corrections (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    track_id UUID REFERENCES tracks(id) ON DELETE CASCADE,
    artist TEXT,
    genre TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT genre_corrections_one_target CHECK ((track_id IS NULL) <> (artist IS NULL))
);
CREATE UNIQUE INDEX idx_genre_corrections_track ON genre_corrections (track_id) WHERE track_id IS NOT NULL;
CREATE UNIQUE INDEX idx_genre_corrections_artist ON genre_corrections (lower(artist)) WHERE artist IS NOT NULL;

-- The genre came from a correction, not the blend.
ALTER TABLE track_station_genres ADD COLUMN corrected BOOLEAN NOT NULL DEFAULT FALSE;
