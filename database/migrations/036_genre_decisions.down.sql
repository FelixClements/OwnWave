DROP VIEW IF EXISTS genre_review_queue;

DELETE FROM genre_corrections WHERE release_id IS NOT NULL;
DROP INDEX IF EXISTS idx_genre_corrections_release;
ALTER TABLE genre_corrections DROP CONSTRAINT genre_corrections_one_target;
ALTER TABLE genre_corrections DROP COLUMN release_id;
ALTER TABLE genre_corrections ADD CONSTRAINT genre_corrections_one_target
    CHECK ((track_id IS NULL) <> (artist IS NULL));

DROP TABLE IF EXISTS track_genre_decisions;
