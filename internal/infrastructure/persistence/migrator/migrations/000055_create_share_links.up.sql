CREATE TABLE share_links (
    id          BIGSERIAL PRIMARY KEY,
    token       VARCHAR(32) NOT NULL,
    scene_id    BIGINT NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    share_type  VARCHAR(20) NOT NULL DEFAULT 'public',
    expires_at  TIMESTAMPTZ,
    view_count  BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_share_type CHECK (share_type IN ('public', 'auth_required'))
);
CREATE UNIQUE INDEX idx_share_links_token ON share_links(token);
CREATE INDEX idx_share_links_scene_user ON share_links(scene_id, user_id);
CREATE INDEX idx_share_links_user_id ON share_links(user_id);
