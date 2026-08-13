CREATE TABLE IF NOT EXISTS app_state (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO app_state (key, value)
VALUES ('setup_completed', '{"completed": false}'::jsonb)
ON CONFLICT (key) DO NOTHING;
