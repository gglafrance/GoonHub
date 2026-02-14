-- Remove fingerprint trigger config
DELETE FROM trigger_config WHERE phase = 'fingerprint';

-- Remove fingerprint retry config
DELETE FROM retry_config WHERE phase = 'fingerprint';

-- Revert CHECK constraints
ALTER TABLE trigger_config DROP CONSTRAINT IF EXISTS valid_phase;
ALTER TABLE trigger_config ADD CONSTRAINT valid_phase
  CHECK (phase IN ('metadata', 'thumbnail', 'sprites', 'animated_thumbnails', 'scan'));

ALTER TABLE trigger_config DROP CONSTRAINT IF EXISTS valid_after_phase;
ALTER TABLE trigger_config ADD CONSTRAINT valid_after_phase
  CHECK (after_phase IS NULL OR after_phase IN ('metadata', 'thumbnail', 'sprites', 'animated_thumbnails'));

ALTER TABLE retry_config DROP CONSTRAINT IF EXISTS valid_retry_phase;
ALTER TABLE retry_config ADD CONSTRAINT valid_retry_phase
  CHECK (phase IN ('metadata', 'thumbnail', 'sprites', 'animated_thumbnails', 'scan'));
