-- Device sessions: long-lived Bearer tokens for apps (Android, Android
-- Auto), listed and revoked per device. Web sessions stay cookie-based.
ALTER TABLE sessions
    ADD COLUMN kind TEXT NOT NULL DEFAULT 'web' CHECK (kind IN ('web', 'device')),
    ADD COLUMN device_name TEXT,
    ADD COLUMN last_used_at TIMESTAMPTZ;
CREATE INDEX idx_sessions_user_kind ON sessions (user_id, kind);
