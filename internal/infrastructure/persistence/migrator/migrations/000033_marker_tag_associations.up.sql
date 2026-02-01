-- Label-level default tags (per user)
CREATE TABLE IF NOT EXISTS marker_label_tags (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label VARCHAR(100) NOT NULL,
    tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, label, tag_id)
);

CREATE INDEX idx_marker_label_tags_user_label ON marker_label_tags(user_id, label);
CREATE INDEX idx_marker_label_tags_tag_id ON marker_label_tags(tag_id);

-- Individual marker tags
CREATE TABLE IF NOT EXISTS marker_tags (
    id SERIAL PRIMARY KEY,
    marker_id BIGINT NOT NULL REFERENCES user_video_markers(id) ON DELETE CASCADE,
    tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    is_from_label BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(marker_id, tag_id)
);

CREATE INDEX idx_marker_tags_marker_id ON marker_tags(marker_id);
CREATE INDEX idx_marker_tags_tag_id ON marker_tags(tag_id);
