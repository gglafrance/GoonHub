ALTER TABLE user_settings DROP COLUMN IF EXISTS marker_loop;

ALTER TABLE user_scene_markers
    DROP CONSTRAINT IF EXISTS chk_user_scene_markers_end_after_start,
    DROP COLUMN IF EXISTS end_timestamp;
