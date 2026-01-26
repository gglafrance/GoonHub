-- Remove scan trigger record
DELETE FROM trigger_config WHERE phase = 'scan';

-- Restore original valid_phase constraint
ALTER TABLE trigger_config DROP CONSTRAINT valid_phase;
ALTER TABLE trigger_config ADD CONSTRAINT valid_phase CHECK (phase IN ('metadata', 'thumbnail', 'sprites'));
