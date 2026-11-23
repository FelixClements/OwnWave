-- Each song's nearest songs by audio embedding, kept up to date as the
-- library changes, so rebuilding a playlist's library picks is a join and
-- a new song rebuilds only the playlists it reaches (docs/adr/0007).
-- Go's pipeline (internal/playlists/neighbours.go) is the only writer. The
-- first background run after this migration fills both tables.

-- Changes whenever a list's contents change; playlists remember the
-- revisions of their songs' lists.
CREATE SEQUENCE song_neighbour_rev;

-- One row per best copy with an embedding: what its list was built from.
CREATE TABLE song_neighbour_lists (
    song_id UUID PRIMARY KEY REFERENCES tracks(id) ON DELETE CASCADE,
    -- track_audio_genres.analyzed_at of the embedding the list is for.
    analyzed_at TIMESTAMPTZ NOT NULL,
    -- The rules version (neighboursVersion); another one rebuilds the list.
    version INT NOT NULL,
    -- How many neighbours the list holds at most. A list with fewer holds
    -- every song at or above the similarity floor.
    depth INT NOT NULL,
    rev BIGINT NOT NULL DEFAULT nextval('song_neighbour_rev')
);

-- A song's nearest best copies at or above the similarity floor.
-- neighbour_id has no foreign key on purpose: when a neighbour is deleted
-- or becomes a copy, the next run finds its rows and rebuilds those lists,
-- which a cascade would hide.
CREATE TABLE song_neighbours (
    song_id UUID NOT NULL REFERENCES song_neighbour_lists(song_id) ON DELETE CASCADE,
    neighbour_id UUID NOT NULL,
    similarity DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (song_id, neighbour_id)
);
CREATE INDEX idx_song_neighbours_neighbour ON song_neighbours (neighbour_id);
