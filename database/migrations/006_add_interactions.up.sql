CREATE TABLE IF NOT EXISTS listening_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    track_id UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    station_id UUID REFERENCES stations(id) ON DELETE SET NULL,
    played_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_listening_history_track_id ON listening_history(track_id);
CREATE INDEX IF NOT EXISTS idx_listening_history_played_at ON listening_history(played_at DESC);

CREATE TABLE IF NOT EXISTS track_feedback (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    track_id UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    feedback VARCHAR(20) NOT NULL CHECK (feedback IN ('like', 'skip', 'ban')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(track_id, feedback)
);

CREATE INDEX IF NOT EXISTS idx_track_feedback_track_id ON track_feedback(track_id);
CREATE INDEX IF NOT EXISTS idx_track_feedback_feedback ON track_feedback(feedback);
