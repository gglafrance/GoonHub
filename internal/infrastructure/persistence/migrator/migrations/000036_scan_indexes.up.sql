-- Partial index on stored_path for fast existence checks during scans (excludes soft-deleted)
CREATE INDEX IF NOT EXISTS idx_scenes_stored_path ON scenes(stored_path) WHERE deleted_at IS NULL;

-- Composite index on (size, original_filename) for move detection during scans
CREATE INDEX IF NOT EXISTS idx_scenes_size_filename ON scenes(size, original_filename);
