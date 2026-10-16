DROP INDEX IF EXISTS idx_scan_jobs_masters_created;
DROP INDEX IF EXISTS idx_scan_jobs_parent_id;
ALTER TABLE scan_jobs DROP COLUMN IF EXISTS parent_id;
