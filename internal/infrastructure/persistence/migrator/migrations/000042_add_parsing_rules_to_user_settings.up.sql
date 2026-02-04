ALTER TABLE user_settings ADD COLUMN parsing_rules JSONB DEFAULT '{"presets":[],"activePresetId":null}'::jsonb;
