-- Add scan to valid phases in trigger_config
ALTER TABLE trigger_config DROP CONSTRAINT valid_phase;
ALTER TABLE trigger_config ADD CONSTRAINT valid_phase CHECK (phase IN ('metadata', 'thumbnail', 'sprites', 'scan'));

-- Insert default scan trigger (manual by default)
INSERT INTO trigger_config (phase, trigger_type, after_phase, cron_expression) VALUES
    ('scan', 'manual', NULL, NULL);
