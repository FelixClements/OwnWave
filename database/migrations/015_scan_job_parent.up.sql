-- A library scan is a master job with one child job per music folder. The
-- link lets the scan report folder progress and lets history list only
-- master jobs; deleting a master job removes its children.
ALTER TABLE scan_jobs
    ADD COLUMN IF NOT EXISTS parent_id UUID REFERENCES scan_jobs(id) ON DELETE CASCADE;

-- Existing child jobs are only linked through their progress rows.
UPDATE scan_jobs j
SET parent_id = p.master_job_id
FROM scan_job_progress p
WHERE p.job_id = j.id
  AND p.master_job_id IS NOT NULL
  AND p.master_job_id <> j.id
  AND j.parent_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_scan_jobs_parent_id ON scan_jobs (parent_id);
CREATE INDEX IF NOT EXISTS idx_scan_jobs_masters_created
    ON scan_jobs (created_at DESC) WHERE parent_id IS NULL;
