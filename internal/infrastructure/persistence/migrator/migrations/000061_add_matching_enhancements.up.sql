ALTER TABLE duplication_config
    ADD COLUMN IF NOT EXISTS audio_max_hash_occurrences INTEGER NOT NULL DEFAULT 10,
    ADD COLUMN IF NOT EXISTS audio_min_span INTEGER NOT NULL DEFAULT 160,
    ADD COLUMN IF NOT EXISTS visual_min_span INTEGER NOT NULL DEFAULT 30;

-- Update audio_min_hashes default from 40 to 80
UPDATE duplication_config SET audio_min_hashes = 80 WHERE audio_min_hashes = 40;
