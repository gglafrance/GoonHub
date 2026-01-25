CREATE TABLE storage_paths (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    path VARCHAR(500) NOT NULL UNIQUE,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insert default storage path (new structure)
INSERT INTO storage_paths (name, path, is_default) VALUES ('Default', './data/videos', TRUE);

-- Ensure only one default path at a time
CREATE UNIQUE INDEX idx_storage_paths_single_default ON storage_paths (is_default) WHERE is_default = TRUE;
