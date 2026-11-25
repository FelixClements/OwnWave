-- Settings and last run of each outside genre source (issue #148): the
-- admin "Outside sources" page. A value saved here overrides the stack env
-- (MUSICBRAINZ_ENABLED, DISCOGS_ENABLED / DISCOGS_TOKEN, LASTFM_ENABLED /
-- LASTFM_API_KEY); NULL means the env decides, as before this table.
CREATE TABLE outside_sources (
    source TEXT PRIMARY KEY CHECK (source IN ('musicbrainz', 'discogs', 'lastfm')),
    -- NULL: the env switch decides.
    enabled BOOLEAN,
    -- The Discogs token or Last.fm key, encrypted by the Python service with
    -- AES-256-GCM (version byte, 12-byte nonce, ciphertext and tag). The key
    -- comes from env, never from the database, so a dump alone does not
    -- reveal the secret. NULL: the env value is used.
    secret BYTEA,
    -- Its last 4 characters, the most the API ever shows.
    secret_last4 TEXT,
    secret_updated_at TIMESTAMPTZ,
    -- The last fetch run: {"status": "ok" | "refused_key" | "failed",
    -- "requests", "stored", "new_data", "failures", "more"}.
    last_run JSONB,
    last_run_at TIMESTAMPTZ
);
