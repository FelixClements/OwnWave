-- Library picks look up each playlist track's nearest songs (ADR 0002).
-- Without an index every lookup reads all embeddings (5 KB each, stored out
-- of line): about 0.15 s each, so rebuilding every playlist's picks kept a
-- core busy for about 14 minutes on a library of 7k songs. With HNSW a
-- lookup takes about 1 ms.
--
-- Parallel index builds need more shared memory than Docker's default
-- 64 MB /dev/shm, so build in this process only.
SET max_parallel_maintenance_workers = 0;
SET maintenance_work_mem = '256MB';
CREATE INDEX IF NOT EXISTS idx_track_audio_genres_embedding
    ON track_audio_genres USING hnsw (embedding vector_cosine_ops);
