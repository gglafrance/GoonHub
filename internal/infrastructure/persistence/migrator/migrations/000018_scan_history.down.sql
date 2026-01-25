-- Drop indexes
DROP INDEX IF EXISTS idx_scan_history_started_at;
DROP INDEX IF EXISTS idx_scan_history_status;

-- Drop scan_history table
DROP TABLE IF EXISTS scan_history;

-- Remove storage_path_id from videos table
ALTER TABLE videos DROP COLUMN IF EXISTS storage_path_id;
