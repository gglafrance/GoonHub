-- Matching segments shown on the duplicates page (seconds) and delayed file deletion of resolved duplicates
ALTER TABLE duplicate_group_members
    ADD COLUMN match_start REAL NULL,
    ADD COLUMN match_end REAL NULL,
    ADD COLUMN ref_scene_id BIGINT NULL REFERENCES scenes(id) ON DELETE SET NULL,
    ADD COLUMN ref_start REAL NULL,
    ADD COLUMN ref_end REAL NULL,
    ADD COLUMN delete_after TIMESTAMPTZ NULL;

CREATE INDEX idx_duplicate_group_members_delete_after
    ON duplicate_group_members (delete_after)
    WHERE delete_after IS NOT NULL;

ALTER TABLE duplication_config
    ADD COLUMN delete_resolved_files BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN deletion_delay_hours INTEGER NOT NULL DEFAULT 72;
