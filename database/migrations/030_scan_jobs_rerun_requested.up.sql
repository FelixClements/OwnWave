-- A scan request that arrives while a scan of the same path is running sets
-- this flag instead of being dropped. The scan queues one follow-up scan when
-- it ends (ADR 0006, #120). A constant default is a catalog-only change, so
-- the ALTER holds its table lock only briefly.
ALTER TABLE scan_jobs
    ADD COLUMN IF NOT EXISTS rerun_requested BOOLEAN NOT NULL DEFAULT false;
