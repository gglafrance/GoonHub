-- Revert audio_min_hashes default
UPDATE duplication_config SET audio_min_hashes = 40 WHERE audio_min_hashes = 80;

ALTER TABLE duplication_config
    DROP COLUMN IF EXISTS audio_max_hash_occurrences,
    DROP COLUMN IF EXISTS audio_min_span,
    DROP COLUMN IF EXISTS visual_min_span;
