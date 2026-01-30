-- Table to track when view counts were last incremented per user+video
-- Used for atomic 24-hour deduplication to prevent race conditions
CREATE TABLE IF NOT EXISTS user_video_view_counts (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    last_counted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, video_id)
);

CREATE INDEX idx_user_video_view_counts_user_id ON user_video_view_counts(user_id);
CREATE INDEX idx_user_video_view_counts_video_id ON user_video_view_counts(video_id);
