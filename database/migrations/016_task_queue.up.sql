-- Background work for the analytics worker (replaces the Redis/Celery broker).
-- Workers claim pending rows with FOR UPDATE SKIP LOCKED; a finished row is
-- deleted, a row that ran out of attempts stays as 'failed' for inspection.
CREATE TABLE IF NOT EXISTS task_queue (
    id BIGSERIAL PRIMARY KEY,
    task TEXT NOT NULL,
    args JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    run_after TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_task_queue_pending ON task_queue (id) WHERE status = 'pending';
