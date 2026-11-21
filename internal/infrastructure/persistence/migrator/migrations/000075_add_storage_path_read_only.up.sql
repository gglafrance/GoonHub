-- Read-only storage paths: GoonHub never deletes, moves, renames or writes
-- media files under such a path (scene rows can still be deleted).
ALTER TABLE storage_paths
    ADD COLUMN read_only BOOLEAN NOT NULL DEFAULT false;
