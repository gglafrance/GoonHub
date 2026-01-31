CREATE TABLE IF NOT EXISTS user_video_markers (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    timestamp INTEGER NOT NULL CHECK (timestamp >= 0),
    label VARCHAR(100),
    color VARCHAR(7) NOT NULL DEFAULT '#FFFFFF',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_video_markers_user_video ON user_video_markers(user_id, video_id);
CREATE INDEX idx_user_video_markers_timestamp ON user_video_markers(video_id, timestamp);
