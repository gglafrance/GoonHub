-- Remove studio_id from videos
ALTER TABLE videos DROP COLUMN IF EXISTS studio_id;

-- Drop indexes
DROP INDEX IF EXISTS idx_videos_studio_id;
DROP INDEX IF EXISTS idx_studios_porndb_id;
DROP INDEX IF EXISTS idx_studios_deleted_at;
DROP INDEX IF EXISTS idx_studios_name;
DROP INDEX IF EXISTS idx_studios_uuid;

-- Drop studios table
DROP TABLE IF EXISTS studios;
