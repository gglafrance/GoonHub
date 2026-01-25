-- Add storage_path_id to videos table
ALTER TABLE videos ADD COLUMN IF NOT EXISTS storage_path_id INTEGER REFERENCES storage_paths(id);

-- Create scan_history table
CREATE TABLE IF NOT EXISTS scan_history (
    id SERIAL PRIMARY KEY,
    status VARCHAR(20) NOT NULL DEFAULT 'running',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    paths_scanned INT NOT NULL DEFAULT 0,
    files_found INT NOT NULL DEFAULT 0,
    videos_added INT NOT NULL DEFAULT 0,
    videos_skipped INT NOT NULL DEFAULT 0,
    errors INT NOT NULL DEFAULT 0,
    error_message TEXT,
    current_path VARCHAR(500),
    current_file VARCHAR(500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create index for faster status lookups
CREATE INDEX IF NOT EXISTS idx_scan_history_status ON scan_history(status);
CREATE INDEX IF NOT EXISTS idx_scan_history_started_at ON scan_history(started_at DESC);
