-- trigger_config: update CHECK constraints to include fingerprint
ALTER TABLE trigger_config DROP CONSTRAINT IF EXISTS valid_phase;
ALTER TABLE trigger_config ADD CONSTRAINT valid_phase
  CHECK (phase IN ('metadata', 'thumbnail', 'sprites', 'animated_thumbnails', 'fingerprint', 'scan'));

ALTER TABLE trigger_config DROP CONSTRAINT IF EXISTS valid_after_phase;
ALTER TABLE trigger_config ADD CONSTRAINT valid_after_phase
  CHECK (after_phase IS NULL OR after_phase IN ('metadata', 'thumbnail', 'sprites', 'animated_thumbnails', 'fingerprint'));

-- retry_config: update CHECK constraint to include fingerprint
ALTER TABLE retry_config DROP CONSTRAINT IF EXISTS valid_retry_phase;
ALTER TABLE retry_config ADD CONSTRAINT valid_retry_phase
  CHECK (phase IN ('metadata', 'thumbnail', 'sprites', 'animated_thumbnails', 'fingerprint', 'scan'));

-- Default trigger config for fingerprint (after metadata by default)
INSERT INTO trigger_config (phase, trigger_type, after_phase) VALUES ('fingerprint', 'after_job', 'metadata')
  ON CONFLICT DO NOTHING;

-- Default retry config for fingerprint
INSERT INTO retry_config (phase, max_retries, initial_delay_seconds, max_delay_seconds, backoff_factor)
  VALUES ('fingerprint', 3, 30, 300, 2.0)
  ON CONFLICT DO NOTHING;
