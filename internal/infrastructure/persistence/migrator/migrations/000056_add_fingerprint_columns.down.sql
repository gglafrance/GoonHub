DROP INDEX IF EXISTS idx_scenes_fingerprint_type;
ALTER TABLE scenes DROP COLUMN IF EXISTS fingerprint_at;
ALTER TABLE scenes DROP COLUMN IF EXISTS fingerprint_type;
ALTER TABLE scenes DROP COLUMN IF EXISTS visual_fingerprint;
ALTER TABLE scenes DROP COLUMN IF EXISTS audio_fingerprint;
