CREATE TABLE duplicate_groups (
    id BIGSERIAL PRIMARY KEY,
    status VARCHAR(20) NOT NULL DEFAULT 'unresolved',
    scene_count INTEGER NOT NULL DEFAULT 0,
    best_scene_id BIGINT REFERENCES scenes(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX idx_duplicate_groups_status ON duplicate_groups(status);

CREATE TABLE duplicate_group_members (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES duplicate_groups(id) ON DELETE CASCADE,
    scene_id BIGINT NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    is_best BOOLEAN NOT NULL DEFAULT FALSE,
    confidence_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    match_type VARCHAR(10) NOT NULL DEFAULT 'audio',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(group_id, scene_id)
);

CREATE INDEX idx_duplicate_group_members_scene_id ON duplicate_group_members(scene_id);
CREATE INDEX idx_duplicate_group_members_group_id ON duplicate_group_members(group_id);
