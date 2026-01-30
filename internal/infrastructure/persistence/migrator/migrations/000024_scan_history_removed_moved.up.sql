-- Add videos_removed and videos_moved columns to scan_history
ALTER TABLE scan_history ADD COLUMN IF NOT EXISTS videos_removed INT NOT NULL DEFAULT 0;
ALTER TABLE scan_history ADD COLUMN IF NOT EXISTS videos_moved INT NOT NULL DEFAULT 0;
