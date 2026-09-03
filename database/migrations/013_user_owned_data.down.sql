DROP INDEX IF EXISTS idx_track_feedback_user_track;
DROP INDEX IF EXISTS idx_stations_user;
DROP INDEX IF EXISTS idx_listening_history_user;
DROP INDEX IF EXISTS idx_track_feedback_user;
DROP INDEX IF EXISTS idx_stations_user_auto_name;

CREATE UNIQUE INDEX IF NOT EXISTS idx_stations_auto_name
    ON stations(name) WHERE is_auto = TRUE;

ALTER TABLE stations DROP COLUMN IF EXISTS user_id;
ALTER TABLE listening_history DROP COLUMN IF EXISTS user_id;
ALTER TABLE track_feedback DROP COLUMN IF EXISTS user_id;

CREATE UNIQUE INDEX IF NOT EXISTS track_feedback_track_id_feedback_key
    ON track_feedback(track_id, feedback);
