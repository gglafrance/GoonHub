DROP INDEX IF EXISTS idx_scenes_source_scene_id;
ALTER TABLE scenes DROP CONSTRAINT IF EXISTS fk_scenes_source_scene;
ALTER TABLE scenes DROP COLUMN IF EXISTS source_end;
ALTER TABLE scenes DROP COLUMN IF EXISTS source_start;
ALTER TABLE scenes DROP COLUMN IF EXISTS source_scene_id;
ALTER TABLE app_settings DROP COLUMN IF EXISTS shorts_save_dir;
ALTER TABLE app_settings DROP COLUMN IF EXISTS shorts_max_duration;
