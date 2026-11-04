-- Genre stations build and maintain themselves: one shared station per
-- station genre with enough matched tracks across all playlists. Their
-- pool is "every playlist", evaluated live, so a new playlist never
-- creates a second station for the same genre.
--
-- Every existing station is deleted and rebuilt: the genre stations by the
-- next refresh, custom ones by hand. Listening history and likes/bans stay
-- (listening_history.station_id is set to NULL).
DELETE FROM stations;
DELETE FROM station_suggestion_dismissals;

ALTER TABLE stations
    -- The pool is every playlist, including ones added later.
    ADD COLUMN all_playlists BOOLEAN NOT NULL DEFAULT FALSE,
    -- Set only on genre stations (one per station genre); custom stations are NULL.
    ADD COLUMN auto_genre TEXT,
    -- A genre station is listed and playable only while active. The refresh
    -- sets it; a hidden station keeps its id, name and history.
    ADD COLUMN auto_active BOOLEAN NOT NULL DEFAULT FALSE,
    -- An admin excluded the genre: the refresh never shows its station.
    ADD COLUMN auto_excluded BOOLEAN NOT NULL DEFAULT FALSE,
    -- An admin renamed the station: the refresh keeps the name.
    ADD COLUMN name_is_custom BOOLEAN NOT NULL DEFAULT FALSE,
    ADD CONSTRAINT stations_auto_genre_shared
        CHECK (auto_genre IS NULL OR (user_id IS NULL AND all_playlists));

-- At most one station per genre, whoever creates it.
CREATE UNIQUE INDEX idx_stations_auto_genre ON stations (auto_genre) WHERE auto_genre IS NOT NULL;

-- One row per run of the station refresh (playlist matching + genre
-- stations), newest kept, for the admin pipeline view.
CREATE TABLE station_refresh_runs (
    id BIGSERIAL PRIMARY KEY,
    trigger TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    -- Playlist tracks newly matched to a local file; NULL when matching did not run.
    matched INT,
    -- [{"genre", "name", "change": created|shown|hidden|renamed}]
    changes JSONB NOT NULL DEFAULT '[]',
    error TEXT
);

INSERT INTO app_state (key, value) VALUES ('auto_stations', '{"enabled": true}'::jsonb)
ON CONFLICT (key) DO NOTHING;
