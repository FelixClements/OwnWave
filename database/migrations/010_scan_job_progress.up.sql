CREATE TABLE IF NOT EXISTS scan_job_progress (
    job_id UUID PRIMARY KEY REFERENCES scan_jobs(id) ON DELETE CASCADE,
    master_job_id UUID REFERENCES scan_jobs(id) ON DELETE CASCADE,
    stats JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_scan_job_progress_master_job_id
    ON scan_job_progress(master_job_id);
