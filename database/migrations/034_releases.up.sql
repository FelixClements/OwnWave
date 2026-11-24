-- Releases and the outside ids the genre engine v2 matches by (ADR 0008,
-- issue #134). The Python scan is the only writer.
--
-- A release is one published album or single as a whole: the MusicBrainz
-- release when the file names one, otherwise the files in one folder with
-- the same album tag. Unlike albums ((track artist, album title)), a
-- compilation stays one release.
CREATE TABLE releases (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- MusicBrainz release id (MUSICBRAINZ_ALBUMID), when the files have one.
    mb_release_id TEXT,
    -- Folder of the files. Part of the key only without mb_release_id; a
    -- MusicBrainz release split over folders (CD1, CD2) keeps the first.
    folder TEXT NOT NULL,
    -- Album tag of the files.
    title TEXT NOT NULL,
    album_artist TEXT,
    mb_release_group_id TEXT,
    -- MusicBrainz release type as tagged, such as "album; compilation".
    mb_release_type TEXT,
    -- The tags say compilation: album artist "Various Artists", a
    -- MusicBrainz release type or the compilation flag.
    tagged_compilation BOOLEAN NOT NULL DEFAULT FALSE,
    -- tagged_compilation, or more than 3 distinct track artists. Kept up to
    -- date by the scan whenever the release's tracks change.
    is_compilation BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX releases_mb_release_id_key ON releases (mb_release_id)
    WHERE mb_release_id IS NOT NULL;
CREATE UNIQUE INDEX releases_folder_title_key ON releases (folder, title)
    WHERE mb_release_id IS NULL;

-- Tags read from each file. NULL until the file's tags are read with
-- tags_version 1 or later; the next scan re-reads every older file.
ALTER TABLE tracks
    ADD COLUMN release_id UUID REFERENCES releases(id) ON DELETE SET NULL,
    ADD COLUMN isrc TEXT,
    -- MUSICBRAINZ_TRACKID, which names the recording.
    ADD COLUMN mb_recording_id TEXT,
    ADD COLUMN mb_artist_ids TEXT[],
    -- Which tag reader (tags.TAGS_VERSION) filled the row; 0 = before #134.
    ADD COLUMN tags_version SMALLINT NOT NULL DEFAULT 0;

CREATE INDEX tracks_release_id_idx ON tracks (release_id);
CREATE INDEX tracks_mb_recording_id_idx ON tracks (mb_recording_id)
    WHERE mb_recording_id IS NOT NULL;
CREATE INDEX tracks_isrc_idx ON tracks (isrc) WHERE isrc IS NOT NULL;
