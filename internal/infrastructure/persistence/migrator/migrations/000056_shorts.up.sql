-- Shorts: app-wide feed length limit and clip save folder, plus the link from
-- a clip to the scene it was cut from. Idempotent so it can be re-applied.
ALTER TABLE app_settings ADD COLUMN IF NOT EXISTS shorts_max_duration INTEGER NOT NULL DEFAULT 60;
ALTER TABLE app_settings ADD COLUMN IF NOT EXISTS shorts_save_dir TEXT NULL;

ALTER TABLE scenes ADD COLUMN IF NOT EXISTS source_scene_id BIGINT NULL;
ALTER TABLE scenes ADD COLUMN IF NOT EXISTS source_start DOUBLE PRECISION NULL;
ALTER TABLE scenes ADD COLUMN IF NOT EXISTS source_end DOUBLE PRECISION NULL;

ALTER TABLE scenes DROP CONSTRAINT IF EXISTS fk_scenes_source_scene;
ALTER TABLE scenes ADD CONSTRAINT fk_scenes_source_scene
    FOREIGN KEY (source_scene_id) REFERENCES scenes(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_scenes_source_scene_id ON scenes(source_scene_id);
