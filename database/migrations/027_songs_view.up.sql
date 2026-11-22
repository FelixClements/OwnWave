-- A song is one recording, however many files of it the library has. songs
-- gives every track its song: song_id is the song's best copy, the file a
-- station plays (tracks.copy_of, or the track itself when that is NULL).
-- Read song identity here, never from tracks.copy_of, so the rule has one
-- home. See docs/adr/0002 and issue #111.
--
-- best_copy is song_id = track_id. It is spelled out so the planner can use
-- the statistics on copy_of: the expression form estimates badly.
CREATE VIEW songs AS
SELECT id AS track_id,
       COALESCE(copy_of, id) AS song_id,
       copy_of IS NULL AS best_copy
FROM tracks;
