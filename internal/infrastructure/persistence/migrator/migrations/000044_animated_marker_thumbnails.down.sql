-- Remove default configs for animated_thumbnails
DELETE FROM trigger_config WHERE phase = 'animated_thumbnails';
DELETE FROM retry_config WHERE phase = 'animated_thumbnails';

-- Restore CHECK constraints without animated_thumbnails
ALTER TABLE trigger_config DROP CONSTRAINT IF EXISTS valid_phase;
ALTER TABLE trigger_config ADD CONSTRAINT valid_phase
  CHECK (phase IN ('metadata', 'thumbnail', 'sprites', 'scan'));

ALTER TABLE retry_config DROP CONSTRAINT IF EXISTS valid_retry_phase;
ALTER TABLE retry_config ADD CONSTRAINT valid_retry_phase
  CHECK (phase IN ('metadata', 'thumbnail', 'sprites', 'scan'));

-- Remove processing_config columns
ALTER TABLE processing_config DROP COLUMN IF EXISTS marker_thumbnail_type;
ALTER TABLE processing_config DROP COLUMN IF EXISTS marker_animated_duration;

-- Remove pool_config column
ALTER TABLE pool_config DROP COLUMN IF EXISTS animated_thumbnails_workers;

-- Remove animated_thumbnail_path from user_scene_markers
ALTER TABLE user_scene_markers DROP COLUMN IF EXISTS animated_thumbnail_path;
