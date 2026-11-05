ALTER TABLE duplication_config ADD COLUMN IF NOT EXISTS fingerprint_mode VARCHAR(20) NOT NULL DEFAULT 'audio_only';
