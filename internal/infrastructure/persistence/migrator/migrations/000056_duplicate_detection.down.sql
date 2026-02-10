-- Remove fingerprint trigger and retry config
DELETE FROM trigger_config WHERE phase = 'fingerprint';
DELETE FROM retry_config WHERE phase = 'fingerprint';

-- Revert CHECK constraints
ALTER TABLE trigger_config DROP CONSTRAINT IF EXISTS valid_phase;
ALTER TABLE trigger_config ADD CONSTRAINT valid_phase
  CHECK (phase IN ('metadata', 'thumbnail', 'sprites', 'animated_thumbnails', 'scan'));

ALTER TABLE retry_config DROP CONSTRAINT IF EXISTS valid_retry_phase;
ALTER TABLE retry_config ADD CONSTRAINT valid_retry_phase
  CHECK (phase IN ('metadata', 'thumbnail', 'sprites', 'animated_thumbnails', 'scan'));

-- Remove fingerprint workers from pool_config
ALTER TABLE pool_config DROP COLUMN IF EXISTS fingerprint_workers;

-- Remove fingerprint fields from scenes
DROP INDEX IF EXISTS idx_scenes_duplicate_group_id;
DROP INDEX IF EXISTS idx_scenes_fingerprint_status;
ALTER TABLE scenes DROP COLUMN IF EXISTS duplicate_group_id;
ALTER TABLE scenes DROP COLUMN IF EXISTS fingerprint_count;
ALTER TABLE scenes DROP COLUMN IF EXISTS fingerprint_status;

-- Drop tables in reverse order
DROP TABLE IF EXISTS duplicate_config;
DROP TABLE IF EXISTS duplicate_group_members;
DROP TABLE IF EXISTS duplicate_groups;
DROP TABLE IF EXISTS scene_fingerprints;
