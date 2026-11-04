ALTER TABLE playlists
    DROP COLUMN IF EXISTS cover_url,
    DROP COLUMN IF EXISTS cover_image,
    DROP COLUMN IF EXISTS cover_type;
