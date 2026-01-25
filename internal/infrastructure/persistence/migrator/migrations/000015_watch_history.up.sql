CREATE TABLE user_video_watches (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    watched_at TIMESTAMP NOT NULL DEFAULT NOW(),
    watch_duration INTEGER NOT NULL DEFAULT 0,
    last_position INTEGER NOT NULL DEFAULT 0,
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_video_watches_user_video ON user_video_watches(user_id, video_id);
CREATE INDEX idx_user_video_watches_user_date ON user_video_watches(user_id, watched_at DESC);
