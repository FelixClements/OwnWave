ALTER TABLE stations ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE listening_history ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE track_feedback ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE CASCADE;

UPDATE stations
SET user_id = (SELECT id FROM users WHERE is_admin ORDER BY created_at ASC LIMIT 1)
WHERE user_id IS NULL;

UPDATE listening_history
SET user_id = (SELECT id FROM users WHERE is_admin ORDER BY created_at ASC LIMIT 1)
WHERE user_id IS NULL;

UPDATE track_feedback
SET user_id = (SELECT id FROM users WHERE is_admin ORDER BY created_at ASC LIMIT 1)
WHERE user_id IS NULL;

DELETE FROM stations WHERE user_id IS NULL;
DELETE FROM listening_history WHERE user_id IS NULL;
DELETE FROM track_feedback WHERE user_id IS NULL;

ALTER TABLE stations ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE listening_history ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE track_feedback ALTER COLUMN user_id SET NOT NULL;

DROP INDEX IF EXISTS idx_stations_auto_name;
CREATE UNIQUE INDEX IF NOT EXISTS idx_stations_user_auto_name
    ON stations(user_id, name) WHERE is_auto = TRUE;

ALTER TABLE track_feedback DROP CONSTRAINT IF EXISTS track_feedback_track_id_feedback_key;
CREATE UNIQUE INDEX IF NOT EXISTS idx_track_feedback_user_track
    ON track_feedback(user_id, track_id, feedback);

CREATE INDEX IF NOT EXISTS idx_stations_user ON stations(user_id);
CREATE INDEX IF NOT EXISTS idx_listening_history_user ON listening_history(user_id, played_at DESC);
CREATE INDEX IF NOT EXISTS idx_track_feedback_user ON track_feedback(user_id);
