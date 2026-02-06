-- user_scene_markers: add animated thumbnail path
ALTER TABLE user_scene_markers ADD COLUMN animated_thumbnail_path VARCHAR(255) DEFAULT '';

-- pool_config: add animated thumbnails workers
ALTER TABLE pool_config ADD COLUMN animated_thumbnails_workers INTEGER NOT NULL DEFAULT 1;

-- processing_config: add marker type + duration
ALTER TABLE processing_config
  ADD COLUMN marker_thumbnail_type VARCHAR(10) NOT NULL DEFAULT 'static',
  ADD COLUMN marker_animated_duration INTEGER NOT NULL DEFAULT 10;

-- trigger_config: update CHECK constraint to include animated_thumbnails
ALTER TABLE trigger_config DROP CONSTRAINT IF EXISTS valid_phase;
ALTER TABLE trigger_config ADD CONSTRAINT valid_phase
  CHECK (phase IN ('metadata', 'thumbnail', 'sprites', 'animated_thumbnails', 'scan'));

-- retry_config: update CHECK constraint
ALTER TABLE retry_config DROP CONSTRAINT IF EXISTS valid_retry_phase;
ALTER TABLE retry_config ADD CONSTRAINT valid_retry_phase
  CHECK (phase IN ('metadata', 'thumbnail', 'sprites', 'animated_thumbnails', 'scan'));

-- Default trigger config for animated_thumbnails (manual by default)
INSERT INTO trigger_config (phase, trigger_type) VALUES ('animated_thumbnails', 'manual')
  ON CONFLICT DO NOTHING;

-- Default retry config for animated_thumbnails
INSERT INTO retry_config (phase, max_retries, initial_delay_seconds, max_delay_seconds, backoff_factor)
  VALUES ('animated_thumbnails', 3, 30, 300, 2.0)
  ON CONFLICT DO NOTHING;
