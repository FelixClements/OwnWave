-- Scan job retention (ADR 0006, #120): the cleanup Python now does at the
-- end of every scan, applied once to the rows left by earlier scans.
--
-- 1. Each kept finished scan (master job) gets its folder jobs' summed
--    progress written into its own scan_job_progress row, with the folder
--    counts, so the admin view shows the same totals without the folder rows.
-- 2. Kept: the newest 100 scans, every scan still pending or running, and
--    the folder jobs of the scans still pending or running. Everything else
--    goes: the folder jobs of finished scans and finished scans beyond the
--    newest 100.
--
-- Before #118 every scan left about 5,900 folder jobs, so this can be
-- millions of rows. Deleting them row by row runs three foreign-key checks
-- per row (about 30 s per million rows), and Go runs this before it serves.
-- Instead the kept rows (a few hundred) are copied aside, both tables are
-- truncated and the kept rows put back: a few seconds whatever the size.
--
-- The tables are locked first, so a scan running in an old worker during the
-- deploy waits for the few seconds this takes and loses nothing. This is its
-- own migration so the ALTER in 030 is not part of it.

LOCK TABLE scan_jobs, scan_job_progress IN ACCESS EXCLUSIVE MODE;

CREATE TEMP TABLE kept_scans ON COMMIT DROP AS
SELECT id, status IN ('completed', 'failed') AS finished
FROM scan_jobs
WHERE parent_id IS NULL
  AND (
      status NOT IN ('completed', 'failed')
      OR id IN (SELECT id FROM scan_jobs WHERE parent_id IS NULL ORDER BY created_at DESC LIMIT 100)
  );

WITH kids AS (
    SELECT c.parent_id AS master_id,
           COUNT(*) AS total,
           COUNT(*) FILTER (WHERE c.status = 'completed') AS done,
           COUNT(*) FILTER (WHERE c.status = 'failed') AS failed
    FROM scan_jobs c
    JOIN kept_scans k ON k.id = c.parent_id AND k.finished
    GROUP BY c.parent_id
),
sums AS (
    SELECT p.master_job_id AS master_id,
           jsonb_build_object(
               'total_files', COALESCE(SUM((p.stats->>'total_files')::numeric), 0)::bigint,
               'imported', COALESCE(SUM((p.stats->>'imported')::numeric), 0)::bigint,
               'scanned', COALESCE(SUM((p.stats->>'scanned')::numeric), 0)::bigint,
               'features', COALESCE(SUM((p.stats->>'features')::numeric), 0)::bigint,
               'model_success', COALESCE(SUM((p.stats->>'model_success')::numeric), 0)::bigint,
               'model_failed', COALESCE(SUM((p.stats->>'model_failed')::numeric), 0)::bigint
           ) AS counts
    FROM scan_job_progress p
    JOIN kids k ON k.master_id = p.master_job_id
    WHERE p.job_id <> p.master_job_id
    GROUP BY p.master_job_id
),
fails AS (
    SELECT p.master_job_id AS master_id, jsonb_agg(f.path) AS failed_paths
    FROM scan_job_progress p
    JOIN kids k ON k.master_id = p.master_job_id
    CROSS JOIN LATERAL jsonb_array_elements(
        CASE WHEN jsonb_typeof(p.stats->'failed_paths') = 'array' THEN p.stats->'failed_paths' ELSE '[]'::jsonb END
    ) AS f(path)
    WHERE p.job_id <> p.master_job_id
    GROUP BY p.master_job_id
)
INSERT INTO scan_job_progress (job_id, master_job_id, stats, updated_at)
SELECT k.master_id,
       NULL,
       COALESCE(s.counts, jsonb_build_object(
           'total_files', 0, 'imported', 0, 'scanned', 0, 'features', 0, 'model_success', 0, 'model_failed', 0
       ))
       || jsonb_build_object(
           'failed_paths', COALESCE(f.failed_paths, '[]'::jsonb),
           'folders', jsonb_build_object('total', k.total, 'done', k.done, 'failed', k.failed)
       ),
       NOW()
FROM kids k
LEFT JOIN sums s ON s.master_id = k.master_id
LEFT JOIN fails f ON f.master_id = k.master_id
ON CONFLICT (job_id) DO UPDATE
SET master_job_id = NULL, stats = EXCLUDED.stats, updated_at = NOW();

CREATE TEMP TABLE kept_jobs ON COMMIT DROP AS
SELECT j.* FROM scan_jobs j JOIN kept_scans k ON k.id = j.id
UNION ALL
SELECT c.* FROM scan_jobs c JOIN kept_scans k ON k.id = c.parent_id AND NOT k.finished;

CREATE TEMP TABLE kept_progress ON COMMIT DROP AS
SELECT p.* FROM scan_job_progress p
WHERE p.job_id IN (SELECT id FROM kept_jobs)
  AND (p.master_job_id IS NULL OR p.master_job_id IN (SELECT id FROM kept_jobs));

TRUNCATE scan_job_progress, scan_jobs;

-- Foreign keys are checked at the end of each statement, so the folder jobs
-- may come before their scan.
INSERT INTO scan_jobs SELECT * FROM kept_jobs;
INSERT INTO scan_job_progress SELECT * FROM kept_progress;

DROP TABLE kept_progress, kept_jobs, kept_scans;
