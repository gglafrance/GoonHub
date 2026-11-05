ALTER TABLE scenes ADD COLUMN audio_fingerprint BYTEA;
ALTER TABLE scenes ADD COLUMN visual_fingerprint BYTEA;
ALTER TABLE scenes ADD COLUMN fingerprint_type VARCHAR(10);
ALTER TABLE scenes ADD COLUMN fingerprint_at TIMESTAMPTZ;

CREATE INDEX idx_scenes_fingerprint_type ON scenes(fingerprint_type) WHERE fingerprint_type IS NOT NULL;
