-- "Most replayed" heatmap: per user and scene, how long each part of the scene was watched.
-- The scene is split into a fixed number of equal buckets (100); each element is the
-- watched time in milliseconds that fell into that bucket, summed over every viewing (so
-- rewatching a part increases it and seeking over a part does not count it).
CREATE TABLE user_scene_heatmaps (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scene_id BIGINT NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    buckets INTEGER[] NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, scene_id)
);

CREATE INDEX idx_user_scene_heatmaps_scene ON user_scene_heatmaps (scene_id);

-- Show the heatmap above the player progress bar, and record what is watched
ALTER TABLE user_settings
    ADD COLUMN show_replay_heatmap BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN record_replay_heatmap BOOLEAN NOT NULL DEFAULT true;
