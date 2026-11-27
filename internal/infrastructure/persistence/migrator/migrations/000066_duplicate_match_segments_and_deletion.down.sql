ALTER TABLE duplication_config
    DROP COLUMN IF EXISTS deletion_delay_hours,
    DROP COLUMN IF EXISTS delete_resolved_files;

DROP INDEX IF EXISTS idx_duplicate_group_members_delete_after;

ALTER TABLE duplicate_group_members
    DROP COLUMN IF EXISTS delete_after,
    DROP COLUMN IF EXISTS ref_end,
    DROP COLUMN IF EXISTS ref_start,
    DROP COLUMN IF EXISTS ref_scene_id,
    DROP COLUMN IF EXISTS match_end,
    DROP COLUMN IF EXISTS match_start;
