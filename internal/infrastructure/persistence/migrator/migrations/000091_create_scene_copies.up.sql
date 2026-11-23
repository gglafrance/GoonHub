-- Files that are byte-identical copies of a scene's file at another path (same size and same
-- oshash). The library scan records such a file here instead of creating a new scene, so a
-- rescan never re-adds the extra copies of a video (e.g. the other files of a Stash scene).
-- Rows are bookkeeping only: deleting a scene removes its rows (CASCADE), never the files.
CREATE TABLE scene_copies (
    id BIGSERIAL PRIMARY KEY,
    scene_id BIGINT NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    stored_path TEXT NOT NULL,
    storage_path_id INTEGER REFERENCES storage_paths(id) ON DELETE SET NULL,
    size BIGINT NOT NULL,
    oshash VARCHAR(16) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_scene_copies_stored_path UNIQUE (stored_path)
);

CREATE INDEX idx_scene_copies_scene_id ON scene_copies(scene_id);

-- Number of files a scan recognised as copies of an existing scene (new or already recorded)
ALTER TABLE scan_history ADD COLUMN IF NOT EXISTS copies_found INT NOT NULL DEFAULT 0;
