-- Genres fetched from outside sources by ID (ADR 0008 decision 5, issue
-- #135). One row per looked-up recording, release, release group or Last.fm
-- track, also when the source had nothing, so a lookup is not repeated before
-- it is due again (about 6 months). The Python worker is the only writer.
--
-- Rows hold the source's own words. Translating them into genres and styles
-- is the genre engine's job.
CREATE TABLE outside_genres (
    -- mb_recording, mb_release and mb_release_group: ref is the MusicBrainz id.
    -- discogs_release: ref is the Discogs release id.
    -- lastfm_track: ref is the artist name and the title as tagged, joined by
    -- chr(31) (Last.fm is matched by exact artist and title).
    source TEXT NOT NULL CHECK (source IN (
        'mb_recording', 'mb_release', 'mb_release_group', 'discogs_release', 'lastfm_track'
    )),
    ref TEXT NOT NULL,
    -- FALSE when the source does not know the id (or the track).
    found BOOLEAN NOT NULL,
    -- [{"name": "deep house", "count": 3, "kind": "genre"}], kind being
    -- genre (MusicBrainz, Discogs), style (Discogs) or tag (Last.fm); count
    -- is votes (MusicBrainz), weight 0-100 (Last.fm) or null (Discogs).
    genres JSONB NOT NULL DEFAULT '[]',
    -- mb_release only: the Discogs release its URL relation links to.
    discogs_release_id TEXT,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (source, ref)
);
