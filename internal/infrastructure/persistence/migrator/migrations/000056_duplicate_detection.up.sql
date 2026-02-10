-- 1. Scene fingerprint hashes (per-frame dHash storage)
CREATE TABLE scene_fingerprints (
    id BIGSERIAL PRIMARY KEY,
    scene_id INTEGER NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    frame_index INTEGER NOT NULL,
    hash_value BIGINT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (scene_id, frame_index)
);
CREATE INDEX idx_scene_fingerprints_scene_id ON scene_fingerprints(scene_id);
CREATE INDEX idx_scene_fingerprints_hash_value ON scene_fingerprints(hash_value);

-- 2. Duplicate groups
CREATE TABLE duplicate_groups (
    id SERIAL PRIMARY KEY,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    winner_scene_id INTEGER REFERENCES scenes(id) ON DELETE SET NULL,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_duplicate_groups_status ON duplicate_groups(status);

-- 3. Duplicate group members (junction)
CREATE TABLE duplicate_group_members (
    id SERIAL PRIMARY KEY,
    group_id INTEGER NOT NULL REFERENCES duplicate_groups(id) ON DELETE CASCADE,
    scene_id INTEGER NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    match_percentage DECIMAL(5,2) NOT NULL DEFAULT 0,
    frame_offset INTEGER NOT NULL DEFAULT 0,
    is_winner BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (group_id, scene_id)
);
CREATE INDEX idx_dgm_scene_id ON duplicate_group_members(scene_id);
CREATE INDEX idx_dgm_group_id ON duplicate_group_members(group_id);

-- 4. Duplicate config (singleton)
CREATE TABLE duplicate_config (
    id INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    check_on_upload BOOLEAN NOT NULL DEFAULT TRUE,
    match_threshold INTEGER NOT NULL DEFAULT 80,
    hamming_distance INTEGER NOT NULL DEFAULT 8,
    duplicate_action VARCHAR(20) NOT NULL DEFAULT 'flag',
    keep_best_rules JSONB NOT NULL DEFAULT '["duration","resolution","codec","bitrate"]',
    keep_best_enabled JSONB NOT NULL DEFAULT '{"duration":true,"resolution":true,"codec":true,"bitrate":true}',
    codec_preference JSONB NOT NULL DEFAULT '["h265","hevc","av1","vp9","h264"]',
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
INSERT INTO duplicate_config (id) VALUES (1);

-- 5. Add fingerprint fields to scenes
ALTER TABLE scenes ADD COLUMN fingerprint_status VARCHAR(20) NOT NULL DEFAULT 'pending';
ALTER TABLE scenes ADD COLUMN fingerprint_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE scenes ADD COLUMN duplicate_group_id INTEGER REFERENCES duplicate_groups(id) ON DELETE SET NULL;
CREATE INDEX idx_scenes_fingerprint_status ON scenes(fingerprint_status);
CREATE INDEX idx_scenes_duplicate_group_id ON scenes(duplicate_group_id) WHERE duplicate_group_id IS NOT NULL;

-- 6. Add fingerprint workers to pool_config
ALTER TABLE pool_config ADD COLUMN fingerprint_workers INTEGER NOT NULL DEFAULT 1;

-- 7. Update CHECK constraints for fingerprint phase
ALTER TABLE trigger_config DROP CONSTRAINT IF EXISTS valid_phase;
ALTER TABLE trigger_config ADD CONSTRAINT valid_phase
  CHECK (phase IN ('metadata', 'thumbnail', 'sprites', 'animated_thumbnails', 'fingerprint', 'scan'));

ALTER TABLE retry_config DROP CONSTRAINT IF EXISTS valid_retry_phase;
ALTER TABLE retry_config ADD CONSTRAINT valid_retry_phase
  CHECK (phase IN ('metadata', 'thumbnail', 'sprites', 'animated_thumbnails', 'fingerprint', 'scan'));

-- 8. Default trigger: fingerprint runs after metadata
INSERT INTO trigger_config (phase, trigger_type, after_phase)
  VALUES ('fingerprint', 'after_job', 'metadata') ON CONFLICT DO NOTHING;

INSERT INTO retry_config (phase, max_retries, initial_delay_seconds, max_delay_seconds, backoff_factor)
  VALUES ('fingerprint', 3, 30, 300, 2.0) ON CONFLICT DO NOTHING;
