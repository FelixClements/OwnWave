DELETE FROM sessions WHERE kind = 'device';
DROP INDEX IF EXISTS idx_sessions_user_kind;
ALTER TABLE sessions
    DROP COLUMN IF EXISTS kind,
    DROP COLUMN IF EXISTS device_name,
    DROP COLUMN IF EXISTS last_used_at;
