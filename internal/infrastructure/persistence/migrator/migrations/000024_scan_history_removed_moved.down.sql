-- Remove videos_removed and videos_moved columns from scan_history
ALTER TABLE scan_history DROP COLUMN IF EXISTS videos_removed;
ALTER TABLE scan_history DROP COLUMN IF EXISTS videos_moved;
