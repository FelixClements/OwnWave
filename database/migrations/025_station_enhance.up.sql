-- A playlist station plays only its playlists' tracks. Enhance mixes in the
-- playlists' library picks: songs that sound like them. Off for every
-- station, including existing ones, which played picks until now.
ALTER TABLE stations ADD COLUMN enhance BOOLEAN NOT NULL DEFAULT false;
